package stream

import (
	"context"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
	"github.com/WIZARDISHUNGRY/hls-await/internal/unixmsg"
	"github.com/pkg/errors"
)

const WORKER_FD = 3 // stdin, stdout, stderr, ...

// TODO split the server implmentation off the Worker struct

type Worker struct {
	mutex    sync.RWMutex
	once     sync.Once
	cmd      *exec.Cmd
	listener *net.UnixListener
	client   *rpc.Client
	conn     *net.UnixConn
}

func WithWorker(w *Worker) StreamOption {
	return func(s *Stream) error {
		s.worker = w
		return nil
	}
}

// startWorker runs in the child process
func (w *Worker) startWorker(ctx context.Context) error {
	var retErr error
	w.once.Do(func() {
		retErr = runWorker(ctx)
	})
	return retErr
}

// runWorker runs in separate process
func runWorker(ctx context.Context) error {
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
		<-ctx.Done()
		listener.Close()
	}()

	for ctx.Err() == nil {
		server := rpc.NewServer()
		segApi := &segment.GoAV{
			VerboseDecoder: true, // TODO pass flags
			RecvUnixMsg:    true,
		}

		err = server.Register(segApi)
		if err != nil {
			log.WithError(err).Fatal("server.Register")
		}

		unixConn, err := listener.Accept()
		if err != nil {
			return errors.Wrap(err, "listener.Accept")
		}
		uc := unixConn.(*net.UnixConn)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			for ctx.Err() == nil {

				f, err := unixmsg.RecvFd(uc)

				if err != nil {
					log.WithError(err).Warn("unixmsg.RecvFd")
					return
				}
				log.Infof("unixmsg.RecvFd: %d", f.Fd())
				// TODO push fds into a channel
			}
		}()
		server.ServeConn(unixConn)
		wg.Wait()
	}
	return nil
}

// startChild runs in the parent process
func (w *Worker) startChild(ctx context.Context) error {
	var retErr error
	w.once.Do(func() {
		retErr = w.spawnChild(ctx)
		if retErr == nil {
			go w.loop(ctx)
		}
	})
	return retErr
}

func (w *Worker) closeChild(ctx context.Context) error {
	// PRE: must own write mutex
	if w.client != nil {
		w.client.Close()
	}
	if w.conn != nil {
		w.conn.Close()
	}
	if w.listener != nil {
		w.listener.Close()
	}
	return nil
}

func (w *Worker) spawnChild(ctx context.Context) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.closeChild(ctx)

	args := append([]string{}, os.Args[1:]...)
	args = append(args, "-worker")

	ul, err := net.ListenUnix("unix", &net.UnixAddr{})
	if err != nil {
		return err
	}

	w.listener = ul

	f, err := ul.File()
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = setExtraFile(cmd, WORKER_FD, f)
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("couldn't spawn child: %w", err)
	}
	w.cmd = cmd

	conn, err := net.DialUnix("unix", nil, ul.Addr().(*net.UnixAddr))
	if err != nil {
		return err
	}
	w.conn = conn

	w.client = rpc.NewClient(conn)

	return nil
}

func (w *Worker) loop(ctx context.Context) {
	defer func() {
		w.mutex.Lock()
		defer w.mutex.Unlock()
		if w.cmd != nil {
			w.cmd.Wait()
		}
		w.mutex.Lock()
		defer w.mutex.Unlock()
		w.closeChild(ctx)
	}()
	for ctx.Err() == nil {
		w.mutex.RLock()
		cmd := w.cmd
		w.mutex.RUnlock()
		err := cmd.Wait()
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			break
		}

		log.WithError(err).WithField("exit_code", w.cmd.ProcessState.ExitCode()).Info("respawning child process")
		err = w.spawnChild(ctx)
		if err != nil {
			log.WithError(err).Error("spawn loop")
			time.Sleep(time.Second) // TODO change to backoff
		}
	}
}

func setExtraFile(cmd *exec.Cmd, fd int, f *os.File) error {
	extraFilesOffset := fd - 3 // stdin, stout, stderr, extrafiles...
	if len(cmd.ExtraFiles) != extraFilesOffset {
		return fmt.Errorf("len(cmd.ExtraFiles) != extraFilesOffset (%d != %d) ",
			len(cmd.ExtraFiles), extraFilesOffset)
	}
	cmd.ExtraFiles = append(cmd.ExtraFiles, f)
	return nil
}

func fromFD(fd uintptr) (f *os.File, err error) {
	f = os.NewFile(uintptr(fd), "unix")
	if f == nil {
		err = fmt.Errorf("nil for fd %d", fd)
	}
	return
}

var _ segment.Handler = &Worker{}

func (w *Worker) HandleSegment(request *segment.Request, resp *segment.Response) error {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	if w.client == nil {
		return errors.New("rpc client not set yet")
	}

	if fdr, ok := (*request).(*segment.FDRequest); ok {
		f := os.NewFile(fdr.FD, "whatever")
		err := unixmsg.SendFd(w.conn, f)
		if err != nil {
			return errors.Wrap(err, "unixmsg.SendFd")
		}
		log.Infof("transmit fd %d", fdr.FD)
	}
	err := w.client.Call("GoAV.HandleSegment", request, resp)
	return err
}
