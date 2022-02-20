package stream

import (
	"context"

	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
	"github.com/sirupsen/logrus"
)

func (s *Stream) ProcessSegment(ctx context.Context, file string) {

	h := segment.GoAV{
		VerboseDecoder: s.flags.VerboseDecoder,
	}

	resp, err := h.HandleSegment(ctx, segment.Request{Filename: file})

	if err != nil {
		log.WithError(err).Error("Stream.ProcessSegment")
	}
	log.WithFields(logrus.Fields{
		"num_images": len(resp.Images),
		"filename":   file,
	}).Debug("got images")
	for _, img := range resp.Images {
		select {
		case <-ctx.Done():
			return
		case s.imageChan <- img:
		}
	}
}
