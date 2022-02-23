package worker

import (
	"context"

	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
)

type InProcess struct {
}

// startWorker runs in the child process
func (w *InProcess) Start(ctx context.Context) error {
	return nil
}

func (w *InProcess) Handler() segment.Handler {
	return &segment.GoAV{
		VerboseDecoder: true, // TODO pass flags
		RecvUnixMsg:    false,
	}
}
