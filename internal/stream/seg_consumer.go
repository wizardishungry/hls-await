package stream

import (
	"context"
	"io"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
	"github.com/grafov/m3u8"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const segmentMaxDuration = 30 * time.Second

var maxPipeSize = func() int {
	if runtime.GOOS != "linux" {
		return -1
	}

	buf, err := os.ReadFile("/proc/sys/fs/pipe-max-size")
	if err != nil {
		err := errors.Wrapf(err, "get max pipesize: ReadFile")
		panic(err)
	}

	maxPipeSize, err := strconv.ParseInt(string(buf[:len(buf)-1]), 10, 64)
	if err != nil {
		err := errors.Wrapf(err, "get max pipesize: ParseInt")
		panic(err)
	}

	return int(maxPipeSize)
}()

func (s *Stream) handleSegments(ctx context.Context, mediapl *m3u8.MediaPlaylist) error {
	log := logger.Entry(ctx)

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
			ctx, cancel := context.WithTimeout(ctx, segmentMaxDuration)
			defer cancel()
			start := time.Now()
			name := tsURL.String()
			log.Infof("getting %s", name)
			tsResp, err := s.httpGet(ctx, name)
			if err != nil {
				return errors.Wrap(err, "httpGet")
			}
			getDone := time.Now().Sub(start)
			defer tsResp.Body.Close()

			if ctx.Err() != nil {
				return nil
			}

			r, w, err := os.Pipe()
			if err != nil {
				return errors.Wrap(err, "os.Pipe")
			}
			defer r.Close()
			defer w.Close()

			if maxPipeSize > 0 {
				i, err := unix.FcntlInt(w.Fd(), unix.F_SETPIPE_SZ, maxPipeSize)
				if err != nil {
					log.WithError(err).Errorf("F_SETPIPE_SZ: %d", i)
				}
			}

			var copyDur time.Duration
			go func() {
				copyStart := time.Now()
				if _, err := io.Copy(w, tsResp.Body); err != nil {
					log.WithError(err).Warn("io.Copy")
				}
				w.Close()
				copyDur = time.Now().Sub(copyStart)
			}()

			rFD := r.Fd()

			request := &segment.Request{FD: rFD}

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
	log.WithField("seg_count", segCount).Info("segs processed")
	return nil
}
