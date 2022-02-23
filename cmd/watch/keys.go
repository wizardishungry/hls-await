package main

import (
	"context"
	"fmt"

	"github.com/mattn/go-tty"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

func scanKeys(ctx context.Context) {
	tty, err := tty.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer tty.Close()

	for {
		r, err := tty.ReadRune()
		if err != nil {
			log.Fatal(err)
		}
		h, ok := keyMap[r]
		if !ok {
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
			cb: func(c context.Context) {
				fmt.Println(currentStream.GetFSM().Current())
			},
			desc: "Get current state",
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
