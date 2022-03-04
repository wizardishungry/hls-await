package imagescore

import (
	"context"
	"fmt"
	"image"
	"testing"

	"github.com/WIZARDISHUNGRY/hls-await/internal/corpus"
	"golang.org/x/exp/slices"
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
	{
		desc:   "png256",
		scoreF: func() ImageScorer { return NewDownsampler(NewPngScorer()) },
	},
	{
		desc:   "gzip256",
		scoreF: func() ImageScorer { return NewDownsampler(NewGzipScorer()) },
	},
	{
		desc:   "jpeg256",
		scoreF: func() ImageScorer { return NewDownsampler(NewJpegScorer()) },
	},
	{
		desc:   "gif256",
		scoreF: func() ImageScorer { return NewDownsampler(NewGifScorer()) },
	},
}

func TestScoringAlgos(t *testing.T) {
	images := getTestingImages(t)

	for _, tc := range standardTestCases {
		tC := tc // capture
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			scorer := tC.scoreF()
			var interestingMin, uninterestingMax float64
			var interestingMax, uninterestingMin float64

			for _, iC := range images {
				c := iC.corpus
				class := iC.desc
				scores := make([]float64, 0, len(c.ImagesMap()))
				for filename, img := range c.ImagesMap() {
					// fmt.Printf("%s: %s/%s \n", tC.desc, class, filename)
					score, err := scorer.ScoreImage(ctx, img)
					if err != nil {
						t.Fatalf("ScoreImage(%s/%s): %v", class, filename, err)
					}
					scores = append(scores, score)
					// fmt.Printf("%s: %s/%s %f\n", tC.desc, class, filename, score)
				}
				slices.Sort(scores)
				if class == "interesting" {
					interestingMin = scores[0]
					interestingMax = scores[len(scores)-1]
				}
				if class == "uninteresting" {
					uninterestingMin = scores[0]
					uninterestingMax = scores[len(scores)-1]
				}
				defer func(class string) {
					min := scores[0]
					max := scores[len(scores)-1]
					return
					fmt.Printf("%s/%s: %.4f %.4f\n", tC.desc, class, min, max)
				}(class)
			}
			diff := interestingMin - uninterestingMax
			// fmt.Printf("%s: %.4f - %.4f = %.4f\n", tC.desc, interestingMin, uninterestingMax, diff)
			scale := func(i float64) float64 { return 1.0 / (interestingMax - uninterestingMin) * (i - uninterestingMin) }
			diff = scale(interestingMin) - scale(uninterestingMax)
			_ = uninterestingMin
			fmt.Printf("%s scaled: %.4f - %.4f = %.4f\n", tC.desc, scale(interestingMin), scale(uninterestingMax), diff) // larger is better
			fmt.Printf("%s if you want to use it pick a threshold in this range: %.4f - %.4f\n", tC.desc, interestingMin, uninterestingMax)
		})
	}
}

func BenchmarkScoreImage(b *testing.B) {
	const (
		xDim = 720
		yDim = 576
	)
	rect := image.Rectangle{Min: image.Point{}, Max: image.Point{X: xDim, Y: yDim}}
	img := image.NewRGBA(rect)

	for _, tC := range standardTestCases {
		b.Run(tC.desc, func(b *testing.B) {
			ctx := context.Background()
			scorer := tC.scoreF()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				_, err := scorer.ScoreImage(ctx, img)
				if err != nil {
					b.Fatalf("ScoreImage: %v", err)
				}
			}
		})
	}

}

type imageClass struct {
	desc   string
	corpus *corpus.Corpus
}

func getTestingImages(t *testing.T) []imageClass {

	mustLoad := func(path string) *corpus.Corpus {
		c, err := corpus.LoadEmbedded(path)
		if err != nil {
			c, err = corpus.LoadFS(path)
			if err != nil {
				t.Fatalf("mustLoad:%v", err)
			}
		}
		return c
	}

	return []imageClass{
		{"interesting", mustLoad("interesting")},
		// {"testpatterns", mustLoad("testpatterns")},
		{"uninteresting", mustLoad("uninteresting")},
	}
}
