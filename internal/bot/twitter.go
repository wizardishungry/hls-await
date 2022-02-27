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
	minUpdateInterval     = 20 * time.Second // used for tweeting quickly after a manual restart
	numImages             = 4                // per post
	maxQueuedIntervals    = 4
	maxQueuedImages       = 25 * updateIntervalMinutes * 60 * maxQueuedIntervals * ImageFraction // about 2 updateIntervals at 25fps x the image fraction
	maxQueuedImagesMult   = 1.5
	replyWindow           = 3 * updateInterval
	ImageFraction         = (1 / 25.0) // this is the proportion of images that make it from the decoder to here, aiming for 1/s (@25fps)
	postTimeout           = time.Minute
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
	lastPosted time.Time
	lastID     int64
	bulkScorer *imagescore.BulkScore
}

func NewBot() *Bot {
	b := &Bot{
		client: newClient(),
		c:      make(chan image.Image, 100), // TODO magic number
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

	newImageSlice := func() []image.Image { return make([]image.Image, 0, maxQueuedImages) }

	images := newImageSlice()

	ticker := time.NewTicker(b.calcUpdateInterval(ctx))
	defer ticker.Stop()
	unusedImagesC := make(chan []image.Image, 1)
	defer close(unusedImagesC)
	for {
		select {
		case <-ctx.Done():
			return nil
		case img, ok := <-b.c:
			if !ok {
				return nil
			}
			images = append(images, img)
		case imgs := <-unusedImagesC:
			if len(imgs) > 0 {
				log.Warn("unused images retained")
				images = append(imgs, images...) // unused images get moved to the front
			}
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(ctx, postTimeout)
			srcImages := images
			images = newImageSlice()
			go func() {
				defer cancel()
				unusedImages, err := b.maybeDoPost(ctx, srcImages)
				// this runs in a goroutine because image scoring+uploading is slow as crap
				if err != nil {
					log.WithError(err).Warn("maybeDoPost")
				}
				select {
				case unusedImagesC <- unusedImages:
				default:
					log.Warn("unused images discarded")
				}
				ticker.Reset(b.calcUpdateInterval(ctx))
			}()
		}
		limit := len(images) - maxQueuedImages*maxQueuedImagesMult
		if limit > 0 {
			limit := len(images) - maxQueuedImages
			log.WithField("num_images", limit).WithField("maxQueuedImages", maxQueuedImages).
				Info("eliminated images (over maxQueuedImages)")
			images = images[limit:]
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

func (b *Bot) maybeDoPost(ctx context.Context, srcImages []image.Image) ([]image.Image, error) {
	log := logger.Entry(ctx)

	const mimeType = "image/png"
	if len(srcImages) < numImages+2 {
		log.WithField("num_images", len(srcImages)).Info("not enough images to post")
		return srcImages, nil
	}
	g, ctx := errgroup.WithContext(ctx)

	{
		inputImages := []image.Image{}

		var (
			inputImagesMutex sync.Mutex
			elimCount        int
		)
		log.WithField("num_images", len(srcImages)).Info("bulk scoring in progress")
		for _, i := range srcImages {
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
			return nil, err // eliminate images that caused bulkscorer to fail
		}
		log.WithField("elim_count", elimCount).Debug("bulk scoring eliminated images")
		srcImages = inputImages // overwrite so we can potentially prune
	}

	// Try to pick a good spread of images
	offset := len(srcImages) / (numImages + 2)
	images := []image.Image{}
	for i := 0; i < numImages; i++ {
		img := srcImages[(i+1)*offset]
		images = append(images, img)
	}

	params := &twitter.StatusUpdateParams{
		MediaIds: make([]int64, len(images)),
	}

	if time.Now().Sub(b.lastPosted) < replyWindow {
		params.InReplyToStatusID = b.lastID
	}

	log.Info("uploading pics")
	// TODO retry
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
		return srcImages, err
	}

	log.Info("posting status")
	status := ""

	if params.InReplyToStatusID == 0 {
		n := time.Now().In(loc)
		status = fmt.Sprintf("It's currently %s in Pyongyang & KCTV is on the air!", n.Format(time.Kitchen))
	}

	params.Status = status
	// TODO retry
	t, _, err := b.client.Statuses.Update(status, params)
	if err != nil {
		return srcImages, errors.Wrap(err, "Statuses.Update")
	}

	// These are racey TODO
	b.lastPosted = time.Now()
	b.lastID = t.ID

	return nil, nil
}
