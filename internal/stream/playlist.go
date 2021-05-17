package stream

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/grafov/m3u8"
)

const (
	minPollDuration = time.Second
	maxPollDuration = time.Minute
)

func doPlaylist(ctx context.Context, u *url.URL) (*m3u8.MediaPlaylist, error) {
	resp, err := httpGet(ctx, u.String())
	if err != nil {
		return nil, err
	}
	p, listType, err := m3u8.DecodeFrom(bufio.NewReader(resp.Body), true)
	if err != nil {
		return nil, err
	}
	if err := resp.Body.Close(); err != nil {
		return nil, err
	}
	if listType != m3u8.MEDIA {
		return nil, fmt.Errorf("playlist is not a media playlist %+v", listType)
	}
	return p.(*m3u8.MediaPlaylist), nil
}
