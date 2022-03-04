package filter

import (
	"context"
	"image"

	"github.com/WIZARDISHUNGRY/hls-await/internal/corpus"
	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
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
	hashes := make(map[string]*goimagehash.ExtImageHash, len(c.Images()))
	for file, img := range c.ImagesMap() {
		hash, err := goimagehash.ExtPerceptionHash(img, dim, dim)
		if err != nil {
			panic(errors.Wrap(err, "goimagehash.ExtPerceptionHash"))
		}
		hashes[file] = hash
	}
	return func(ctx context.Context, img image.Image) (ok bool, err error) {
		log := logger.Entry(ctx)
		defer func() {
			log.Tracef("MinDistFromCorpus(%s): %v %v", c.Name(), ok, err)
		}()
		hash, err := goimagehash.ExtPerceptionHash(img, dim, dim)
		if err != nil {
			return false, errors.Wrap(err, "goimagehash.ExtPerceptionHash")
		}
		for file, corpusHash := range hashes {
			if ctx.Err() != nil {
				return true, ctx.Err()
			}
			dist, err := hash.Distance(corpusHash)
			if err != nil {
				return false, errors.Wrap(err, "hash.Distance")
			}
			if dist <= minDist && dist != 0 { // TODO temp adding a special 0 case here
				log.Tracef("MinDistFromCorpus(%s/%s): %v <= %v", c.Name(), file, dist, minDist)
				return false, nil // TODO: do we want verbose errors?
			}
		}
		return true, nil
	}
}
