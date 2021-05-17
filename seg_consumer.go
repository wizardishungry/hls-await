package main

import (
	"context"
	"image"
	"io"
	"net/url"
	"os"

	"github.com/grafov/m3u8"
)

var segmentMap map[url.URL]struct{} = make(map[url.URL]struct{})

func handleSegments(ctx context.Context, imageChan chan image.Image, u *url.URL, mediapl *m3u8.MediaPlaylist) {
	count := 0
	for _, seg := range mediapl.Segments {
		if seg == nil {
			continue
		}
		count++
	}
	log.Println("media segment count is", count)
	log.Println("fast start count is", *flagFastStart)
	if !*flagFastResume {
		defer func() { *flagFastStart = 0 }()
	}
	segs := mediapl.Segments
	segCount := 0
	for i, seg := range segs {
		if seg == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return
		default:
		}

		tsURL, err := u.Parse(seg.URI)
		if err != nil {
			panic(err)
		}
		if _, ok := segmentMap[*tsURL]; ok || (*flagFastStart > 0 && *flagFastStart+i < count) {
			// log.Println("skipping", *tsURL)
			segmentMap[*tsURL] = struct{}{}
			continue
		}
		segCount++
		func() {
			log.Println("getting", tsURL.String())
			tsResp, err := httpGet(ctx, tsURL.String())
			if err != nil {
				log.Println("httpGet", err)
				return
			}
			defer tsResp.Body.Close()

			path, cleanup, err := mk()
			if err != nil {
				log.Println("mkfifo", err)
				return
			}
			defer func() {
				if err := cleanup(); err != nil {
					log.Println("mkfifo cleanup", err)
				}
			}()

			c := make(chan struct{}, 0)
			defer func() { <-c }()

			go func() { // TODO use a worker pool?
				defer close(c)
				out, err := os.Create(path)
				if err != nil {
					log.Println("fifo os.Create", err)
					return
				}
				defer func() {
					if err := out.Close(); err != nil {
						log.Println("fifo os.Close", err)
					}
				}()
				if i, err := io.Copy(out, tsResp.Body); err != nil {
					log.Println("fifo io.Copy", i, err)
					// return
				}
			}()
			log.Println("frame ", path)
			ProcessFrame(ctx, imageChan, path)
			segmentMap[*tsURL] = struct{}{}
		}()
	}
	log.Println("segs processed", segCount)
}
