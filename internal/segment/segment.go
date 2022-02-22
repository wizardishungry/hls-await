package segment

import (
	"encoding/gob"
	"image"
	"io"
	"os"
)

type Handler interface {
	HandleSegment(request *Request, resp *Response) error // yes, an interface pointer as first arg, we'll try it!
}

type Request interface {
	Reader() (io.Reader, error)
}

var _ Request = &FilenameRequest{}

type FilenameRequest struct {
	// Segment io.Reader
	Filename string
}

func (fr *FilenameRequest) Reader() (io.Reader, error) {
	return os.Open(fr.Filename)
}

type Response struct {
	Label     string
	RawImages []*image.RGBA // TODO change back to image.Image an only conditionally convert to image.RGBA if using privsep
}

func init() {
	gob.Register(&FilenameRequest{})
}
