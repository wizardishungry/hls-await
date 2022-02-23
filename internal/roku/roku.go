package roku

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"jonwillia.ms/roku"
)

var log *logrus.Logger = logrus.New() // TODO move onto struct

func Run(ctx context.Context) func() (*roku.Remote, error) {

	const dur = time.Minute
	var (
		mutex  sync.Mutex
		remote *roku.Remote
		errC   = make(chan error)
		timer  = time.NewTimer(time.Minute)
	)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
	LOOP:
		for ctx.Err() == nil {

			devs, err := roku.FindRokuDevices()
			switch {
			case len(devs) == 0:
				fallthrough
			case err != nil:
				log.WithError(err).Warn("roku.FindRokuDevices")
				time.Sleep(10 * time.Second) // TODO not abortable
				continue LOOP
			}
			dev := devs[0]
			log.Infof("found roku %s : %s", dev.Addr, dev.Name)
			r, err := roku.NewRemote(dev.Addr)
			if err != nil {
				log.WithError(err).Warn("roku.NewRemote")
				time.Sleep(10 * time.Second) // TODO not abortable
				continue LOOP
			}
			mutex.Lock()
			remote = r
			mutex.Unlock()
			select {
			case <-errC:
			case <-ctx.Done():
			case <-timer.C:
			}
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(dur)
		}

		return nil
	})
	// no g.Wait :)

	return func() (*roku.Remote, error) {
		mutex.Lock()
		defer mutex.Unlock()
		if remote == nil {
			return nil, errors.New("no roku")
		}

		return remote, nil
	}
}

func On(remote *roku.Remote, u string) error {
	fmt.Println("launch", u)

	err := remote.LaunchWithValues(&roku.App{Id: "63218", Name: "Roku Stream Tester"},
		url.Values{
			"live":          {"true"},
			"autoCookie":    {"true"},
			"debugVideoHud": {"false"},
			"url":           {u},
			"fmt":           {"HLS"},
			"drmParams":     {"{}"},
			"headers":       {`{"Referer":"https://kcnawatch.org/korea-central-tv-livestream/"}`}, // TODO
			"metadata":      {`{"isFullHD":false}`},
			"cookies":       {"[]"},
		},
	)
	if err != nil {
		return err
	}

	for i := 0; i < 10; i++ { // try to be a little quiet
		err := remote.VolumeDown()
		if err != nil {
			return err
		}
	}
	return nil
}
