package worker

import (
	"context"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"sync"

	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
	"github.com/WIZARDISHUNGRY/hls-await/internal/unixmsg"
	"github.com/pkg/errors"
)

type Child struct {
	once sync.Once
}

func (c *Child) Start(ctx context.Context) error {
	var retErr error
	c.once.Do(func() { // This should block and then error out
		retErr = c.runWorker(ctx)
	})
	return retErr
}

func (c *Child) Restart() {
	log.Fatalf("We should never be restarting a child worker.")
}

func (c *Child) runWorker(ctx context.Context) error {
	// log = log.WithField("child", true)
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

	for ctx.Err() == nil {
		// NB: this does not support multiple client connections, all clients share the same parent Worker
		// and only a single ffmpeg call will be running at a time
		err := func() error {

			fds := make(chan uintptr)
			defer close(fds)

			var wg sync.WaitGroup
			defer wg.Wait()

			server := rpc.NewServer()
			segApi := c.Handler().(*segment.GoAV)
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

func (c *Child) Handler() segment.Handler {
	return &segment.GoAV{
		VerboseDecoder: true, // TODO pass flags
		RecvUnixMsg:    true,
	}
}

func fromFD(fd uintptr) (f *os.File, err error) {
	f = os.NewFile(uintptr(fd), "unix")
	if f == nil {
		err = fmt.Errorf("nil for fd %d", fd)
	}
	return
}
