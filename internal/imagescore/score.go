package imagescore

import (
	"context"
	"encoding/gob"
	"fmt"
	"image"
	"image/color"
	"io"
	"sync/atomic"

	"github.com/WIZARDISHUNGRY/hls-await/internal/filter"
	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/pkg/errors"
)

func Filter(bs ImageScorer, minScore float64) filter.FilterFunc {
	return func(ctx context.Context, img image.Image) (bool, error) {
		log := logger.Entry(ctx)
		score, err := bs.ScoreImage(ctx, img)
		if err != nil {
			return false, errors.Wrap(err, "bulkScorer.ScoreImage")
		}
		if score < minScore {
			log.WithField("score", score).Trace("bulk score eliminated image")
			return false, nil
		}
		log.WithField("score", score).Trace("bulk score passed image")
		return true, nil
	}
}

type ImageScorer interface {
	ScoreImage(ctx context.Context, img image.Image) (float64, error)
}

type discardCounter struct {
	count int
}

var _ io.Writer = &discardCounter{}

func (dc *discardCounter) Write(p []byte) (n int, err error) {
	dc.count += len(p)
	return len(p), nil
}

type uncompressedImageSizeCache struct {
	bounds atomic.Value //uncompressedImageSizeCacheEntry
}
type uncompressedImageSizeCacheEntry struct {
	bounds image.Rectangle
	size   float64
}

func (uisc *uncompressedImageSizeCache) size(img image.Image) (float64, error) {
	v := uisc.bounds.Load()
	if v != nil {
		entry, ok := v.(uncompressedImageSizeCacheEntry)
		if !ok {
			panic("v.(uncompressedImageSizeCacheEntry)")
		}
		if entry.bounds == img.Bounds().Canon() {
			return entry.size, nil
		}
		return -1, fmt.Errorf("bounds mismatch: stored(%+v) != new(%+v)", entry, img.Bounds().Canon())
	}

	size, err := uncompressedImageSize(img)
	if err == nil {
		entry := uncompressedImageSizeCacheEntry{bounds: img.Bounds().Canon(), size: size}
		uisc.bounds.Store(entry)
	}
	return size, err
}

func uncompressedImageSize(img image.Image) (float64, error) {
	buf := &discardCounter{}
	err := gob.NewEncoder(buf).Encode(&img)
	if err != nil {
		return -1, err
	}

	return float64(buf.count), nil
}

func init() {
	gob.Register(&image.RGBA{}) // need because this is contained in interface
	gob.Register(&color.RGBA{})
	gob.Register(&image.Paletted{})
}
