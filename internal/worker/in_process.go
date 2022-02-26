package worker

import (
	"context"

	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
)

type InProcess struct {
}

// startWorker runs in the child process
func (ip *InProcess) Start(ctx context.Context) error {
	return nil
}

func (ip *InProcess) Restart(ctx context.Context) {
	log := logger.Entry(ctx)

	log.Warn("Restarting an in process worker not supported.")
}

func (ip *InProcess) Handler(ctx context.Context) segment.Handler {
	return &segment.GoAV{
		Context:        ctx,
		VerboseDecoder: true, // TODO pass flags
		RecvUnixMsg:    false,
	}
}
