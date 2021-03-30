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
				{Name: "steady", Src: []string{"up"}, Dst: "up"},
				{Name: "steady", Src: []string{"going_up"}, Dst: "going_up"},
				{Name: "steady", Src: []string{"going_down"}, Dst: "going_down"},
				{Name: "steady_timer", Src: []string{"up"}, Dst: "going_down"},
				{Name: "steady_timer", Src: []string{"going_down", "down", "going_up"}, Dst: "down"},
				{Name: "unsteady", Src: []string{"undefined", "down", "going_up"}, Dst: "going_up"},
				{Name: "unsteady", Src: []string{"up", "going_down"}, Dst: "up"},
				{Name: "unsteady_timer", Src: []string{"going_up", "going_down", "up"}, Dst: "up"},
				{Name: "no_data", Src: []string{"undefined", "down"}, Dst: "undefined"},
				{Name: "no_data", Src: []string{"going_up"}, Dst: "going_up"},
				{Name: "no_data", Src: []string{"going_down", "up"}, Dst: "going_down"},
				{Name: "no_data_timer", Src: []string{"undefined", "down", "going_up", "going_down", "up"}, Dst: "undefined"},
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
	const duration = 30 * time.Second
	allStates := []string{"steady", "unsteady", "no_data"}
	var noDataTimer, steadyTimer, unsteadyTimer *time.Timer
	idleTimer := time.NewTicker(duration)
	var f *fsm.FSM

	cancelAll := func() {
		if noDataTimer != nil {
			noDataTimer.Stop()
			noDataTimer = nil
		}
		if steadyTimer != nil {
			steadyTimer.Stop()
			steadyTimer = nil
		}
		if unsteadyTimer != nil {
			unsteadyTimer.Stop()
			unsteadyTimer = nil
		}
	}

	f = fsm.NewFSM(
		"no_data",
		fsm.Events{
			{Name: "steady", Src: allStates, Dst: "steady"},
			{Name: "unsteady", Src: allStates, Dst: "unsteady"},
			{Name: "no_data", Src: allStates, Dst: "no_data"},
		},
		fsm.Callbacks{
			"enter_no_data": func(e *fsm.Event) {
				cancelAll()
				noDataTimer = time.AfterFunc(duration, func() {
					target.Event("no_data_timer")
				})
			},
			"enter_steady": func(e *fsm.Event) {
				cancelAll()
				steadyTimer = time.AfterFunc(duration, func() {
					target.Event("steady_timer")
				})
			},
			"enter_unsteady": func(e *fsm.Event) {
				cancelAll()
				unsteadyTimer = time.AfterFunc(duration, func() {
					target.Event("unsteady_timer")
				})
			},
			"after_event": func(e *fsm.Event) {
				if e.Src != e.Dst {
					fmt.Printf("⏰[%s -> %s] %s\n", e.Src, e.Dst, e.Event)
				}
				idleTimer.Reset(duration)

				err := target.Event(e.Dst)
				if _, ok := err.(fsm.NoTransitionError); err != nil && !ok {
					fmt.Println("problem with clock event", e, err)
				}
			},
		},
	)

	go func() {
		for {
			<-idleTimer.C
			f.Event("no_data")
		}
	}()

	return f
}
