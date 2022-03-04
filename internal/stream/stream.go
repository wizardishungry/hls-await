package stream

import (
	"context"
	"fmt"
	"image"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/WIZARDISHUNGRY/hls-await/internal/bot"
	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	my_roku "github.com/WIZARDISHUNGRY/hls-await/internal/roku"
	"github.com/WIZARDISHUNGRY/hls-await/internal/worker"
	"github.com/WIZARDISHUNGRY/hls-await/pkg/proxy"
	"github.com/looplab/fsm"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"jonwillia.ms/roku"
)

type StreamOption func(s *Stream) error

func NewStream(opts ...StreamOption) (*Stream, error) {

	s := newStream()

	for _, opt := range opts {
		err := opt(s)
		if err != nil {
			return nil, err
		}
	}

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
	fsm        *FSM

	worker    worker.Worker
	bot       *bot.Bot
	sendToBot int32 // for atomic

	client       *http.Client
	frameCounter int
}

func newStream() *Stream {
	s := &Stream{
		oneShot:    make(chan struct{}, 1),
		imageChan:  make(chan image.Image, 100), // TODO magic size
		segmentMap: make(map[url.URL]struct{}),
	}
	return s
}
func (s *Stream) close() error { // TODO once
	close(s.oneShot)
	close(s.imageChan)
	return nil
}
func (s *Stream) OneShot() chan<- struct{} { return s.oneShot }

func (s *Stream) Run(ctx context.Context) error {

	log := logger.Entry(ctx)
	level, err := logrus.ParseLevel(s.flags.LogLevel)
	if err != nil {
		return err
	}
	log.Logger.SetLevel(level)

	s.fsm = s.newFSM(ctx)
	s.client = s.NewHttpClient(ctx)

	if s.flags.DumpFSM {
		fmt.Println(fsm.Visualize(s.GetFSM()))
		os.Exit(0)
	}

	err = s.worker.Start(ctx)
	if err != nil {
		return fmt.Errorf("%T.Start %w", s.worker, err)
	}

	defer s.close()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error { return s.consumeImages(ctx) })
	g.Go(func() error { return s.processPlaylist(ctx) })

	return g.Wait()
}

func (s *Stream) processPlaylist(ctx context.Context) error {
	log := logger.Entry(ctx)

	pollDuration := minPollDuration
	for {
		start := time.Now()
		mediapl, err := s.doPlaylist(ctx, s.url)
		if err != nil {
			log.WithError(err).Error("doPlaylist")
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
		log.Info("processPlaylist elapsed time", elapsed)
		log.Info("processPlaylist pollDuration", pollDuration)
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		log.Info("processPlaylist sleeping for", sleepFor)
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
