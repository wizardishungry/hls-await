package imagescore

import (
	"compress/gzip"
	"context"
	"image"
)

type GzipScorer struct{}

var _ ImageScorer = &GzipScorer{}

func NewGzipScorer() *GzipScorer { return &GzipScorer{} }

func (ps *GzipScorer) ScoreImage(ctx context.Context, img image.Image) (float64, error) {
	buf := &discardCounter{}
	img256 := downSampleImage(img)

	origBuf, err := imageBytes(img256)
	if err != nil {
		return -1, err
	}

	enc, err := gzip.NewWriterLevel(buf, gzip.BestSpeed)
	if err != nil {
		return 0, err

	}

	_, err = enc.Write(origBuf)
	if err != nil {
		return -1, err
	}

	err = enc.Close()
	if err != nil {
		return -1, err
	}

	origSize, err := uncompressedImageSize(img)
	if err != nil {
		return 0, err
	}

	return float64(buf.count) / float64(origSize), nil
}
