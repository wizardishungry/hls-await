package stream

import (
	"context"
	"image"
	"image/color"
	"os"
	"sync"
	"sync/atomic"

	"github.com/WIZARDISHUNGRY/hls-await/internal/corpus"
	"github.com/WIZARDISHUNGRY/hls-await/internal/filter"
	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/eliukblau/pixterm/pkg/ansimage"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
)

const goimagehashDim = 8 // should be power of 2, color bars show noise at 16
var (                    // TODO move into struct
	singleImage      image.Image
	singleImageMutex sync.Mutex
)

func (s *Stream) consumeImages(ctx context.Context) error {
	log := logger.Entry(ctx)

	c, err := corpus.Load("testpatterns")
	if err != nil {
		return errors.Wrap(err, "corpus.Load")
	}

	filterFunc := filter.Multi(
		filter.Motion(goimagehashDim, s.flags.Threshold),
		filter.DefaultMinDistFromCorpus(c),
	)

	oneShot := false
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.oneShot:
			if s.flags.OneShot {
				oneShot = true
				log.Println("photo time!")
			}
		case img := <-s.imageChan:
			err := s.consumeImage(ctx, filterFunc, img, oneShot)
			if err != nil {
				log.WithError(err).Warn("consumeImag")
			}
		}
	}
}

func (s *Stream) consumeImage(ctx context.Context,
	filterFunc filter.FilterFunc,
	img image.Image,
	oneShot bool,
) error {
	log := logger.Entry(ctx)
	g, ctx := errgroup.WithContext(ctx)

	{ // not currently used
		singleImageMutex.Lock()
		singleImage = img
		singleImageMutex.Unlock()
	}

	g.Go(func() error {
		if oneShot {
			oneShot = false
			goto CLICK

		}
		if s.flags.AnsiArt == 0 {
			return nil
		}
		if s.frameCounter%s.flags.AnsiArt != 0 {
			return nil
		}
	CLICK:

		ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
		if err != nil {
			return errors.Wrap(err, "unix.IoctlGetWinsize")
		}
		ansi, err := ansimage.NewScaledFromImage(img, 8*int(ws.Col), 7*int(ws.Row), color.Black, ansimage.ScaleModeFit, ansimage.DitheringWithChars)
		if err != nil {
			return errors.Wrap(err, "ansimage.NewScaledFromImage")
		}
		if s.flags.Flicker {
			// TODO this is unimpressive now that images are fractioned
			log.Print("\033[H\033[2J") // flicker
		}
		ansi.Draw()

		return nil
	})

	g.Go(func() error {
		ok, err := filterFunc(ctx, img)
		if err != nil {
			return err
		}
		if ok {
			log.Tracef("[%d] passed filter", s.frameCounter)
			s.PushEvent(ctx, "unsteady")
		} else {
			s.PushEvent(ctx, "steady")
		}
		return nil
	})

	err := g.Wait()
	if err != nil {
		log.WithError(err).Warn("consumeImages")
	}

	if atomic.LoadInt32(&s.sendToBot) != 0 && s.bot != nil {
		s.bot.Chan() <- img
	}

	s.frameCounter++
	return nil
}
