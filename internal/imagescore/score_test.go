package imagescore

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestScoringAlgos(t *testing.T) {
	images := getTestingImages(t)
	fmt.Println(images)
	testCases := []struct {
		desc string
	}{
		{
			desc: "",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			ctx := context.Background()
			scorer := NewPngScorer()

			for class, imageSlice := range images {
				for filename, img := range imageSlice {
					score, err := scorer.ScoreImage(ctx, img)
					if err != nil {
						t.Fatalf("ScoreImage(%s/%s): %v", class, filename, err)
					}
					fmt.Printf("%s %s: %f\n", class, filename, score)
				}
			}

		})
	}
}

func getTestingImages(t *testing.T) map[string]map[string]image.Image {
	images := make(map[string]map[string]image.Image)
	err := filepath.Walk("testdata/images", func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		img, err := png.Decode(f)
		class := filepath.Base(filepath.Dir(path))
		key := filepath.Base(path)
		classMap, ok := images[class]
		if !ok {
			classMap = make(map[string]image.Image)
		}
		classMap[key] = img
		images[class] = classMap
		return err
	})
	if err != nil {
		t.Fatalf("filepath.Walk: %v", err)
	}
	return images
}
