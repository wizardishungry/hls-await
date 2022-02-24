package bot

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"path/filepath"
	"runtime"
	"time"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

var log *logrus.Logger = logrus.New() // TODO move onto struct

const (
	TWITTER_CONSUMER_KEY    = "TWITTER_CONSUMER_KEY"
	TWITTER_CONSUMER_SECRET = "TWITTER_CONSUMER_SECRET"
	TWITTER_ACCESS_TOKEN    = "TWITTER_ACCESS_TOKEN"
	TWITTER_ACCESS_SECRET   = "TWITTER_ACCESS_SECRET"

	updateIntervalMinutes = 10
	updateInterval        = updateIntervalMinutes * time.Minute
	numImages             = 4                                   // per post
	maxQueuedImages       = 25 * updateIntervalMinutes * 60 * 2 // about 2 updateIntervals at 25fps
	replyWindow           = 3 * updateInterval
)

var (
	_, b, _, _ = runtime.Caller(0)

	// root folder of this project
	root = filepath.Join(filepath.Dir(b), "../..")
)

func newClient() *twitter.Client {
	path := root + "/.env"
	myEnv, err := godotenv.Read(path)
	if err != nil {
		panic(err)
	}

	consumerKey := myEnv["TWITTER_CONSUMER_KEY"]
	consumerSecret := myEnv["TWITTER_CONSUMER_SECRET"]
	accessToken := myEnv["TWITTER_ACCESS_TOKEN"]
	accessSecret := myEnv["TWITTER_ACCESS_SECRET"]
	if accessSecret == "" {
		return nil
	}

	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)
	httpClient := config.Client(oauth1.NoContext, token) // TODO use a real context?

	return twitter.NewClient(httpClient)
}

type Bot struct {
	client     *twitter.Client
	c          chan image.Image
	images     []image.Image
	lastPosted time.Time // TODO read from stream on boot
	lastID     int64     // TODO read from stream on boot
}

func NewBot() *Bot {
	b := &Bot{
		client: newClient(),
		c:      make(chan image.Image, 100), // TODO magic number
		images: make([]image.Image, 0, maxQueuedImages),
	}
	if b.client == nil {
		return nil
	}
	return b
}

func (b *Bot) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)
	b.getLastTweetMaybe()
	g.Go(func() error { return b.consumeImages(ctx) })
	return g.Wait()
}

func (b *Bot) Chan() chan<- image.Image {
	return b.c
}

func (b *Bot) consumeImages(ctx context.Context) error {

	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case img, ok := <-b.c:
			if !ok {
				return nil
			}

			b.images = append(b.images, img)
		case <-ticker.C:
			err := b.maybeDoPost(ctx) // TODO retry
			if err != nil {
				log.WithError(err).Warn("maybeDoPost")
			}
		}
	}
}

func (b *Bot) maybeDoPost(ctx context.Context) error {
	const mimeType = "image/png"
	if len(b.images) < numImages+2 {
		log.WithField("num_images", len(b.images)).Info("not enough images to post")
		return nil
	}
	defer func() {
		limit := len(b.images) - maxQueuedImages
		if limit > 0 {
			b.images = b.images[limit:]
		}
	}()
	offset := len(b.images) / (numImages + 2)
	images := []image.Image{}

	for i := 0; i < numImages; i++ {
		img := b.images[(i+1)*offset]
		images = append(images, img)
	}

	params := &twitter.StatusUpdateParams{
		MediaIds: make([]int64, len(images)),
	}

	if time.Now().Sub(b.lastPosted) < replyWindow {
		params.InReplyToStatusID = b.lastID
	}

	log.Info("uploading pics")
	g, ctx := errgroup.WithContext(ctx)
	for i, img := range images {
		img := img
		i := i
		g.Go(func() error {
			f := &bytes.Buffer{}
			err := png.Encode(f, img)
			if err != nil {
				return errors.Wrap(err, "png.Encode")
			}
			media, _, err := b.client.Media.Upload(f.Bytes(), mimeType)
			if err != nil {
				return errors.Wrap(err, "Media.Upload")
			}
			params.MediaIds[i] = media.MediaID
			return nil
		})
	}
	err := g.Wait()
	if err != nil {
		return err
	}

	log.Info("posting status")
	t, _, err := b.client.Statuses.Update("", params)
	if err != nil {
		return errors.Wrap(err, "Statuses.Update")
	}
	b.lastPosted = time.Now()
	b.lastID = t.ID

	// TODO: on boot check timeline and reply to recent thread
	//t.CreatedAt fmt is  `Wed Feb 23 23:25:53 +0000 2022``

	b.images = b.images[0:0]
	return nil
}
