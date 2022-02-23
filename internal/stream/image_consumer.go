package stream

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"sync"

	"github.com/corona10/goimagehash"
	"github.com/eliukblau/pixterm/pkg/ansimage"
	"github.com/mattn/go-sixel"
	"golang.org/x/sys/unix"
)

const goimagehashDim = 16 // should be power of 2, color bars show noise at 16
var (                     // TODO move into struct
	firstHash          *goimagehash.ExtImageHash
	firstHashAvg       *goimagehash.ImageHash
	globalFrameCounter int
	singleImage        image.Image
	singleImageMutex   sync.Mutex
)

func (s *Stream) consumeImages(ctx context.Context) error {

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
			if img == nil {
				return nil
			}

			go func(img image.Image) {
				singleImageMutex.Lock()
				defer singleImageMutex.Unlock()
				singleImage = img
			}(img)

			func(img image.Image) {
				if oneShot {
					oneShot = false
					goto CLICK

				}
				if s.flags.AnsiArt == 0 {
					return
				}
				if globalFrameCounter%s.flags.AnsiArt != 0 {
					return
				}
			CLICK:
				var err error
				if s.flags.Sixel {
					if s.flags.Flicker {
						log.Print("\033[H\033[2J") // flicker
					}
					err = sixel.NewEncoder(os.Stdout).Encode(img)
				} else {
					var ansi *ansimage.ANSImage

					ws, wsErr := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
					if wsErr != nil {
						log.Println("IoctlGetWinsize: ", err)
						return
					}
					fmt.Println(ws.Col, ws.Row)
					ansi, err = ansimage.NewScaledFromImage(img, 8*int(ws.Col), 7*int(ws.Row), color.Black, ansimage.ScaleModeFit, ansimage.DitheringWithChars)
					// ansi, err = ansimage.NewFromImage(img, color.Black, ansimage.DitheringWithChars)
					if err == nil {
						if s.flags.Flicker {
							log.Print("\033[H\033[2J") // flicker
						}
						ansi.Draw()
					}
				}
				if err != nil {
					log.Println("ansi render err", err)
				}
			}(img)
			func(img image.Image) {
				hash, err := goimagehash.ExtPerceptionHash(img, goimagehashDim, goimagehashDim)
				if err != nil {
					log.Println("consumeImages: ExtPerceptionHash error", err)
					return
				}
				if firstHash == nil {
					firstHash = hash
					return
				}
				distance, err := firstHash.Distance(hash)
				if err != nil {
					log.Println("consumeImages: ExtPerceptionHash Distance error", err)
					return
				}
				// log.Printf("[%d] ExtPerceptionHash distance is %d\n", globalFrameCounter, distance) // TODO convert to "verbose"
				if distance >= s.flags.Threshold {
					firstHash = hash
					s.pushEvent("unsteady")
				} else {
					s.pushEvent("steady")
				}
			}(img)
			func(img image.Image) {
				hash, err := goimagehash.AverageHash(img)
				if err != nil {
					log.Println("consumeImages: AverageHash error", err)
					return
				}
				if firstHashAvg == nil {
					firstHashAvg = hash
					return
				}
				distance, err := firstHashAvg.Distance(hash)
				if err != nil {
					log.Println("consumeImages: AverageHash Distance error", err)
					return
				}
				if distance >= s.flags.Threshold {
					firstHashAvg = hash
					// log.Printf("[%d] AverageHash distance is %d\n", globalFrameCounter, distance) // TODO convert to "verbose"
				}
			}(img)
			globalFrameCounter++
		}
	}
}
