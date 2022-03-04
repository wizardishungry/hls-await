package corpus

import (
	"embed"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/exp/maps"
)

//go:embed images/uninteresting images/testpatterns
var content embed.FS

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

func LoadFS(path string) (*Corpus, error) {
	c := &Corpus{
		name:   path,
		images: make(map[string]image.Image),
	}

	err := filepath.Walk(
		fmt.Sprintf("%s/images/%s", corpusRoot, path),
		func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return c.loadEntry(path, info)
		})

	return c, err
}

func LoadEmbedded(path string) (*Corpus, error) {
	c := &Corpus{
		name:   path,
		images: make(map[string]image.Image),
	}
	dirPath := fmt.Sprintf("images/%s", path)
	dir, err := content.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	for _, entry := range dir {
		if err := c.loadPath(content.Open, dirPath+"/"+entry.Name()); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *Corpus) loadPath(open func(name string) (fs.File, error), path string) error {
	f, err := open(path)
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
	return nil
}

func (c *Corpus) loadEntry(path string, info fs.FileInfo) error {
	if info.IsDir() {
		return nil
	}
	return c.loadPath(func(name string) (fs.File, error) {
		return os.Open(name)
	}, path)
}
