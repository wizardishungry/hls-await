package bot

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"runtime"
	"time"

	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func (b *Bot) maybeDoPost(ctx context.Context, srcImages []image.Image) ([]image.Image, error) {
	log := logger.Entry(ctx)

	const mimeType = "image/png"
	if len(srcImages) < numImages+2 {
		log.WithField("num_images", len(srcImages)).Info("not enough images to post")
		return srcImages, nil
	}

	{ // Bulk Scoring
		var (
			numWorkers = runtime.GOMAXPROCS(0)
			imagesIn   = make(chan image.Image, numWorkers)
			imagesOut  = make(chan image.Image, numWorkers)
			elimCount  int
		)
		log.WithField("num_images", len(srcImages)).Info("bulk scoring in progress")

		go func() { // feed images to channel
			defer close(imagesIn)
			for _, i := range srcImages {
				select {
				case <-ctx.Done():
					return
				case imagesIn <- i:
				}
			}
		}()

		egScore, ctx := errgroup.WithContext(ctx)
		for i := 0; i < numWorkers; i++ { // consume channel with numWorkers
			egScore.Go(func() error {
				for img := range imagesIn {
					score, err := b.bulkScorer.ScoreImage(ctx, img)
					if err != nil {
						return err
					}
					const minScore = 0.012 // TODO not great: jpeg specific
					if score < minScore {
						elimCount++
						log.WithField("score", score).Trace("eliminated image")
						continue
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					case imagesOut <- img:
					}
				}
				return nil
			})
		}

		imageSliceC := make(chan []image.Image)
		go func() { // collect channel to slice
			defer close(imageSliceC)
			images := make([]image.Image, 0, len(srcImages))
			for img := range imagesOut {
				images = append(images, img)
			}
			if ctx.Err() != nil {
				return
			}
			imageSliceC <- images
		}()

		err := egScore.Wait()
		close(imagesOut)
		if err != nil {
			return nil, err // eliminate images that caused bulkscorer to fail
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case images, ok := <-imageSliceC: // await slice
			if !ok {
				return nil, fmt.Errorf("no images received from fanout") // should never happen
			}
			srcImages = images
		}

		log.WithField("elim_count", elimCount).Debug("bulk scoring eliminated images")
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
