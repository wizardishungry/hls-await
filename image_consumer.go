package main

import (
	"fmt"
	"image"

	"github.com/corona10/goimagehash"
)

const goimagehashDim = 8 // should be power of 2, color bars show noise at 16

func consumeImages(c <-chan image.Image) {
	var firstHash *goimagehash.ExtImageHash
	var firstHashAvg *goimagehash.ImageHash
	var i int
	for {
		select {
		case img := <-c:
			if img == nil {
				return
			}
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
				fmt.Printf("[%d] ExtPerceptionHash distance is %d\n", i, distance)
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
				fmt.Printf("[%d] AverageHash distance is %d\n", i, distance)
			}(img)
			i++
		}
	}
}
