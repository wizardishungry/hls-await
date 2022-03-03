package filter

import (
	"context"
	"fmt"
	"image"

	"github.com/WIZARDISHUNGRY/hls-await/internal/corpus"
	"github.com/corona10/goimagehash"
	"github.com/pkg/errors"
)

const (
	defaultDim     = 16
	defaultMinDist = 12
)

func DefaultMinDistFromCorpus(c *corpus.Corpus) FilterFunc {
	return MinDistFromCorpus(c, defaultDim, defaultMinDist)
}

// MinDistFromCorpus returns a filter function that rejects images that fall under a threshold when
// comparing the ExtPerceptionHash against a corpus. It is safe for concurrent use.
func MinDistFromCorpus(c *corpus.Corpus, dim, minDist int) FilterFunc {
	hashes := make([]*goimagehash.ExtImageHash, 0, len(c.Images()))
	for _, img := range c.Images() {
		hash, err := goimagehash.ExtPerceptionHash(img, dim, dim)
		if err != nil {
			panic(errors.Wrap(err, "goimagehash.ExtPerceptionHash"))
		}
		hashes = append(hashes, hash)
	}
	return func(ctx context.Context, img image.Image) (bool, error) {
		hash, err := goimagehash.ExtPerceptionHash(img, dim, dim)
		if err != nil {
			return false, errors.Wrap(err, "goimagehash.ExtPerceptionHash")
		}
		for _, corpusHash := range hashes {
			if ctx.Err() != nil {
				return true, ctx.Err()
			}
			dist, err := hash.Distance(corpusHash)
			if err != nil {
				return false, errors.Wrap(err, "hash.Distance")
			}
			fmt.Println(dist)
			if dist <= minDist && dist != 0 { // TODO temp adding a special case here
				return false, nil // TODO: do we want verbose errors?
			}
		}
		return true, nil
	}
}
