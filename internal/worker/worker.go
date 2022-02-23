package worker

import (
	"context"

	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger = logrus.New() // TODO move onto struct

const (
	WORKER_FD = 3 + iota // stdin, stdout, stderr, ...
)

type Worker interface {
	Start(context.Context) (err error)
	Handler() segment.Handler
}

var (
	_ Worker = &Parent{}
	_ Worker = &Child{}
)
