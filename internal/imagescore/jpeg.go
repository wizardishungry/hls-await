package imagescore

import (
	"context"
	"image"
	"image/jpeg"
)

type JpegScorer struct {
	uncompressedImageSizeCache
}

var _ ImageScorer = &JpegScorer{}

func NewJpegScorer() *JpegScorer { return &JpegScorer{} }

func (js *JpegScorer) ScoreImage(ctx context.Context, img image.Image) (float64, error) {
	opts := jpeg.Options{Quality: jpeg.DefaultQuality}
	buf := &discardCounter{}

	err := jpeg.Encode(buf, img, &opts)
	if err != nil {
		return 0, err
	}

	origSize, err := js.size(img)
	if err != nil {
		return 0, err
	}
	return float64(buf.count) / origSize, nil
}
