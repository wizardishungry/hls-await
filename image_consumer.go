package main

import (
	"context"
	"image"
	"image/color"
	"os"
	"sync"

	"github.com/corona10/goimagehash"
	"github.com/eliukblau/pixterm/pkg/ansimage"
	"github.com/mattn/go-sixel"
)

const goimagehashDim = 16 // should be power of 2, color bars show noise at 16
var (
	firstHash          *goimagehash.ExtImageHash
	firstHashAvg       *goimagehash.ImageHash
	globalFrameCounter int
	singleImage        image.Image
	singleImageMutex   sync.Mutex
)

func consumeImages(ctx context.Context, c <-chan image.Image, cAnsi <-chan struct{}) {

	oneShot := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-cAnsi:
			if *flagOneShot {
				oneShot = true
				log.Println("photo time!")
			}
		case img := <-c:
			if img == nil {
				return
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
				if *flagAnsiArt == 0 {
					return
				}
				if globalFrameCounter%*flagAnsiArt != 0 {
					return
				}
			CLICK:
				var err error
				if *flagSixel {
					if *flagFlicker {
						log.Print("\033[H\033[2J") // flicker
					}
					err = sixel.NewEncoder(os.Stdout).Encode(img)
				} else {
					var ansi *ansimage.ANSImage

					ansi, err = ansimage.NewFromImage(img, color.Black, ansimage.DitheringWithChars)
					if err == nil {
						if *flagFlicker {
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
				if distance >= *flagThreshold {
					firstHash = hash
					pushEvent("unsteady")
				} else {
					pushEvent("steady")
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
				if distance >= *flagThreshold {
					firstHashAvg = hash
					// log.Printf("[%d] AverageHash distance is %d\n", globalFrameCounter, distance) // TODO convert to "verbose"
				}
			}(img)
			globalFrameCounter++
		}
	}
}
