package bot

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"time"

	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const (
	maxAge = 2 * updateIntervalMinutes // Don't post old images
	minAge = 90 * time.Second
)

func (b *Bot) maybeDoPost(ctx context.Context, srcImages []imageRecord) ([]imageRecord, error) {
	log := logger.Entry(ctx)

	const mimeType = "image/png"

	if len(srcImages) < numImages {
		log.WithField("num_images", len(srcImages)).Info("not enough images to post")
		return srcImages, nil
	}

	// Don't post old images
	firstGood := -1
	for i, img := range srcImages {
		if firstGood == -1 && time.Now().Sub(img.time) < maxAge {
			firstGood = i
		}
	}

	if firstGood < 0 {
		log.WithField("num_images", len(srcImages)).Info("discarding all images due to age")
		return nil, nil
	}
	if firstGood != 0 {
		log.WithField("num_images", len(srcImages)-firstGood).Info("discarding some images due to age")
		srcImages = srcImages[firstGood:]
	}

	if len(srcImages) < numImages {
		log.WithField("num_images", len(srcImages)).Info("not enough images to post")
		return srcImages, nil
	}

	if age := time.Now().Sub(srcImages[0].time); age < minAge {
		log.WithField("num_images", len(srcImages)).
			WithField("age", age).
			Info("images are too fresh too consider posting")
		return srcImages, nil
	}

	// Try to pick a good spread of images
	offset := len(srcImages) / numImages
	images := make([]imageRecord, 0, numImages)
	for i := 0; i < numImages; i++ {
		img := srcImages[i*offset+(offset/2)] // for 100: 12,37,62,97 8: 1,3,5,7 12: 1,4,7,10; 16: 2,6,10,14 1000: 125,375,625,875
		images = append(images, img)
	}

	params := &twitter.StatusUpdateParams{
		MediaIds: make([]int64, len(images)),
	}

	if time.Now().Sub(b.lastPosted) < replyWindow {
		params.InReplyToStatusID = b.lastID
	}

	g, ctx := errgroup.WithContext(ctx)

	log.Info("uploading pics")
	// TODO retry
	for i, img := range images {
		img := img
		i := i
		g.Go(func() error {
			f := &bytes.Buffer{}
			err := png.Encode(f, img.image)
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
