package imagescore

import (
	"context"
	"image"
	"runtime"

	"github.com/pkg/errors"
)

func NewBulkScore(ctx context.Context, scoreF func() ImageScorer) *BulkScore {
	numProcs := runtime.GOMAXPROCS(0)

	bs := &BulkScore{
		scoreF: scoreF,
		input:  make(chan bulkeScoreRequest, numProcs),
	}
	go bs.loops(ctx, numProcs)
	return bs
}

type BulkScore struct {
	scoreF func() ImageScorer
	input  chan bulkeScoreRequest
}

type bulkeScoreRequest struct {
	C   chan bulkScoreResult
	img image.Image
}
type bulkScoreResult struct {
	result float64
	err    error
}

func (bs *BulkScore) ScoreImage(ctx context.Context, img image.Image) (float64, error) {
	bsr := bulkeScoreRequest{
		img: img,
		C:   make(chan bulkScoreResult, 1),
	}
	select {
	case bs.input <- bsr:
	case <-ctx.Done():
		return -1, ctx.Err()
	}
	select {
	case res, ok := <-bsr.C:
		if !ok {
			return -1, errors.New("closed")
		}
		return res.result, res.err
	case <-ctx.Done():
		return -1, ctx.Err()
	}
}

func (bs *BulkScore) loops(ctx context.Context, numProcs int) {
	for i := 0; i < numProcs; i++ {
		go bs.loop(ctx)
	}
}

func (bs *BulkScore) loop(ctx context.Context) {
	scorer := bs.scoreF()
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-bs.input:
			score, err := scorer.ScoreImage(ctx, req.img)
			select {
			case <-ctx.Done():
				close(req.C)
				return
			case req.C <- bulkScoreResult{score, err}:
			}
		}
	}
}
