package imagescore

import (
	"context"
	"image"
	"image/gif"
)

type GifScorer struct{}

var _ ImageScorer = &GifScorer{}

func NewGifScorer() *GifScorer { return &GifScorer{} }

func (ps *GifScorer) ScoreImage(ctx context.Context, img image.Image) (float64, error) {
	opts := gif.Options{NumColors: 256}
	buf := &discardCounter{}

	err := gif.Encode(buf, img, &opts)
	if err != nil {
		return 0, err
	}

	origSize, err := uncompressedImageSize(img)
	if err != nil {
		return 0, err
	}
	return float64(buf.count) / origSize, nil
}
