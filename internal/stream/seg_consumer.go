package stream

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
	"github.com/grafov/m3u8"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
			start := time.Now()
			name := tsURL.String()
			log.Println("getting", name)
			tsResp, err := s.httpGet(ctx, name)
			if err != nil {
				return errors.Wrap(err, "httpGet")
			}
			getDone := time.Now().Sub(start)
			defer tsResp.Body.Close()

			select {
			case <-ctx.Done():
				return nil
			default:
			}

			r, w, err := os.Pipe()
			if err != nil {
				return errors.Wrap(err, "os.Pipe")
			}
			defer r.Close()
			defer w.Close()

			// i, err := unix.FcntlInt(w.Fd(), unix.F_SETPIPE_SZ, 1048576) // /proc/sys/fs/pipe-max-size
			// if err != nil {
			// 	log.WithError(err).Errorf("F_SETPIPE_SZ: %d", i)
			// }

			var copyDur time.Duration
			go func() {
				copyStart := time.Now()
				if _, err := io.Copy(w, tsResp.Body); err != nil {
					log.WithError(err).Warn("io.Copy")
				}
				w.Close()
				// TODO: not sure we can rely on this
				copyDur = time.Now().Sub(copyStart)
			}()

			rFD := r.Fd()

			var request segment.Request = &segment.FDRequest{FD: rFD}

			err = s.ProcessSegment(ctx, request) // TODO retries?
			processDone := time.Now().Sub(start)
			s.segmentMap[*tsURL] = struct{}{}

			log.WithFields(logrus.Fields{
				"get_dur":     getDone,
				"process_dur": (processDone - getDone),
				"overall_dur": processDone,
				"copy_dur":    copyDur,
			}).Infof("processed segment %s", name)

			return err
		}()
		if err != nil {
			log.WithError(err).Error("processing segment")
		}
	}
	log.Println("segs processed", segCount)
	return nil
}
