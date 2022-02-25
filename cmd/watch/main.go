package main

import (
	"context"
	"flag"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/WIZARDISHUNGRY/hls-await/internal/bot"
	"github.com/WIZARDISHUNGRY/hls-await/internal/roku"
	"github.com/WIZARDISHUNGRY/hls-await/internal/stream"
	"github.com/WIZARDISHUNGRY/hls-await/internal/worker"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const streamURL = "https://tv.nknews.org/tvhls/stream.m3u8"

var (
	log           = logrus.New()
	currentStream *stream.Stream
)

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		args = []string{streamURL}
	}

	b := bot.NewBot()
	w := stream.InitWorker()

	ctx, ctxCancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGTERM, syscall.SIGQUIT,
		os.Interrupt, os.Kill,
	)
	defer func() {
		ctxCancel()
		log.Info("main exiting")
	}()
	rokuCB := roku.Run(ctx) // TODO add flag and support autolaunch on motion

	g, ctx := errgroup.WithContext(ctx)

	if _, ok := w.(*worker.Child); !ok { // FIXME hacky
		if b != nil {
			g.Go(func() error {
				return b.Run(ctx)
			})
		} else {
			log.Warn("twitter unconfigured")
		}
	}

	for _, arg := range args {
		u, err := url.Parse(arg)

		if err != nil || u.Scheme == "" {
			log.WithError(err).Fatalf("url.Parse: %s", arg)
		}
		currentStream, err = stream.NewStream(
			stream.WithFlags(),
			stream.WithURL(u),
			stream.WithWorker(w),
			stream.WithRokuCB(rokuCB),
			stream.WithBot(b),
		)
		if err != nil {
			log.WithError(err).Fatal("stream.NewStream")
		}
		log.Infof("monitoring %+v", u)

		g.Go(func() error {
			return currentStream.Run(ctx)
		})

	}

	go scanKeys(ctx)

	if err := g.Wait(); err != nil {
		log.Error(err)
	}
}
