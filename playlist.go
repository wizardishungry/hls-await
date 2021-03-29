package main

import (
	"bufio"
	"context"
	"fmt"

	"github.com/grafov/m3u8"
)

func processPlaylist(url string) {
	for {

	}
}

func doPlaylist(ctx context.Context, url string) (*m3u8.MediaPlaylist, error) {
	resp, err := httpGet(ctx, streamURL)
	p, listType, err := m3u8.DecodeFrom(bufio.NewReader(resp.Body), true)
	if err != nil {
		panic(err)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, err
	}
	if listType != m3u8.MEDIA {
		return nil, fmt.Errorf("playlist is not a media playlist %+v", listType)
	}
	return p.(*m3u8.MediaPlaylist), nil
}
