package stream

import (
	"context"
	"time"

	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
	"github.com/sirupsen/logrus"
)

const workerMaxDuration = 10 * time.Second // if the worker appears to be stalled

func (s *Stream) ProcessSegment(ctx context.Context, request segment.Request) error {

	h := s.worker.Handler()

	timeOut := time.NewTimer(workerMaxDuration)
	workerDone := make(chan struct{})
	go func() {
		// safety timeout since net/rpc doesn't use contexts
		select {
		case <-ctx.Done():
		case <-workerDone:
		case <-timeOut.C:
			s.worker.Restart()
		}
		if !timeOut.Stop() {
			<-timeOut.C
		}
	}()

	var resp segment.Response
	err := h.HandleSegment(&request, &resp)
	close(workerDone)

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
