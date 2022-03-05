package imagescore

import (
	gzip "compress/flate"
	"context"
	"image"
)

// GzipScorer actually uses flate instead
type GzipScorer struct {
	uncompressedImageSizeCache
}

var _ ImageScorer = &GzipScorer{}

func NewGzipScorer() *GzipScorer { return &GzipScorer{} }

func (gs *GzipScorer) ScoreImage(ctx context.Context, img image.Image) (float64, error) {
	buf := &discardCounter{}

	enc, err := gzip.NewWriter(buf, gzip.DefaultCompression)
	if err != nil {
		return 0, err

	}

	err = enc.Close()
	if err != nil {
		return -1, err
	}

	origSize, err := gs.size(img)
	if err != nil {
		return 0, err
	}

	return float64(buf.count) / float64(origSize), nil
}
