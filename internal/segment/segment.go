package segment

import (
	"encoding/gob"
	"fmt"
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
	Filename string
}

func (fr *FilenameRequest) Reader() (io.Reader, error) {
	return os.Open(fr.Filename)
}

type Response struct {
	Label     string
	RawImages []image.Image
}

func init() {
	gob.Register(&FilenameRequest{})
	gob.Register(&FDRequest{})
	gob.Register(&image.RGBA{})
}

type FDRequest struct {
	FD uintptr
}

func (fdr *FDRequest) Reader() (io.Reader, error) {
	// TODO: DRY
	f := os.NewFile(uintptr(fdr.FD), "unix")
	if f == nil {
		return nil, fmt.Errorf("nil for fd %d", fdr.FD)
	}
	return f, nil
}
