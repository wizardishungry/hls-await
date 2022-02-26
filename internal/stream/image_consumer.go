package stream

import (
	"context"
	"image"
	"image/color"
	"os"
	"sync"
	"sync/atomic"

	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/corona10/goimagehash"
	"github.com/eliukblau/pixterm/pkg/ansimage"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
)

const goimagehashDim = 8 // should be power of 2, color bars show noise at 16
var (                    // TODO move into struct
	firstHash          *goimagehash.ExtImageHash
	globalFrameCounter int
	singleImage        image.Image
	singleImageMutex   sync.Mutex
)

func (s *Stream) consumeImages(ctx context.Context) error {
	log := logger.Entry(ctx)

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
		case i := <-s.imageChan:
			img := i // shadow?
			g, _ := errgroup.WithContext(ctx)

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
				if globalFrameCounter%s.flags.AnsiArt != 0 {
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
				hash, err := goimagehash.ExtPerceptionHash(img, goimagehashDim, goimagehashDim)
				if err != nil {
					return errors.Wrap(err, "ExtPerceptionHash error")
				}
				if firstHash == nil {
					firstHash = hash
					return nil
				}
				distance, err := firstHash.Distance(hash)
				if err != nil {
					return errors.Wrap(err, "ExtPerceptionHash Distance error")
				}
				if distance >= s.flags.Threshold {
					log.Infof("[%d] ExtPerceptionHash distance is %d, threshold is %d\n", globalFrameCounter, distance, s.flags.Threshold) // TODO convert to Trace
					firstHash = hash
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

			globalFrameCounter++
		}
	}
}
