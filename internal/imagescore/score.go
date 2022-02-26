package imagescore

import (
	"bytes"
	"context"
	"encoding/gob"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"io"
)

func ScoreImage(ctx context.Context, img image.Image) (float64, error) {
	return 0, nil
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

// maybe we don't want to use this in real code paths since we can hope the input images have all the same dimensions and depth
func uncompressedImageSize(img image.Image) (float64, error) {
	buf := &discardCounter{}
	err := gob.NewEncoder(buf).Encode(&img)
	if err != nil {
		return -1, err
	}
	return float64(buf.count), nil
}

func imageBytes(img image.Image) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := gob.NewEncoder(buf).Encode(&img)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func downSampleImage(img image.Image) image.Image {
	out := image.NewPaletted(img.Bounds(), palette.WebSafe)
	draw.Draw(out, out.Rect, img, image.Point{}, draw.Over)
	return out
}

func init() {
	gob.Register(&image.RGBA{}) // needed because this is contained in interface
	gob.Register(&color.RGBA{})
	gob.Register(&image.Paletted{})
}
