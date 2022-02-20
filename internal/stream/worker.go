package stream

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

const WORKER_FD = 3 // stdin, stdout, stderr, ...

type Worker struct {
	once     sync.Once
	listener *net.UnixListener
	cmd      *exec.Cmd
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
		retErr = w.runWorker(ctx)
		if retErr == nil {
			go func() {
				// rpc serve here TODO
				log.Info("rpc serve in child")
			}()
		}
	})
	return retErr
}

// runWorker runs in separate process that communicates over ExtraFiles
func (w *Worker) runWorker(ctx context.Context) error {
	// log = log.WithField("child", true)
	f, err := fromFD(WORKER_FD)
	if err != nil {
		return err
	}
	defer f.Close()

	listener, err := net.FileListener(f)
	if err != nil {
		return fmt.Errorf("net.FileListener: %w", err)
	}
	w.listener = listener.(*net.UnixListener)

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

func (w *Worker) spawnChild(ctx context.Context) error {
	args := append([]string{}, os.Args[1:]...)
	args = append(args, "-worker")

	ul, err := net.ListenUnix("unix", &net.UnixAddr{})
	if err != nil {
		return err
	}

	if w.listener != nil {
		w.listener.Close()
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
	return nil
}

func (w *Worker) loop(ctx context.Context) {
	defer func() {
		if w.cmd != nil {
			w.cmd.Wait()
		}
	}()
	for ctx.Err() == nil {
		log.Warn("wait")
		err := w.cmd.Wait()
		log.Warn("done wait	")
		if errors.Is(err, context.Canceled) {
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
