package stream

import (
	"context"
	"fmt"
	"image"
	"net/http"
	"net/url"
	"time"

	"github.com/WIZARDISHUNGRY/hls-await/internal/bot"
	my_roku "github.com/WIZARDISHUNGRY/hls-await/internal/roku"
	"github.com/WIZARDISHUNGRY/hls-await/internal/worker"
	"github.com/WIZARDISHUNGRY/hls-await/pkg/proxy"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"jonwillia.ms/roku"
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
	if !s.flags.Worker {
		target, err := s.url.Parse("/")
		if err != nil {
			return nil, err
		}
		u, err := proxy.NewSingleHostReverseProxy(context.TODO(), target, false)
		if err != nil {
			return nil, errors.Wrap(err, "NewSingleHostReverseProxy")
		}
		u.Path = s.url.Path
		s.url = u
	}

	return s, nil
}

func WithURL(u *url.URL) StreamOption {
	return func(s *Stream) error {
		s.url = u
		return nil
	}
}

type Stream struct {
	rokuCB        func() (*roku.Remote, error)
	url, proxyURL *url.URL

	oneShot    chan struct{}
	imageChan  chan image.Image
	flags      *flags
	segmentMap map[url.URL]struct{}
	fsm        FSM

	worker    worker.Worker
	bot       *bot.Bot
	sendToBot int32 // for atomic

	client *http.Client
}

func newStream() *Stream {
	s := &Stream{
		oneShot:    make(chan struct{}, 1),
		imageChan:  make(chan image.Image, 100), // TODO magic size
		segmentMap: make(map[url.URL]struct{}),
	}
	s.client = s.NewHttpClient()
	return s
}
func (s *Stream) close() error { // TODO once
	close(s.oneShot)
	close(s.imageChan)
	return nil
}
func (s *Stream) OneShot() chan<- struct{} { return s.oneShot }

func (s *Stream) Run(ctx context.Context) error {

	err := s.worker.Start(ctx) // TODO inject http proxy
	if err != nil {
		return fmt.Errorf("%T.Start %w", s.worker, err)
	}

	defer s.close()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return s.consumeImages(ctx) })

	pollDuration := minPollDuration
	for {
		start := time.Now()
		mediapl, err := s.doPlaylist(ctx, s.url)
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

func (s *Stream) LaunchRoku() error {
	remote, err := s.RokuCB()
	if err != nil {
		return err
	}
	return my_roku.On(remote, s.url.String())
}

func (s *Stream) RokuCB() (*roku.Remote, error) {
	return s.rokuCB()
}
