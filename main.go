package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/looplab/fsm"
)

const streamURL = "https://tv.nknews.org/tvhls/stream.m3u8"

var mk mkfifo
var cleanup func() error

var globalWG sync.WaitGroup

var (
	flagDumpHttp   = flag.Bool("dump-http", false, "dumps http headers")
	flagAnsiArt    = flag.Int("ansi-art", 0, "output ansi art on modulo frame")
	flagThreshold  = flag.Int("threshold", 1, "need this much to output a warning")
	flagFlicker    = flag.Bool("flicker", false, "reset terminal in ansi mode")
	flagFastStart  = flag.Int("fast-start", 1, "start by only processing this many recent segments")
	flagFastResume = flag.Bool("fast-resume", true, "if we see a bunch of new segments, behave like fast start")
	flagDumpFSM    = flag.Bool("dump-fsm", false, "write graphviz src and exit")
	flagOneShot    = flag.Bool("one-shot", true, "render an ascii frame when entering up state")
)

var myFSM = NewFSM()

func pushEvent(s string) {
	err := myFSM.Target.Event(s)
	if _, ok := err.(fsm.NoTransitionError); err != nil && !ok {
		fmt.Println("push event error", s, err, myFSM.Target.Current())
	}
}

func main() {
	flag.Parse()

	if *flagDumpFSM {
		fmt.Println(fsm.Visualize(myFSM.FSM))
		os.Exit(0)
	}

	var err error
	mk, cleanup, err = MkFIFOFactory()
	if err != nil {
		panic(err)
	}
	defer func() {
		err := cleanup()
		if err != nil {
			fmt.Println("MkFIFOFactory()cleanup()", err)
		}
	}()

	// segmentChan := make(chan url.URL)
	// go consumeSegments(segmentChan)
	// defer close(segmentChan)

	u, err := url.Parse(streamURL)
	if err != nil {
		panic(err)
	}

	killSignal := make(chan os.Signal, 0)
	signal.Notify(killSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGINFO, os.Interrupt, os.Kill)
	ctx, ctxCancel := context.WithCancel(context.Background())

	globalWG.Add(1)
	go processPlaylist(ctx, u)

LOOP:
	for {
		select {
		case s := <-killSignal:
			if s == syscall.SIGINFO {
				fmt.Println("siginfo")
				oneShot <- struct{}{}
				break
			}
			ctxCancel()
			fmt.Println("exiting ", s)
			break LOOP
		}
	}

	globalWG.Wait()
}
