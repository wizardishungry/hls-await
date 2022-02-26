package imagescore

import (
	"context"
	"image"
	"sync"
	"testing"
)

//go:generate sh -c "go test ./... -run '^$' -benchmem -bench . |tee benchresult.txt"

func BenchmarkBulk(b *testing.B) {
	const (
		xDim = 720
		yDim = 576
	)
	rect := image.Rectangle{Min: image.Point{}, Max: image.Point{X: xDim, Y: yDim}}

	for _, tC := range standardTestCases {
		b.Run(tC.desc, func(b *testing.B) {
			ctx := context.Background()
			bs := NewBulkScore(ctx, tC.scoreF)
			b.RunParallel(func(p *testing.PB) {
				for p.Next() {
					img := image.NewRGBA(rect)
					_, err := bs.ScoreImage(ctx, img)
					if err != nil {
						b.Fatalf("ScoreImage: %v", err)
					}
				}
			})

		})
	}

}

func FuzzBulk(f *testing.F) {
	f.Fuzz(func(t *testing.T, count uint16, xDim, yDim int, numWorkers uint8) {

		if xDim == 0 || yDim == 0 || count == 0 || numWorkers == 0 {
			t.Skip()
		}
		rect := image.Rectangle{Min: image.Point{}, Max: image.Point{X: xDim, Y: yDim}}

		ctx := context.Background()
		bs := NewBulkScore(ctx,
			func() ImageScorer { return NewPngScorer() },
		)

		var wg sync.WaitGroup
		wg.Add(int(numWorkers))
		for i := 0; i < int(numWorkers); i++ {
			go func() {
				defer wg.Done()
				for i := count; i < count/uint16(numWorkers); i++ {
					img := image.NewRGBA(rect)
					_, err := bs.ScoreImage(ctx, img)
					if err != nil {
						t.Fatalf("ScoreImage: %v", err)
					}
				}
			}()
		}
		wg.Wait()
	})
}
