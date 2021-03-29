package main

import (
	"context"
	"fmt"
	"image"
	"io"
	"net/url"
	"os"

	"github.com/grafov/m3u8"
)

var segmentMap map[url.URL]struct{} = make(map[url.URL]struct{})

func processSegments(ctx context.Context, u *url.URL, mediaplChan <-chan *m3u8.MediaPlaylist) {
}

func handleSegments(ctx context.Context, imageChan chan image.Image, u *url.URL, mediapl *m3u8.MediaPlaylist) {

	for _, seg := range mediapl.Segments {
		if seg == nil {
			continue
		}

		tsURL, err := u.Parse(seg.URI)
		if err != nil {
			panic(err)
		}
		if _, ok := segmentMap[*tsURL]; ok {
			fmt.Println("skipping", tsURL)
			continue
		}
		func() {
			tsResp, err := httpGet(ctx, tsURL.String())
			if err != nil {
				panic(err)
			}
			defer tsResp.Body.Close()

			path, cleanup, err := mk()
			if err != nil {
				fmt.Println("mkfifo", err)
				return
			}
			defer func() {
				if err := cleanup(); err != nil {
					fmt.Println("mkfifo cleanup", err)
				}
			}()

			go func() { // TODO use a worker pool?
				out, err := os.Create(path)
				if err != nil {
					fmt.Println("fifo os.Create", err)
					return
				}
				if i, err := io.Copy(out, tsResp.Body); err != nil {
					fmt.Println("fifo io.Copy", i, err)
					// return
				}
			}()
			fmt.Println("frame ", path)
			ProcessFrame(imageChan, path)
			segmentMap[*tsURL] = struct{}{}
		}()
	}

}
