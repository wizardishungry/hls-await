package main

import (
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFifo(t *testing.T) {
	mk, cleanup, err := MkFIFOFactory()
	require.NoError(t, err)
	defer func() {
		err := cleanup()
		require.NoError(t, err)
	}()

	pipe, cleanupPipe, err := mk()
	require.NoError(t, err)
	defer func() {
		err := cleanupPipe()
		require.NoError(t, err)
	}()
	require.FileExists(t, pipe)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		f, err := os.Create(pipe)
		require.NoError(t, err)
		_, err = f.WriteString("banana")
		require.NoError(t, err)
		err = f.Close()
		require.NoError(t, err)
		err = cleanupPipe() // safe to call multiple times
		// cleanupPipe = func() error { return nil }
		require.NoError(t, err)
	}()
	b, err := ioutil.ReadFile(pipe)
	require.NoError(t, err)
	require.NotEmpty(t, b)
	require.Equal(t, "banana", string(b))
	wg.Wait()
}
