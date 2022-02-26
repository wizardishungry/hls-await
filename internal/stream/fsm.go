package stream

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/looplab/fsm"
)

type FSM struct {
	Clock  func() time.Time
	FSM    *fsm.FSM
	Target chan string
}

func (s *Stream) PushEvent(str string) {
	fmt.Println("pushEvent", str) // FIXME convert to trace
	select {
	case s.fsm.Target <- str:
	case <-time.After(time.Second):
		fmt.Println("pushEvent hung")
	}
	fmt.Println("pushEvent done", str) // FIXME convert to trace
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
					select {
					default:
					case s.oneShot <- struct{}{}:
					}
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

func newTimer(target *fsm.FSM) chan string {

	c := make(chan string, 1)

	const (
		duration = 30 * time.Second // TODO move to global config
		idleDur  = 3 * duration
	)
	go func() {
		idleTimer := time.NewTicker(idleDur)

		for {
			select {
			case <-idleTimer.C:
				c <- "no_data"
				continue
			case event := <-c:
				target.Event(event)
				func() {
					eventTimer := time.NewTimer(duration)

					idleTimer.Reset(idleDur)
					for {
						select {
						case <-idleTimer.C:
							c <- "no_data"
							continue
						case nextEvent := <-c:
							if nextEvent != event {
								c <- nextEvent
								return
							}
						case <-eventTimer.C:
							target.Event(event + "_timer")
							return
						}
					}
				}()
			}
		}
	}()

	return c
}
