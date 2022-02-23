package main

import (
	"context"
	"flag"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/WIZARDISHUNGRY/hls-await/internal/stream"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const streamURL = "https://tv.nknews.org/tvhls/stream.m3u8"

var log = logrus.New()

func main() {
	flag.Parse()

	// if *flagDumpFSM {
	// 	s, err := stream.NewStream()
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	log.Println(fsm.Visualize(s.GetFSM()))
	// 	return
	// }

	args := flag.Args()
	if len(args) == 0 {
		args = []string{streamURL}
	}

	worker := stream.InitWorker()

	ctx, ctxCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	// TODO need to readd SIGUSR1 support for one shot
	// TODO add support for hitting enter to get a screenshot
	defer ctxCancel()
	g, ctx := errgroup.WithContext(ctx)

	for _, arg := range args {

		u, err := url.Parse(arg)
		if err != nil || u.Scheme == "" {
			log.WithError(err).Fatalf("url.Parse: %s", arg)
		}
		s, err := stream.NewStream(
			stream.WithFlags(),
			stream.WithURL(*u),
			stream.WithWorker(worker),
		)
		if err != nil {
			log.WithError(err).Fatal("stream.NewStream")
		}
		log.Infof("monitoring %+v", u)

		g.Go(func() error {
			return s.Run(ctx)
		})
	}

	if err := g.Wait(); err != nil {
		log.Error(err)
	}
}
