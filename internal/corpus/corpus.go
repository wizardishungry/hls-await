package corpus

import (
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/exp/maps"
)

type Corpus struct {
	name   string
	images map[string]image.Image
}

func (c *Corpus) Images() []image.Image {
	return maps.Values(c.images)
}
func (c *Corpus) ImagesMap() map[string]image.Image {
	return c.images
}
func (c *Corpus) Name() string {
	return c.name
}

var (
	_, b, _, _ = runtime.Caller(0)
	corpusRoot = filepath.Dir(b)
)

func Load(path string) (*Corpus, error) {
	c := &Corpus{
		name:   path,
		images: make(map[string]image.Image),
	}

	err := filepath.Walk(
		fmt.Sprintf("%s/images/%s", corpusRoot, path),
		func(path string, info fs.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			img, err := png.Decode(f)
			if err != nil {
				return err
			}
			key := filepath.Base(path)

			c.images[key] = img
			return err
		})

	return c, err
}
