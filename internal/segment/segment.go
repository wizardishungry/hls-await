package segment

import (
	"image"
)

type Handler interface {
	HandleSegment(request *Request, resp *Response) error // TODO
}
type Request struct {
	// Segment io.Reader
	Filename string
}
type Response struct {
	Label     string
	RawImages []*image.RGBA
}

func imagesToBitmaps(img *image.YCbCr) {

}
