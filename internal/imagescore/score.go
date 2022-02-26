package imagescore

import (
	"context"
	"encoding/gob"
	"image"
	"image/png"
	"io"
)

func ScoreImage(ctx context.Context, img image.Image) (float64, error) {
	return 0, nil
}

type ImageScorer interface {
	ScoreImage(ctx context.Context, img image.Image) (float64, error)
}

type PngScorer struct{}

var _ ImageScorer = &PngScorer{}

func NewPngScorer() *PngScorer { return &PngScorer{} }

func (ps *PngScorer) ScoreImage(ctx context.Context, img image.Image) (float64, error) {
	enc := png.Encoder{
		CompressionLevel: png.BestSpeed,
		// BufferPool: , // TOOO: try un/shared buffer pool across threads - should this be the same pool as for the output buffer
	}

	buf := &discardCounter{}

	err := enc.Encode(buf, img)
	if err != nil {
		return 0, err
	}

	origSize, err := uncompressedSize(img)
	if err != nil {
		return 0, err
	}
	return float64(buf.count) / origSize, nil
}

type discardCounter struct {
	count int
}

var _ io.Writer = &discardCounter{}

func (dc *discardCounter) Write(p []byte) (n int, err error) {
	dc.count += len(p)
	return len(p), nil
}

// maybe we don't want to use this in real code paths since we can hope the input images have all the same dimensions and depth
func uncompressedSize(img image.Image) (float64, error) {
	buf := &discardCounter{}
	err := gob.NewEncoder(buf).Encode(&img)
	if err != nil {
		return -1, err
	}
	return float64(buf.count), nil
}
func init() {
	gob.Register(&image.RGBA{}) // needed because this is contained in interface
}
