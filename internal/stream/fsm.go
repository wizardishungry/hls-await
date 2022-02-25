package stream

import (
	"sync/atomic"
	"time"

	"github.com/looplab/fsm"
)

type FSM struct {
	Clock       func() time.Time
	FSM, Target *fsm.FSM
}

func (s *Stream) pushEvent(str string) {
	// fmt.Println("pushEvent", str) // FIXME convert to trace
	err := s.fsm.Target.Event(str)
	if _, ok := err.(fsm.NoTransitionError); err != nil && !ok {
		log.Println("push event error", s, err, s.fsm.Target.Current())
	}
}

//go:generate sh -c "cd ../../ && go run ./... -dump-fsm | dot -Nmargin=0.8 -s144 -Tsvg /dev/stdin -o fsm.svg"
func (s *Stream) GetFSM() *fsm.FSM {
	return s.fsm.FSM
}

func (s *Stream) newFSM() *FSM {
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
					s.oneShot <- struct{}{}
				},
				"after_event": func(e *fsm.Event) {
					// fmt.Println("event", e.Event) // TODO convert to Debug
					if e.Src != e.Dst {
						log.Printf("🏳[%s -> %s] %s\n", e.Src, e.Dst, e.Event)
						up := e.Dst == "up"
						i32up := int32(0)
						if up {
							i32up = 1
						}
						atomic.StoreInt32(&s.sendToBot, i32up)
						if !up {
							return
						}
						// img := func() image.Image {
						// 	singleImageMutex.Lock()
						// 	defer singleImageMutex.Unlock()
						// 	return singleImage
						// }()
						//
						// f := &bytes.Buffer{}
						// err := png.Encode(f, img)
						// if err != nil {
						// 	log.Println("png.Encode", err)
						// }
						// TODO: action here
						// _ = f

					}
				},
			},
		)}
	f.Target = newTimer(f.FSM)
	return &f
}

func newTimer(target *fsm.FSM) *fsm.FSM {
	const duration = 30 * time.Second // TODO move to const
	allStates := []string{"steady", "unsteady", "no_data"}
	var noDataTimer, steadyTimer, unsteadyTimer *time.Timer
	idleTimer := time.NewTicker(duration)
	var f *fsm.FSM

	cancelTimer := func(t *time.Timer) {
		if t != nil {
			t.Stop()
			t = nil
		}
	}

	// TODO this whole thing is too complicated
	// replace with a goroutine that polls on timers

	var enterNoData, enterSteady, enterUnsteady func(e *fsm.Event)

	enterNoData = func(e *fsm.Event) {
		cancelTimer(steadyTimer)
		cancelTimer(unsteadyTimer)
		noDataTimer = time.AfterFunc(duration, func() {
			target.Event("no_data_timer")
			enterNoData(e)
		})
	}
	enterSteady = func(e *fsm.Event) {
		cancelTimer(noDataTimer)
		// do not cancel unsteady timer
		steadyTimer = time.AfterFunc(duration, func() {
			cancelTimer(unsteadyTimer)
			target.Event("steady_timer")
			enterSteady(e)
		})
	}
	enterUnsteady = func(e *fsm.Event) {
		cancelTimer(steadyTimer)
		cancelTimer(noDataTimer)
		unsteadyTimer = time.AfterFunc(duration, func() {
			cancelTimer(steadyTimer)
			target.Event("unsteady_timer")
			enterUnsteady(e)
		})
	}

	go func() {
		for {
			<-idleTimer.C
			f.Event("no_data")
		}
	}()

	f = fsm.NewFSM(
		"no_data",
		fsm.Events{
			{Name: "steady", Src: allStates, Dst: "steady"},
			{Name: "unsteady", Src: allStates, Dst: "unsteady"},
			{Name: "no_data", Src: allStates, Dst: "no_data"},
		},
		fsm.Callbacks{
			"enter_no_data":  enterNoData,
			"enter_steady":   enterSteady,
			"enter_unsteady": enterUnsteady,
			"after_event": func(e *fsm.Event) {
				// fmt.Println("timer event", e.Event) // TODO convert to Trace
				if e.Src != e.Dst {
					log.Printf("⏰[%s -> %s] %s\n", e.Src, e.Dst, e.Event) // TODO convert to Debug
				}
				idleTimer.Reset(duration)

				err := target.Event(e.Dst)
				if _, ok := err.(fsm.NoTransitionError); err != nil && !ok {
					log.Println("problem with clock event", e, err)
				}
			},
		},
	)

	return f
}
