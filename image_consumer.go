package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"sync"

	"github.com/corona10/goimagehash"
	"github.com/eliukblau/pixterm/pkg/ansimage"
)

const goimagehashDim = 8 // should be power of 2, color bars show noise at 16
var (
	firstHash          *goimagehash.ExtImageHash
	firstHashAvg       *goimagehash.ImageHash
	globalFrameCounter int
	singleImage        image.Image
	singleImageMutex   sync.Mutex
)

func consumeImages(ctx context.Context, c <-chan image.Image, cAnsi <-chan struct{}) {
	defer globalWG.Done()

	oneShot := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-cAnsi:
			if *flagOneShot {
				oneShot = true
				fmt.Println("photo time!")
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
				ansi, err := ansimage.NewFromImage(img, color.Black, ansimage.DitheringWithChars)
				if err != nil {
					fmt.Println(err)
				} else {
					if *flagFlicker {
						fmt.Print("\033[H\033[2J") // flicker
					}
					ansi.Draw()
				}
			}(img)
			func(img image.Image) {
				hash, err := goimagehash.ExtPerceptionHash(img, goimagehashDim, goimagehashDim)
				if err != nil {
					fmt.Println("consumeImages: ExtPerceptionHash error", err)
					return
				}
				if firstHash == nil {
					firstHash = hash
					return
				}
				distance, err := firstHash.Distance(hash)
				if err != nil {
					fmt.Println("consumeImages: ExtPerceptionHash Distance error", err)
					return
				}
				if distance >= *flagThreshold {
					firstHash = hash
					fmt.Printf("[%d] ExtPerceptionHash distance is %d\n", globalFrameCounter, distance)
					pushEvent("unsteady")
				} else {
					pushEvent("steady")
				}
			}(img)
			func(img image.Image) {
				hash, err := goimagehash.AverageHash(img)
				if err != nil {
					fmt.Println("consumeImages: AverageHash error", err)
					return
				}
				if firstHashAvg == nil {
					firstHashAvg = hash
					return
				}
				distance, err := firstHashAvg.Distance(hash)
				if err != nil {
					fmt.Println("consumeImages: AverageHash Distance error", err)
					return
				}
				if distance >= *flagThreshold {
					firstHashAvg = hash
					fmt.Printf("[%d] AverageHash distance is %d\n", globalFrameCounter, distance)
				}
			}(img)
			globalFrameCounter++
		}
	}
}
