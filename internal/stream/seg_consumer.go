package stream

import (
	"context"
	"io"
	"os"

	"github.com/grafov/m3u8"
)

func (s *Stream) handleSegments(ctx context.Context, mediapl *m3u8.MediaPlaylist) error {
	count := 0
	for _, seg := range mediapl.Segments {
		if seg == nil {
			continue
		}
		count++
	}
	log.Println("media segment count is", count)
	log.Println("fast start count is", s.flags.FastStart)
	if !s.flags.FastResume {
		defer func() { s.flags.FastStart = 0 }()
	}
	segs := mediapl.Segments
	segCount := 0
	for i, seg := range segs {
		if seg == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		tsURL, err := s.url.Parse(seg.URI)
		if err != nil {
			return err
		}
		if _, ok := s.segmentMap[*tsURL]; ok || (s.flags.FastStart > 0 && s.flags.FastStart+i < count) {
			// log.Println("skipping", *tsURL)
			s.segmentMap[*tsURL] = struct{}{}
			continue
		}
		segCount++
		func() {
			log.Println("getting", tsURL.String())
			tsResp, err := s.httpGet(ctx, tsURL.String())
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
			defer func() {
				select {
				case <-ctx.Done():
				case <-c:
				}
			}()

			go func() { // TODO use a worker pool?
				defer close(c)
				out, err := os.Create(path)
				if err != nil {
					log.Println("fifo os.Create", err)
					return
				}
				defer out.Close()

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
			s.ProcessSegment(ctx, path)
			s.segmentMap[*tsURL] = struct{}{}
		}()
	}
	log.Println("segs processed", segCount)
	return nil
}
