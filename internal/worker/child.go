package worker

import (
	"context"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
	"github.com/WIZARDISHUNGRY/hls-await/internal/unixmsg"
	"github.com/pkg/errors"
)

const (
	durWaitBeforeStopTheWorld = 2 * time.Second
	maxConsecutivePanics      = 2
)

type Child struct {
	once      sync.Once
	memstatsC chan error
}

func (c *Child) Start(ctx context.Context) error {
	var retErr error
	c.once.Do(func() { // This should block and then error out
		retErr = c.runWorker(ctx)
	})
	return retErr
}

func (c *Child) Restart(ctx context.Context) {
	log := logger.Entry(ctx)
	log.Fatalf("We should never be restarting a child worker.")
}

func (c *Child) runWorker(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	log := logger.Entry(ctx)
	f, err := fromFD(WORKER_FD)
	if err != nil {
		return err
	}
	defer f.Close()

	l, err := net.FileListener(f)
	if err != nil {
		return fmt.Errorf("net.FileListener: %w", err)
	}
	listener := l.(*net.UnixListener)
	go func() {
		// func (*ListenConfig) Listen is the way to make this abortable by context and we don't have that here
		<-ctx.Done()
		listener.Close()
	}()

	c.memstatsC = make(chan error, 1)
	go func() {
		bToMb := func(b uint64) float64 {
			return float64(b) / 1024 / 1024
		}
		getRss := func() (uint64, error) {
			buf, err := os.ReadFile("/proc/self/statm")
			if err != nil {
				return 0, err
			}

			fields := strings.Split(string(buf), " ")
			if len(fields) < 2 {
				return 0, errors.New("Cannot parse statm")
			}

			rss, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, err
			}

			return uint64(rss) * uint64(os.Getpagesize()), err
		}

		var (
			panicCount    int
			watchdogCount int
		)
		for {
			var (
				m   runtime.MemStats
				err error = nil
			)
			timer := time.NewTimer(30 * time.Second) // watchdog
			select {
			case <-ctx.Done():
				return
			case err = <-c.memstatsC: // not in the hot path to avoid stop the world while running
				if !timer.Stop() {
					<-timer.C
				}
				watchdogCount = 0
			case <-timer.C:
				watchdogCount++
			}

			if err != nil {
				panicCount++
				l := log.WithError(err).WithField("panic_count", panicCount)
				h := l.Error
				if panicCount > maxConsecutivePanics {
					h = l.Fatal
				}
				h("panicCounter")
			} else {
				panicCount = 0
			}

			const maxWatchdogCount = 4
			if watchdogCount > maxWatchdogCount {
				log.Fatalf("exceeded maxWatchdogCount(%d), exiting", maxWatchdogCount)
			}

			time.Sleep(durWaitBeforeStopTheWorld) // give a moment for the rpc to finish

			runtime.ReadMemStats(&m)
			rss, err := getRss()
			if err != nil {
				log.WithError(err).Error("getRss")
			}

			allocsF := bToMb(m.Alloc)
			rssF := bToMb(rss)

			f := log.Debugf

			const quota = 2000 // TOO move somewhere else; looks like the CGO stuff leaks :)
			if allocsF > quota || rssF > quota {
				f = log.Panicf // force child to restart (or perhaps use a panic handler)
			}
			f("alloc size %.2fmb; rss size %.2fmb", allocsF, rssF)
			runtime.GC()
		}
	}()

	for ctx.Err() == nil {
		// NB: this does not support multiple client connections, all clients share the same parent Worker
		// and only a single ffmpeg call will be running at a time
		err := func() error {

			fds := make(chan uintptr)
			defer close(fds)

			var wg sync.WaitGroup
			defer wg.Wait()

			server := rpc.NewServer()
			segApi := c.Handler(ctx).(*segment.GoAV)
			segApi.FDs = fds

			err = server.Register(segApi)
			if err != nil {
				log.WithError(err).Fatal("server.Register")
			}

			conn, err := listener.Accept()
			if err != nil {
				return errors.Wrap(err, "apiConn = listener.Accept")
			}
			apiConn := conn.(*net.UnixConn)

			wg.Add(1)
			go func() {
				defer wg.Done()
				server.ServeConn(apiConn)
			}()

			conn, err = listener.Accept()
			if err != nil {
				return errors.Wrap(err, "fdConn = listener.Accept")
			}

			wg.Add(1)
			go func() {
				defer wg.Done()

				fdConn := conn.(*net.UnixConn)
				for ctx.Err() == nil {

					fd, err := unixmsg.RecvFd(fdConn)

					if err != nil {
						log.WithError(err).Warn("unixmsg.RecvFd")
						return
					}
					log.Infof("unixmsg.RecvFd: %d", fd)
					// push fds into a channel, danger may deadlock
					select {
					case <-ctx.Done():
						return
					case fds <- fd:
					}
				}
			}()

			return nil
		}()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Child) Handler(ctx context.Context) segment.Handler {
	return &segment.GoAV{
		Context:        ctx,
		VerboseDecoder: true, // TODO pass flags
		RecvUnixMsg:    true,
		DoneCB:         c.doneCB,
	}
}

func (c *Child) doneCB(err error) {
	c.memstatsC <- err
}

func fromFD(fd uintptr) (f *os.File, err error) {
	f = os.NewFile(uintptr(fd), "unix")
	if f == nil {
		err = fmt.Errorf("nil for fd %d", fd)
	}
	return
}
