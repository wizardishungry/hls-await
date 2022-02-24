package stream

import (
	"github.com/WIZARDISHUNGRY/hls-await/internal/bot"
	"github.com/WIZARDISHUNGRY/hls-await/internal/worker"
	"jonwillia.ms/roku"
)

func InitWorker() worker.Worker {
	if someFlags.Worker {
		return &worker.Child{}
	}
	if !someFlags.Privsep {
		return &worker.InProcess{}
	}
	return &worker.Parent{}
}

func WithWorker(w worker.Worker) StreamOption {
	// TODO: allow in-process workers
	return func(s *Stream) error {
		s.worker = w
		return nil
	}
}

func WithBot(b *bot.Bot) StreamOption {
	return func(s *Stream) error {
		s.bot = b
		return nil
	}
}

func WithRokuCB(rokuCB func() (*roku.Remote, error)) StreamOption {
	return func(s *Stream) error {
		s.rokuCB = rokuCB
		return nil
	}
}
