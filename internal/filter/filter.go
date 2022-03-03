package filter

import (
	"context"
	"image"
)

type FilterFunc func(context.Context, image.Image) (bool, error)

func Multi(fxns ...FilterFunc) FilterFunc {
	return func(ctx context.Context, img image.Image) (bool, error) {
		for _, fxn := range fxns {
			ok, err := fxn(ctx, img)
			if !ok || err != nil {
				return false, err
			}
		}
		return true, nil
	}
}
