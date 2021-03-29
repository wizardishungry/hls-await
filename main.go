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
)

const streamURL = "https://tv.nknews.org/tvhls/stream.m3u8"

var mk mkfifo
var cleanup func() error

var globalWG sync.WaitGroup

var (
	flagSet  = flag.NewFlagSet("", flag.ExitOnError)
	dumpHttp = flagSet.Bool("dump-http", false, "dumps http headers")
)

func main() {
	flagSet.Parse(os.Args)

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
	signal.Notify(killSignal, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	ctx, ctxCancel := context.WithCancel(context.Background())

	globalWG.Add(1)
	go processPlaylist(ctx, u)

	select {
	case s := <-killSignal:
		ctxCancel()
		fmt.Println("exiting ", s)
	}
	globalWG.Wait()
}
