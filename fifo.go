package main

import (
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"sync/atomic"
	"syscall"
)

type mkfifo func() (string, func() error, error)

func MkFIFOFactory() (mkfifo, func() error, error) {
	dir, err := ioutil.TempDir(os.TempDir(), "hls-await-")
	if err != nil {
		return nil, nil, err
	}
	var counter uint64
	return func() (string, func() error, error) {
			i := atomic.AddUint64(&counter, 1)
			dst := path.Join(dir, strconv.FormatUint(i, 16))
			err := syscall.Mkfifo(dst, 0600)
			if err != nil {
				return "", nil, err
			}
			return dst, func() error {
				if dst == "" {
					return nil
				}
				defer func() { dst = "" }()
				return os.Remove(dst)
			}, nil
		}, func() error {
			return os.RemoveAll(dir)
		}, nil
}
