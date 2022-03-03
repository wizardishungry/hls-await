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

	type imageWithIndex struct {
		idx   int
		image image.Image
	}

	var (
		log        = logger.Entry(ctx)
		numWorkers = runtime.GOMAXPROCS(0)
		imagesIn   = make(chan imageWithIndex, numWorkers)
		imagesOut  = make([]image.Image, len(srcImages))
		elimCount  int
	)
	log.WithField("num_images", len(srcImages)).
		Info("bulk scoring in progress")

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { // feed images to channel
		defer close(imagesIn)
		for i, img := range srcImages {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case imagesIn <- imageWithIndex{idx: i, image: img}:
			}
		}
		return nil
	})

	for i := 0; i < numWorkers; i++ { // consume channel with numWorkers
		g.Go(func() error {
			for imgIdx := range imagesIn {
				score, err := b.bulkScorer.ScoreImage(ctx, imgIdx.image)
				if err != nil {
					return errors.Wrap(err, "bulkScorer.ScoreImage")
				}
				const minScore = 0.012 // TODO not great: jpeg specific
				if score < minScore {
					elimCount++
					log.WithField("score", score).WithField("idx", imgIdx.idx).Trace("eliminated image")
					continue
				}
				imagesOut[imgIdx.idx] = imgIdx.image
			}
			return nil
		})
	}

	err := g.Wait()
	log.WithField("elim_count", elimCount).WithError(err).Debug("bulk scoring eliminated images")
	if err != nil {
		return nil, err
	}

	size := len(srcImages) - elimCount
	if size == 0 {
		return nil, errors.New("no images returned from bulkscorer")
	}
	out := make([]image.Image, 0, size)
	for _, img := range imagesOut {
		if img != nil {
			out = append(out, img)
		}
	}
	return out, nil
}
