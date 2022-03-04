package stream

import (
	"context"
	"fmt"
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
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	goimagehashDim = 8    // should be power of 2, color bars show noise at 16
	imagescoreMin  = 0.06 // GZIP specific, calculated from output of TestScoringAlgos
)

// We picked gzip because it had the best results and good speed + low allocs
var imageScorerAlgo = imagescore.NewGzipScorer

func (s *Stream) consumeImages(ctx context.Context) error {
	log := logger.Entry(ctx)

	c, err := corpus.LoadEmbedded("testpatterns")
	if err != nil {
		return errors.Wrap(err, "corpus.Load")
	}

	bs := imagescore.NewBulkScore(ctx,
		func() imagescore.ImageScorer {
			return imageScorerAlgo()
		},
	)

	filterFunc := filter.Multi(
		filter.Motion(goimagehashDim, s.flags.Threshold),
		filter.DefaultMinDistFromCorpus(c),
		imagescore.Filter(bs, imagescoreMin),
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
			// TODO add timeout and concurrency control here
			go func(ctx context.Context, filterFunc filter.FilterFunc, img image.Image, oneShot bool, frameCount int) {
				err := s.consumeImage(ctx, filterFunc, img, oneShot, frameCount)
				if err != nil {
					log.WithError(err).Warn("consumeImage")
				}
			}(ctx, filterFunc, img, oneShot, frameCount)
			if oneShot {
				oneShot = false
			}
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

	withFrameCount := func(ctx context.Context, frameCount int) (context.Context, *logrus.Entry) {
		log := logger.Entry(ctx).WithField("frame_count", frameCount)
		logger.WithLogEntry(ctx, log)
		return ctx, log
	}
	ctx, log := withFrameCount(ctx, frameCount)

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
			ctx, log := withFrameCount(ctx, entry.counter)
			if !entry.done {
				s.outputImages.Push(entry)
				return
			}
			if entry.passedFilter {
				log.Trace("passed filter")
				s.PushEvent(ctx, "unsteady")
			} else {
				log.Trace("failed filter")
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
			fmt.Print("\033[H\033[2J") // flicker
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
