package stream

import (
	"context"
	"image"
	"image/color"
	"os"
	"sync/atomic"

	"github.com/WIZARDISHUNGRY/hls-await/internal/corpus"
	"github.com/WIZARDISHUNGRY/hls-await/internal/filter"
	"github.com/WIZARDISHUNGRY/hls-await/internal/imagescore"
	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/eliukblau/pixterm/pkg/ansimage"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

const goimagehashDim = 8 // should be power of 2, color bars show noise at 16

func (s *Stream) consumeImages(ctx context.Context) error {
	log := logger.Entry(ctx)

	c, err := corpus.Load("testpatterns")
	if err != nil {
		return errors.Wrap(err, "corpus.Load")
	}

	bs := imagescore.NewBulkScore(ctx,
		func() imagescore.ImageScorer {
			return imagescore.NewJpegScorer()
		},
	)

	filterFunc := filter.Multi(
		filter.Motion(goimagehashDim, s.flags.Threshold),
		filter.DefaultMinDistFromCorpus(c),
		bs.Filter,
	)

	var frameCount int

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
			go func(count int) {
				err := s.consumeImage(ctx, filterFunc, img, oneShot, count)
				if err != nil {
					log.WithError(err).Warn("consumeImage")
				}
			}(frameCount)
			frameCount++
		}
	}
}

func (s *Stream) consumeImage(ctx context.Context,
	filterFunc filter.FilterFunc,
	img image.Image,
	oneShot bool,
	frameCount int,
) error {
	log := logger.Entry(ctx)

	entry := &outputImageEntry{
		counter: frameCount,
		done:    false,
		image:   img,
	}

	var sendToBot bool
	func() {
		s.outputImagesMutex.Lock()
		defer s.outputImagesMutex.Unlock()
		s.outputImages.Push(entry)
	}()
	defer func() {
		s.outputImagesMutex.Lock()
		defer s.outputImagesMutex.Unlock()
		entry.done = true // done
		entry.passedFilter = sendToBot

		for {
			if s.outputImages.Len() == 0 {
				return
			}
			entry := s.outputImages.Pop()
			if entry == nil {
				return
			}
			if !entry.done {
				s.outputImages.Push(entry)
				return
			}
			if entry.passedFilter {
				log.Tracef("[%d] passed filter", entry.counter)
				s.PushEvent(ctx, "unsteady")
			} else {
				log.Tracef("[%d] failed filter", entry.counter)
				s.PushEvent(ctx, "steady")
			}
			if entry.passedFilter && atomic.LoadInt32(&s.sendToBot) != 0 && s.bot != nil {
				s.bot.Chan() <- img
			}
		}
	}()

	err := func() error {
		if oneShot {
			oneShot = false
			goto CLICK

		}
		if s.flags.AnsiArt == 0 {
			return nil
		}
		if frameCount%(s.flags.AnsiArt) != 0 {
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
	}()
	if err != nil {
		log.WithError(err).Warn("consumeImage draw")
	}

	ok, err := filterFunc(ctx, img)
	if err != nil {
		return err
	}
	sendToBot = ok

	return nil
}
