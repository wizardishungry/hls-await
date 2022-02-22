package stream

import (
	"context"
	"io"
	"os"

	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
	"github.com/grafov/m3u8"
	"github.com/pkg/errors"
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
			// log.Println("skipping", *tsURL) // TODO use log level
			s.segmentMap[*tsURL] = struct{}{}
			continue
		}
		segCount++
		err = func() error {
			name := tsURL.String()
			log.Println("getting", name)
			tsResp, err := s.httpGet(ctx, name)
			if err != nil {
				return errors.Wrap(err, "httpGet")
			}
			defer tsResp.Body.Close()

			select {
			case <-ctx.Done():
				return nil
			default:
			}

			// tmpFile, err := os.CreateTemp("", "hls-await-")
			// if err != nil {
			// 	return errors.Wrap(err, "os.CreateTemp")
			// }
			// defer os.Remove(tmpFile.Name())
			// defer tmpFile.Close()

			// if _, err := io.Copy(tmpFile, tsResp.Body); err != nil {
			// 	return errors.Wrap(err, "io.Copy")
			// }
			// if _, err = tmpFile.Seek(0, 0); err != nil {
			// 	return errors.Wrap(err, "tmpFile.Seek")
			// }

			r, w, err := os.Pipe()
			if err != nil {
				return errors.Wrap(err, "os.Pipe")
			}
			defer r.Close()
			defer w.Close()
			if _, err := io.Copy(w, tsResp.Body); err != nil {
				return errors.Wrap(err, "io.Copy")
			}
			rFD := r.Fd()

			var request segment.Request // TODO support passing FDs or readers directly
			// request = &segment.FilenameRequest{Filename: tmpFile.Name()}
			request = &segment.FDRequest{FD: rFD} // TODO use os.Pipe value

			log.Println("processing ", name)
			s.ProcessSegment(ctx, request)
			log.Println("processed ", name)
			s.segmentMap[*tsURL] = struct{}{}
			return nil
		}()
		if err != nil {
			log.WithError(err).Error("processing segment")
		}
	}
	log.Println("segs processed", segCount)
	return nil
}
