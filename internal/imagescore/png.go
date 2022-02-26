package imagescore

import (
	"context"
	"image"
	"image/png"
)

type PngScorer struct{}

var _ ImageScorer = &PngScorer{}

func NewPngScorer() *PngScorer { return &PngScorer{} }

func (ps *PngScorer) ScoreImage(ctx context.Context, img image.Image) (float64, error) {
	enc := png.Encoder{
		CompressionLevel: png.BestSpeed,
		// BufferPool: , // TOOO: try un/shared buffer pool across threads - should this be the same pool as for the output buffer
	}

	buf := &discardCounter{}

	img256 := downSampleImage(img)

	err := enc.Encode(buf, img256)
	if err != nil {
		return 0, err
	}

	origSize, err := uncompressedImageSize(img)
	if err != nil {
		return 0, err
	}
	return float64(buf.count) / origSize, nil
}
