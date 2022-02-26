package imagescore

import (
	"context"
	"image"
	"image/jpeg"
)

type JpegScorer struct{}

var _ ImageScorer = &JpegScorer{}

func NewJpegScorer() *JpegScorer { return &JpegScorer{} }

func (ps *JpegScorer) ScoreImage(ctx context.Context, img image.Image) (float64, error) {
	opts := jpeg.Options{Quality: jpeg.DefaultQuality}
	buf := &discardCounter{}

	img256 := downSampleImage(img)

	err := jpeg.Encode(buf, img256, &opts)
	if err != nil {
		return 0, err
	}

	origSize, err := uncompressedImageSize(img)
	if err != nil {
		return 0, err
	}
	return float64(buf.count) / origSize, nil
}
