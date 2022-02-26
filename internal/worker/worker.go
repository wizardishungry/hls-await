package worker

import (
	"context"

	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
)

const (
	WORKER_FD = 3 + iota // stdin, stdout, stderr, ...
)

type Worker interface {
	Start(ctx context.Context) (err error)
	Restart(ctx context.Context)
	Handler(ctx context.Context) segment.Handler
}

var (
	_ Worker = &Parent{}
	_ Worker = &Child{}
	_ Worker = &InProcess{}
)
