package stream

import (
	"context"
	"image"
	"net/url"
	"time"

	"github.com/WIZARDISHUNGRY/hls-await/internal/fifo"
	"github.com/sirupsen/logrus"
	"jonwillia.ms/iot/pkg/errgroup" // TODO use other ddep
)

var mk fifo.Mkfifo
var cleanup func() error

var log *logrus.Logger = logrus.New() // TODO move onto struct

type StreamOption func(s *Stream) error

func NewStream(opts ...StreamOption) (*Stream, error) {
	s := newStream()
	for _, opt := range opts {
		err := opt(s)
		if err != nil {
			return nil, err
		}
	}

	s.fsm = s.newFSM()

	return s, nil
}

func WithURL(u url.URL) StreamOption {
	return func(s *Stream) error {
		s.url = u
		return nil
	}
}

type Stream struct {
	// StreamOptions
	url url.URL
	// newStream
	oneShot    chan struct{}
	imageChan  chan image.Image
	flags      flags
	segmentMap map[url.URL]struct{}

	// NewStream
	fsm FSM
}

func newStream() *Stream {
	return &Stream{
		oneShot:    make(chan struct{}, 1),
		imageChan:  make(chan image.Image),
		segmentMap: make(map[url.URL]struct{}),
	}
}
func (s *Stream) close() error { // TODO once
	close(s.oneShot)
	close(s.imageChan)
	return nil
}

func (s *Stream) Run(ctx context.Context) error {

	var err error
	mk, cleanup, err = fifo.Factory()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		err := cleanup()
		if err != nil {
			log.Fatal("MkFIFOFactory()cleanup()", err)
		}
	}()

	defer s.close()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return s.consumeImages(ctx) })

	pollDuration := minPollDuration
	for {
		start := time.Now()
		mediapl, err := doPlaylist(ctx, &s.url)
		if err != nil {
			log.Println("processPlaylist", err)
			pollDuration = minPollDuration
		} else {
			if dur := mediapl.TargetDuration; dur > 0 {
				tdDuration := time.Duration(dur * float64(time.Second))
				if tdDuration > minPollDuration {
					pollDuration = tdDuration
				}
				if tdDuration > maxPollDuration {
					pollDuration = maxPollDuration
				}
			}
			err := s.handleSegments(ctx, mediapl)
			if err != nil {
				log.Error("handleSegments", err)
			}
		}
		elapsed := time.Now().Sub(start)
		sleepFor := pollDuration - elapsed
		if sleepFor < minPollDuration {
			sleepFor = minPollDuration
		}
		timer := time.NewTimer(pollDuration)
		log.Println("processPlaylist elapsed time", elapsed)
		log.Println("processPlaylist pollDuration", pollDuration)
		log.Println("processPlaylist sleeping for", sleepFor)
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
	}
}
