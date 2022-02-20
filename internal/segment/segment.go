package segment

import (
	"context"
	"image"
)

type Handler interface {
	HandleSegment(ctx context.Context, request Request) (*Response, error) // TODO
}
type Request struct {
	// Segment io.Reader
	Filename string
}
type Response struct {
	Images []image.Image
}
