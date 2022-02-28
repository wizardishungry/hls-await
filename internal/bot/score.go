package bot

import (
	"context"
	"image"
	"runtime"

	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func (b *Bot) scoreImages(ctx context.Context, srcImages []image.Image) ([]image.Image, error) {
	var (
		log        = logger.Entry(ctx)
		numWorkers = runtime.GOMAXPROCS(0)
		imagesIn   = make(chan image.Image, numWorkers)
		imagesOut  = make(chan image.Image, numWorkers)
		elimCount  int
	)
	log.WithField("num_images", len(srcImages)).
		Info("bulk scoring in progress")

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { // feed images to channel
		defer close(imagesIn)
		for _, i := range srcImages {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case imagesIn <- i:
			}
		}
		return nil
	})

	for i := 0; i < numWorkers; i++ { // consume channel with numWorkers
		g.Go(func() error {
			for img := range imagesIn {
				score, err := b.bulkScorer.ScoreImage(ctx, img)
				if err != nil {
					return errors.Wrap(err, "bulkScorer.ScoreImage")
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
		imageSliceC <- images
	}()

	err := g.Wait()
	close(imagesOut)
	log.WithField("elim_count", elimCount).WithError(err).Debug("bulk scoring eliminated images")
	if err != nil {
		return nil, err // eliminate images that caused bulkscorer to fail
	}
	select {
	default:
	case images, ok := <-imageSliceC: // await slice
		if !ok {
			break
		}
		return images, nil
	}
	return nil, errors.New("no images received from fanout")
}
