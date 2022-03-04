package imagescore

import (
	"context"
	"image"
	"image/color/palette"
	"image/draw"
)

type Downsampler struct {
	scorer ImageScorer
}

var _ ImageScorer = &Downsampler{}

func NewDownsampler(scorer ImageScorer) *Downsampler {
	return &Downsampler{scorer: scorer}
}

func (ds *Downsampler) ScoreImage(ctx context.Context, img image.Image) (float64, error) {
	img = downsampleImage(img)
	if ctx.Err() != nil {
		return -1, ctx.Err()
	}
	return ds.scorer.ScoreImage(ctx, img)
}

func downsampleImage(img image.Image) image.Image {
	out := image.NewPaletted(img.Bounds(), palette.WebSafe)
	draw.Draw(out, out.Rect, img, image.Point{}, draw.Over)
	return out
}
