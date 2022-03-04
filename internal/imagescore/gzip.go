package imagescore

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/gob"
	"image"
)

type GzipScorer struct {
	uncompressedImageSizeCache
}

var _ ImageScorer = &GzipScorer{}

func NewGzipScorer() *GzipScorer { return &GzipScorer{} }

func (gs *GzipScorer) ScoreImage(ctx context.Context, img image.Image) (float64, error) {
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
