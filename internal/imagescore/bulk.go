package imagescore

import (
	"context"
	"image"
	"runtime"

	"github.com/WIZARDISHUNGRY/hls-await/internal/filter"
	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
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
	case res := <-bsr.C:
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
				return
			case req.C <- bulkScoreResult{score, err}:
			}
		}
	}
}

var _ filter.FilterFunc = nil

func (bs *BulkScore) Filter(ctx context.Context, img image.Image) (bool, error) {
	log := logger.Entry(ctx)
	score, err := bs.ScoreImage(ctx, img)
	if err != nil {
		return false, errors.Wrap(err, "bulkScorer.ScoreImage")
	}
	const minScore = 0.012 // TODO not great: jpeg specific
	if score < minScore {
		log.WithField("score", score).Trace("bulk score eliminated image")
		return false, nil
	}
	log.WithField("score", score).Trace("bulk score passed image")
	return true, nil
}
