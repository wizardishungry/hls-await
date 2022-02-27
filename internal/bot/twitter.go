package bot

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/WIZARDISHUNGRY/hls-await/internal/imagescore"
	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const (
	TWITTER_CONSUMER_KEY    = "TWITTER_CONSUMER_KEY"
	TWITTER_CONSUMER_SECRET = "TWITTER_CONSUMER_SECRET"
	TWITTER_ACCESS_TOKEN    = "TWITTER_ACCESS_TOKEN"
	TWITTER_ACCESS_SECRET   = "TWITTER_ACCESS_SECRET"

	updateIntervalMinutes = 10
	updateInterval        = updateIntervalMinutes * time.Minute
	minUpdateInterval     = 20 * time.Second                                    // used for tweeting quickly after a manual restart
	numImages             = 4                                                   // per post
	maxQueuedImages       = 25 * updateIntervalMinutes * 60 * 2 * ImageFraction // about 2 updateIntervals at 25fps x the image fraction
	replyWindow           = 3 * updateInterval
	ImageFraction         = (1 / 25.0) // this is the proportion of images that make it from the decoder to here, aiming for 1/s (@25fps)
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
	lastPosted time.Time
	lastID     int64
	bulkScorer *imagescore.BulkScore
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
	b.getLastTweetMaybe(ctx)
	g.Go(func() error { return b.consumeImages(ctx) })
	return g.Wait()
}

func (b *Bot) Chan() chan<- image.Image {
	return b.c
}

func (b *Bot) consumeImages(ctx context.Context) error {
	log := logger.Entry(ctx)

	b.bulkScorer = imagescore.NewBulkScore(ctx,
		func() imagescore.ImageScorer {
			return imagescore.NewJpegScorer()
		},
	)

	ticker := time.NewTicker(b.calcUpdateInterval(ctx))
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
			// TODO: this should run in a goroutine and steal the images from the struct
			// running the image scoring+uploading is slow as crap
			if err != nil {
				log.WithError(err).Warn("maybeDoPost")
			}
			ticker.Reset(b.calcUpdateInterval(ctx))
		}
	}
}

func (b *Bot) calcUpdateInterval(ctx context.Context) (dur time.Duration) {
	log := logger.Entry(ctx)
	defer func() {
		log.WithField("tweet_in", dur).Debug("calcUpdateInterval")
	}()
	if !b.lastPosted.IsZero() {
		// try to post something quickly after manual restarts
		durSinceLast := time.Now().Sub(b.lastPosted)
		interval := updateInterval - durSinceLast
		if interval < minUpdateInterval {
			interval = minUpdateInterval
		}
		return interval
	}
	log.Warn("no last posted")
	return minUpdateInterval
}

func (b *Bot) maybeDoPost(ctx context.Context) error {
	log := logger.Entry(ctx)

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

	inputImages := []image.Image{}

	g, ctx := errgroup.WithContext(ctx)
	var (
		inputImagesMutex sync.Mutex
		elimCount        int
	)
	for _, i := range b.images {
		img := i
		g.Go(func() error {
			score, err := b.bulkScorer.ScoreImage(ctx, img)
			if err != nil {
				return err
			}
			inputImagesMutex.Lock()
			defer inputImagesMutex.Unlock()

			const minScore = 0.012 // TODO not great: jpeg specific
			if score < minScore {
				elimCount++
				log.WithField("score", score).Trace("eliminated image")
				return nil
			}
			inputImages = append(inputImages, img)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}
	log.WithField("elim_count", elimCount).Debug("eliminated images")

	offset := len(inputImages) / (numImages + 2)
	images := []image.Image{}
	for i := 0; i < numImages; i++ {
		img := inputImages[(i+1)*offset]
		images = append(images, img)
	}

	params := &twitter.StatusUpdateParams{
		MediaIds: make([]int64, len(images)),
	}

	if time.Now().Sub(b.lastPosted) < replyWindow {
		params.InReplyToStatusID = b.lastID
	}

	log.Info("uploading pics")

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

	if err := g.Wait(); err != nil {
		return err
	}

	log.Info("posting status")
	status := ""

	if params.InReplyToStatusID != 0 {
		n := time.Now().In(loc)
		status = fmt.Sprintf("It's currently %s in Pyongyang & KCTV is on the air!", n.Format(time.Kitchen))
	}

	params.Status = status

	t, _, err := b.client.Statuses.Update(status, params)
	if err != nil {
		return errors.Wrap(err, "Statuses.Update")
	}
	b.lastPosted = time.Now()
	b.lastID = t.ID

	b.images = b.images[0:0]
	return nil
}
