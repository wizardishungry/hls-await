package imagescore

import (
	"context"
	"image"
	"image/png"
	"sync"
)

type PngScorer struct {
	enc png.Encoder
}

var _ ImageScorer = &PngScorer{}

func NewPngScorer() *PngScorer {
	return &PngScorer{
		enc: png.Encoder{
			CompressionLevel: png.BestSpeed,
			BufferPool:       &singleThreadBufferPool{},
		},
	}
}

func (ps *PngScorer) ScoreImage(ctx context.Context, img image.Image) (float64, error) {
	buf := &discardCounter{}

	img256 := downSampleImage(img)

	err := ps.enc.Encode(buf, img256)
	if err != nil {
		return 0, err
	}

	origSize, err := uncompressedImageSize(img)
	if err != nil {
		return 0, err
	}
	return float64(buf.count) / origSize, nil
}

type singleThreadBufferPool png.EncoderBuffer

var _ png.EncoderBufferPool = (*singleThreadBufferPool)(nil)

func (bp *singleThreadBufferPool) Get() *png.EncoderBuffer {
	return (*png.EncoderBuffer)(bp)
}
func (bp *singleThreadBufferPool) Put(eb *png.EncoderBuffer) {}

type bufferPool sync.Pool

var _ png.EncoderBufferPool = (*bufferPool)(nil)

var sharedBufferPool *bufferPool = (*bufferPool)(&sync.Pool{
	New: func() any {
		return &png.EncoderBuffer{}
	},
})

func (bp *bufferPool) Get() *png.EncoderBuffer {
	return (*sync.Pool)(bp).Get().(*png.EncoderBuffer)
}
func (bp *bufferPool) Put(eb *png.EncoderBuffer) {
	(*sync.Pool)(bp).Put(eb)
}
