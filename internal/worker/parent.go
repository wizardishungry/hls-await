package worker

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

type Parent struct {
	once         sync.Once
	mutex        sync.RWMutex
	cmd          *exec.Cmd
	listener     *net.UnixListener
	client       *rpc.Client
	conn, connFD *net.UnixConn
}

func (w *Parent) Start(ctx context.Context) error {
	var retErr error
	w.once.Do(func() {
		retErr = w.spawnChild(ctx)
		if retErr == nil {
			go w.loop(ctx)
		}
	})
	return retErr
}

func (w *Parent) closeChild(ctx context.Context) error {
	// PRE: must own write mutex
	if w.client != nil {
		w.client.Close()
	}
	if w.conn != nil {
		w.conn.Close()
	}
	if w.connFD != nil {
		w.connFD.Close()
	}
	if w.listener != nil {
		w.listener.Close()
	}

	return nil
}

func (w *Parent) spawnChild(ctx context.Context) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.closeChild(ctx)

	args := append([]string{}, os.Args[1:]...)
	args = append(args, "-worker")

	// rpc
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

	conn2, err := net.DialUnix("unix", nil, ul.Addr().(*net.UnixAddr))
	if err != nil {
		return err
	}
	w.connFD = conn2

	w.client = rpc.NewClient(conn)

	return nil
}

func (w *Parent) loop(ctx context.Context) {
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

func (w *Parent) Handler() segment.Handler {
	return w
}

func (w *Parent) HandleSegment(request *segment.Request, resp *segment.Response) error {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	if w.client == nil {
		return errors.New("rpc client not set yet")
	}

	if fdr, ok := (*request).(*segment.FDRequest); ok {
		f := os.NewFile(fdr.FD, "whatever")
		err := unixmsg.SendFd(w.connFD, f)
		if err != nil {
			return errors.Wrap(err, "unixmsg.SendFd")
		}
		log.Infof("transmit fd %d", fdr.FD)
	}
	err := w.client.Call("GoAV.HandleSegment", request, resp)
	return err
}
