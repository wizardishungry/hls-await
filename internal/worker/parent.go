package worker

import (
	"context"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/WIZARDISHUNGRY/hls-await/internal/segment"
	"github.com/WIZARDISHUNGRY/hls-await/pkg/unixmsg"
	"github.com/pkg/errors"
)

type Parent struct {
	once         sync.Once
	mutex        sync.RWMutex
	cmd          *exec.Cmd
	listener     *net.UnixListener
	client       *rpc.Client
	conn, connFD *net.UnixConn
	launchCount  int
	lastLaunch   time.Time
	context      context.Context
}

func (p *Parent) Start(ctx context.Context) error {

	var retErr error
	p.once.Do(func() {
		retErr = p.spawnChild(ctx)
		if retErr == nil {
			go p.loop(ctx)
		}
	})
	return retErr
}

func (p *Parent) closeChild(ctx context.Context) error {
	// PRE: must own write mutex
	if p.client != nil {
		p.client.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
	if p.connFD != nil {
		p.connFD.Close()
	}
	if p.listener != nil {
		p.listener.Close()
	}

	p.nicelyKill(ctx, p.cmd) // kick the process

	return nil
}

func (p *Parent) Restart(ctx context.Context) {
	p.mutex.RLock()
	cmd := p.cmd
	p.mutex.RUnlock()
	p.nicelyKill(ctx, cmd)
}

func (p *Parent) nicelyKill(ctx context.Context, cmd *exec.Cmd) {
	log := logger.Entry(ctx)
	// PRE: must own write mutex
	if cmd != nil && cmd.Process != nil {
		log.Info("Signaling child to exit")
		cmd.Process.Signal(syscall.SIGTERM)
		time.Sleep(3 * time.Second)
		cmd.Process.Kill()
	}
}

func (p *Parent) spawnChild(ctx context.Context) (err error) {
	log := logger.Entry(ctx)

	defer func() {
		l := log.WithField("count", p.launchCount)
		if !p.lastLaunch.IsZero() {
			l = l.WithField("lifetime", time.Now().Sub(p.lastLaunch))
		}
		p.lastLaunch = time.Now()
		l.WithError(err).Infof("spawnChild")
	}()
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.closeChild(ctx)

	args := append([]string{}, os.Args[1:]...)
	args = append(args, "-worker")

	// rpc
	ul, err := net.ListenUnix("unix", &net.UnixAddr{})
	if err != nil {
		return err
	}
	p.listener = ul

	f, err := ul.File()
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = setExtraFile(cmd, WORKER_FD, f)
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("couldn't spawn child: %w", err)
	}
	p.launchCount++
	p.cmd = cmd

	// NB: all streams will share the same dialed connection

	conn, err := net.DialUnix("unix", nil, ul.Addr().(*net.UnixAddr))
	if err != nil {
		return err
	}
	p.conn = conn

	conn2, err := net.DialUnix("unix", nil, ul.Addr().(*net.UnixAddr))
	if err != nil {
		return err
	}
	p.connFD = conn2

	p.client = rpc.NewClient(conn)

	return nil
}

func (p *Parent) loop(ctx context.Context) {
	log := logger.Entry(ctx)

	defer func() {
		p.mutex.Lock()
		defer p.mutex.Unlock()
		if p.cmd != nil {
			p.cmd.Wait()
		}
		p.mutex.Lock()
		defer p.mutex.Unlock()
		p.closeChild(ctx)
	}()
	for ctx.Err() == nil {
		p.mutex.RLock()
		cmd := p.cmd
		p.mutex.RUnlock()
		err := cmd.Wait()
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			break
		}

		log.WithError(err).WithField("exit_code", p.cmd.ProcessState.ExitCode()).Info("respawning child process")
		err = p.spawnChild(ctx)
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

func (w *Parent) Handler(ctx context.Context) segment.Handler {
	w.context = ctx
	return w
}

func (w *Parent) HandleSegment(request *segment.Request, resp *segment.Response) error {
	ctx := w.context
	log := logger.Entry(ctx)

	w.mutex.RLock()
	defer w.mutex.RUnlock()

	if w.client == nil {
		return errors.New("rpc client not set yet")
	}

	err := unixmsg.SendFd(w.connFD, request.FD)
	if err != nil {
		return errors.Wrap(err, "unixmsg.SendFd")
	}
	log.Infof("transmitted fd %d", request.FD)
	return w.client.Call("GoAV.HandleSegment", request, resp)
}
