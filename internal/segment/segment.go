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
	Label string
	Pngs  [][]byte
}

func imagesToBitmaps(images []image.Image) {

}
