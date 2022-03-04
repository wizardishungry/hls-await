package filter

import (
	"context"
	"image"

	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/corona10/goimagehash"
	"github.com/pkg/errors"
)

// Motion returns a filter function that rejects images that fall under a threshold when
// comparing the ExtPerceptionHash against the previous image hash.
func Motion(dim, minDist int) FilterFunc {
	var firstHash *goimagehash.ExtImageHash

	return func(ctx context.Context, img image.Image) (bool, error) {
		log := logger.Entry(ctx)

		hash, err := goimagehash.ExtPerceptionHash(img, dim, dim)
		if err != nil {
			return false, errors.Wrap(err, "ExtPerceptionHash error")
		}
		if firstHash == nil {
			firstHash = hash
			return true, nil
		}
		distance, err := firstHash.Distance(hash)
		if err != nil {
			return false, errors.Wrap(err, "ExtPerceptionHash Distance error")
		}
		ok := distance >= minDist
		if !ok {
			log.Tracef("ExtPerceptionHash distance is %d, threshold is %d\n", distance, minDist)
		}
		return ok, nil
	}
}
