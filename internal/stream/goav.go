package stream

import (
	"context"

	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
	"github.com/charlestamz/goav/avcodec"
)

func init() {
	// avformat.AvRegisterAll()
	avcodec.AvcodecRegisterAll()

}

func (s *Stream) ProcessSegment(ctx context.Context, file string) {

	h := segment.GoAV{
		VerboseDecoder: s.flags.VerboseDecoder,
	}

	resp, err := h.HandleSegment(ctx, segment.Request{Filename: file})

	if err != nil {
		log.WithError(err).Error("Stream.ProcessSegment")
	}
	for _, img := range resp.Images {
		select {
		case <-ctx.Done():
			return
		case s.imageChan <- img:
		}
	}
}
