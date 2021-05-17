package main

import (
	"context"
	"flag"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/looplab/fsm"
	"github.com/sirupsen/logrus"
	"jonwillia.ms/iot/pkg/errgroup"
)

const streamURL = "https://tv.nknews.org/tvhls/stream.m3u8"

var log = logrus.New()

var mk mkfifo
var cleanup func() error

var (
	flagURL            = flag.String("url", streamURL, "url")
	flagDumpHttp       = flag.Bool("dump-http", false, "dumps http headers")
	flagVerboseDecoder = flag.Bool("verbose-decoder", false, "ffmpeg debuggging info")
	flagAnsiArt        = flag.Int("ansi-art", 0, "output ansi art on modulo frame")
	flagThreshold      = flag.Int("threshold", 2, "need this much to output a warning")
	flagFlicker        = flag.Bool("flicker", false, "reset terminal in ansi mode")
	flagFastStart      = flag.Int("fast-start", 1, "start by only processing this many recent segments")
	flagFastResume     = flag.Bool("fast-resume", true, "if we see a bunch of new segments, behave like fast start")
	flagDumpFSM        = flag.Bool("dump-fsm", false, "write graphviz src and exit")
	flagOneShot        = flag.Bool("one-shot", true, "render an ansi frame when entering up state")
	flagSixel          = flag.Bool("sixel", false, "output ansi images as sixels")
)

func main() {
	flag.Parse()

	if *flagDumpFSM {
		log.Println(fsm.Visualize(myFSM.FSM))
		return
	}

	var err error
	mk, cleanup, err = MkFIFOFactory()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		err := cleanup()
		if err != nil {
			log.Fatal("MkFIFOFactory()cleanup()", err)
		}
	}()

	args := flag.Args()
	if len(args) == 0 {
		args = []string{*flagURL}
	}

	ctx, ctxCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	// TODO need to readd SIGUSR1 support for one shot
	defer ctxCancel()
	g, ctx := errgroup.WithContext(ctx)

	for _, arg := range args {
		u, err := url.Parse(arg)
		if err != nil {
			log.Fatal(err)
		}

		log.Infof("monitoring %+v", u)
		g.Go(func() error {
			return processPlaylist(ctx, u)
		})

	}

	if err := g.Wait(); err != nil {
		log.Error(err)
	}
}
