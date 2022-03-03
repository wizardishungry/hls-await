package imagescore

import (
	"context"
	"fmt"
	"image"
	"testing"

	"github.com/WIZARDISHUNGRY/hls-await/internal/corpus"
)

var standardTestCases = []struct {
	desc   string
	scoreF func() ImageScorer
}{
	{
		desc:   "png",
		scoreF: func() ImageScorer { return NewPngScorer() },
	},
	{
		desc:   "gzip",
		scoreF: func() ImageScorer { return NewGzipScorer() },
	},
	{
		desc:   "jpeg",
		scoreF: func() ImageScorer { return NewJpegScorer() },
	},
	{
		desc:   "gif",
		scoreF: func() ImageScorer { return NewGifScorer() },
	},
}

func TestScoringAlgos(t *testing.T) {
	images := getTestingImages(t)

	for _, tC := range standardTestCases {
		t.Run(tC.desc, func(t *testing.T) {
			ctx := context.Background()
			scorer := tC.scoreF()

			for class, c := range images {
				for filename, img := range c.ImagesMap() {
					fmt.Printf("%s: %s/%s \n", tC.desc, class, filename)
					score, err := scorer.ScoreImage(ctx, img)
					if err != nil {
						t.Fatalf("ScoreImage(%s/%s): %v", class, filename, err)
					}
					fmt.Printf("%s: %s/%s %f\n", tC.desc, class, filename, score)
				}
			}

		})
	}
}

func BenchmarkScoreImage(b *testing.B) {
	const (
		xDim = 720
		yDim = 576
	)
	rect := image.Rectangle{Min: image.Point{}, Max: image.Point{X: xDim, Y: yDim}}

	for _, tC := range standardTestCases {
		b.Run(tC.desc, func(b *testing.B) {
			ctx := context.Background()
			bs := NewBulkScore(ctx, tC.scoreF)
			for n := 0; n < b.N; n++ {
				img := image.NewRGBA(rect)
				_, err := bs.ScoreImage(ctx, img)
				if err != nil {
					b.Fatalf("ScoreImage: %v", err)
				}
			}
		})
	}

}

func getTestingImages(t *testing.T) map[string]*corpus.Corpus {

	mustLoad := func(path string) *corpus.Corpus {
		c, err := corpus.Load(path)
		if err != nil {
			t.Fatalf("mustLoad:%v", err)
		}
		return c
	}

	return map[string]*corpus.Corpus{
		"interesting":   mustLoad("interesting"),
		"testpatterns":  mustLoad("interesting"),
		"uninteresting": mustLoad("interesting"),
	}
}
