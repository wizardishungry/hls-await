package stream

import (
	"context"
	"fmt"
	"image"
	"net/url"
	"time"

	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
	"github.com/WIZARDISHUNGRY/hls-await/pkg/proxy"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

var log *logrus.Logger = logrus.New() // TODO move onto struct
func init() {
	log.Level = logrus.DebugLevel
}

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
	if true {
		target, err := s.url.Parse("/")
		if err != nil {
			return nil, err
		}
		u, err := proxy.NewSingleHostReverseProxy(context.TODO(), target, false) //  TODO don't do this in client
		if err != nil {
			return nil, err
		}
		u.Path = s.url.Path
		s.url = *u
	}

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
	flags      *flags
	segmentMap map[url.URL]struct{}

	// NewStream
	fsm FSM

	worker *Worker
}

func newStream() *Stream {
	return &Stream{
		oneShot:    make(chan struct{}, 1),
		imageChan:  make(chan image.Image, 100), // TODO magic size
		segmentMap: make(map[url.URL]struct{}),
	}
}
func (s *Stream) close() error { // TODO once
	close(s.oneShot)
	close(s.imageChan)
	return nil
}

func (s *Stream) Run(ctx context.Context) error {

	if s.worker != nil {
		if s.flags.Worker {
			err := s.worker.startWorker(ctx)
			if err != nil {
				return fmt.Errorf("runWorker %w", err)
			}
			return nil
		} else {
			err := s.worker.startChild(ctx)
			if err != nil {
				return fmt.Errorf("startChild %w", err)
			}
			var resp segment.Response
			err = s.worker.HandleSegment(&segment.Request{Filename: "jon"}, &resp)
			fmt.Println("dummy HandleSegment", resp.Label, len(resp.RawImages), err)
		}
	}

	defer s.close()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return s.consumeImages(ctx) })

	pollDuration := minPollDuration
	for {
		start := time.Now()
		mediapl, err := s.doPlaylist(ctx, &s.url)
		if err != nil {
			log.Println("doPlaylist", err)
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
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		log.Println("processPlaylist sleeping for", sleepFor)
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
	}
}
