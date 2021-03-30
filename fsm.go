package main

import (
	"fmt"
	"time"

	"github.com/looplab/fsm"
)

type FSM struct {
	Clock       func() time.Time
	FSM, Target *fsm.FSM
}

//go:generate sh -c "go run ./... -dump-fsm | dot -s144 -Tsvg /dev/stdin -o fsm.svg"

func NewFSM() FSM {
	f := FSM{
		FSM: fsm.NewFSM(
			"undefined",
			fsm.Events{
				{Name: "steady", Src: []string{"undefined", "down"}, Dst: "down"},
				{Name: "steady", Src: []string{"up"}, Dst: "going_down"},
				{Name: "steady", Src: []string{"going_up"}, Dst: "going_up"},
				{Name: "steady_timer", Src: []string{"going_down", "down"}, Dst: "down"},
				{Name: "unsteady", Src: []string{"undefined", "down", "going_up"}, Dst: "going_up"},
				{Name: "unsteady_timer", Src: []string{"going_up", "going_down", "up"}, Dst: "up"},
				{Name: "no_data", Src: []string{"undefined", "down"}, Dst: "undefined"},
				{Name: "no_data", Src: []string{"going_up"}, Dst: "going_up"},
				{Name: "no_data", Src: []string{"going_down", "up"}, Dst: "going_down"},
				{Name: "no_data_timer", Src: []string{"undefined", "down", "undefined", "going_up", "going_down"}, Dst: "undefined"},
			},
			fsm.Callbacks{
				"enter_up": func(e *fsm.Event) {
					oneShot <- struct{}{}
				},
				"after_event": func(e *fsm.Event) {
					if e.Src != e.Dst {
						fmt.Printf("🏳[%s -> %s] %s\n", e.Src, e.Dst, e.Event)
					}
				},
			},
		)}
	f.Target = newTimer(f.FSM)
	return f
}

func newTimer(target *fsm.FSM) *fsm.FSM {
	const duration = time.Minute
	timer := time.NewTimer(duration)
	noDataTimer := time.NewTimer(duration)
	f := fsm.NewFSM(
		"no_data",
		fsm.Events{
			{Name: "timer", Src: []string{"steady", "steady_timer"}, Dst: "steady_timer"},
			{Name: "timer", Src: []string{"unsteady", "unsteady_timer"}, Dst: "unsteady_timer"},
			{Name: "timer", Src: []string{"no_data", "no_data_timer"}, Dst: "no_data_timer"},
			{Name: "steady", Src: []string{"steady_timer"}, Dst: "steady_timer"},
			{Name: "unsteady", Src: []string{"unsteady_timer"}, Dst: "unsteady_timer"},
			{Name: "no_data", Src: []string{"no_data_timer"}, Dst: "no_data_timer"},
			{Name: "steady", Src: []string{"steady", "unsteady", "no_data", "unsteady_timer", "no_data_timer"}, Dst: "steady"},
			{Name: "unsteady", Src: []string{"steady", "unsteady", "no_data", "steady_timer", "no_data_timer"}, Dst: "unsteady"},
			{Name: "no_data", Src: []string{"steady", "unsteady", "no_data", "steady_timer", "unsteady_timer"}, Dst: "no_data"},
		},
		fsm.Callbacks{
			"after_event": func(e *fsm.Event) {
				if e.Src != e.Dst {
					timer.Reset(duration)
					fmt.Printf("⏰[%s -> %s] %s\n", e.Src, e.Dst, e.Event)
					err := target.Event(e.Dst)
					if _, ok := err.(fsm.NoTransitionError); err != nil && !ok {
						fmt.Println("problem with clock event", e, err)
					}
				}
				if e.Event != "no_data" && e.Event != "no_data_timer" {
					noDataTimer.Reset(duration)
				}
			},
		},
	)
	go func() {
		for {
			select {
			case <-timer.C:
				err := f.Event("timer")
				if _, ok := err.(fsm.NoTransitionError); err != nil && !ok {
					fmt.Println("problem sending timer event", err)
				}
			case <-noDataTimer.C:
				err := f.Event("no_data")
				if _, ok := err.(fsm.NoTransitionError); err != nil && !ok {
					fmt.Println("problem sending no data timer event", err)
				}
			}
		}
	}()
	return f
}
