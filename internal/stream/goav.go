package stream

import (
	"context"

	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
	"github.com/sirupsen/logrus"
)

func (s *Stream) ProcessSegment(ctx context.Context, request segment.Request) error {

	h := s.worker.Handler()

	var resp segment.Response
	err := h.HandleSegment(&request, &resp)

	if err != nil {
		return err
	}
	log.WithFields(logrus.Fields{
		"num_images": len(resp.RawImages),
	}).Debug("got images")
	for _, img := range resp.RawImages {
		select {
		case <-ctx.Done():
			return nil
		case s.imageChan <- img:
		}
	}
	return nil
}
