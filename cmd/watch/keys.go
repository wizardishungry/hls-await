package main

import (
	"context"
	"fmt"

	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/mattn/go-tty"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

func scanKeys(ctx context.Context) {
	log := logger.Entry(ctx)
	tty, err := tty.OpenDevice("/dev/stdin")
	if err != nil {
		return // child process will fail
	}
	defer tty.Close()

	for {
		r, err := tty.ReadRune()
		if err != nil {
			log.Fatal(err)
		}
		h, ok := keyMap[r]
		if !ok {
			log.Warnf("unknown key %d, %s\n", r, string(r))
			continue
		}
		h.cb(ctx)
	}
}

type kmt = map[rune]struct {
	cb   func(context.Context)
	desc string
}

var keyMap kmt

func init() {
	keyMap = kmt{
		13: // enter
		{
			cb:   func(c context.Context) { currentStream.OneShot() <- struct{}{} },
			desc: "Dump ansi art frame",
		},
		'f': {
			cb: func(ctx context.Context) {
				log := logger.Entry(ctx)
				log.Infof(currentStream.GetFSM().Current())
			},
			desc: "Get current state",
		},
		'r': {
			cb: func(ctx context.Context) {
				log := logger.Entry(ctx)
				log.Info("roku launch")
				err := currentStream.LaunchRoku()
				if err != nil {
					log.WithError(err).Warn("roku error")
				} else {
					log.Info("roku launched")
				}
			},
			desc: "Launch Stream in Roku Stream Tester",
		},
		'u': {
			cb: func(ctx context.Context) {
				currentStream.PushEvent(ctx, "unsteady")
			},
			desc: "Push an unsteady event", // FIXME remove
		},
		'U': {
			cb: func(c context.Context) {
				currentStream.GetFSM().Event("unsteady_timer")
			},
			desc: "Go up immediately", // FIXME remove
		},
		'?': {
			desc: "Help",
			cb: func(c context.Context) {
				keys := maps.Keys(keyMap)
				slices.Sort(keys)
				for _, k := range keys {
					fmt.Printf("%s\t%s\n", string(k), keyMap[k].desc)
				}
			},
		},
	}
}
