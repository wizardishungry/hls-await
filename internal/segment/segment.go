package segment

import (
	"encoding/gob"
	"image"
)

type Handler interface {
	HandleSegment(request *Request, resp *Response) error // yes, an interface pointer as first arg, we'll try it!
}

type Response struct {
	Label     string
	RawImages []image.Image
}

func init() {
	gob.Register(&image.RGBA{}) // needed because thhis is container in interface
}

type Request struct {
	FD uintptr
}
