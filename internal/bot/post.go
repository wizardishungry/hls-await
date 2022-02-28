package bot

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"time"

	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func (b *Bot) maybeDoPost(ctx context.Context, srcImages []image.Image) ([]image.Image, error) {
	log := logger.Entry(ctx)

	const mimeType = "image/png"
	if len(srcImages) < numImages {
		log.WithField("num_images", len(srcImages)).Info("not enough images to post")
		return srcImages, nil
	}

	if images, err := b.scoreImages(ctx, srcImages); err != nil {
		return nil, errors.Wrap(err, "scoreImages")
	} else {
		srcImages = images
	}

	// Try to pick a good spread of images
	offset := len(srcImages) / numImages
	images := []image.Image{}
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
