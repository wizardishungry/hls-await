package main

import (
	"context"
	"flag"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/WIZARDISHUNGRY/hls-await/internal/bot"
	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/WIZARDISHUNGRY/hls-await/internal/roku"
	"github.com/WIZARDISHUNGRY/hls-await/internal/stream"
	"github.com/WIZARDISHUNGRY/hls-await/internal/worker"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const streamURL = "https://tv.nknews.org/tvhls/stream.m3u8"

var (
	currentStream *stream.Stream
)

var (
	_, b, _, _ = runtime.Caller(0)

	// root folder of this project for trimming frames
	root = filepath.Join(filepath.Dir(b), "../..")
)

func main() {
	logr := logrus.New()
	logr.ReportCaller = true
	logr.Formatter = &logrus.TextFormatter{
		CallerPrettyfier: func(f *runtime.Frame) (function string, file string) {
			lastIdx := strings.LastIndexByte(f.Function, '/')
			if lastIdx == -1 {
				lastIdx = 0
			} else {
				lastIdx++
			}
			fxn := f.Function[lastIdx:]
			lastIdx = strings.IndexByte(fxn, '(')
			if lastIdx == -1 {
				lastIdx = 0
			}
			fxn = fxn[lastIdx:]
			return fxn,
				""
			//	f.File[len(root)+1:]
		},
	}
	log := logr.WithFields(nil)
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		args = []string{streamURL}
	}

	b := bot.NewBot()
	w := stream.InitWorker()

	ctx := context.Background()
	ctx = logger.WithLogEntry(ctx, log)

	ctx, ctxCancel := signal.NotifyContext(ctx,
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGTERM,
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
		log := log
		if len(args) > 1 {
			log = log.WithField("playlist", arg)
		} else {
			log = log.WithFields(logrus.Fields{})
		}
		ctx := logger.WithLogEntry(ctx, log)

		u, err := url.Parse(arg)

		if err != nil || u.Scheme == "" {
			log.WithError(err).Fatalf("url.Parse: %s", arg)
		}
		s, err := stream.NewStream(
			stream.WithFlags(),
			stream.WithURL(u),
			stream.WithWorker(w),
			stream.WithRokuCB(rokuCB),
			stream.WithBot(b),
		)
		if err != nil {
			log.WithError(err).Fatal("stream.NewStream")
		}
		log.Info("monitoring")

		g.Go(func() error {
			return s.Run(ctx)
		})
		currentStream = s
	}
	go scanKeys(ctx)

	if err := g.Wait(); err != nil {
		log.Error(err)
	}
}
