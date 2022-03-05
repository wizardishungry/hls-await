package imagescore

import (
	"bytes"
	gzip "compress/flate"
	"context"
	"encoding/gob"
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

func imageBytes(img image.Image) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := gob.NewEncoder(buf).Encode(&img)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
