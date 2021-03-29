package main

import (
	"bufio"
	"context"
	"fmt"
	"image"
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

func processPlaylist(ctx context.Context, u *url.URL) {
	defer globalWG.Done()
	imageChan := make(chan image.Image)

	globalWG.Add(1)
	go consumeImages(ctx, imageChan)
	defer close(imageChan)

	pollDuration := minPollDuration
	for {
		mediapl, err := doPlaylist(ctx, u)
		if err != nil {
			fmt.Println("processPlaylist", err)
			pollDuration = minPollDuration
		} else {
			if dur := mediapl.TargetDuration; dur > 0 {
				tdDuration := time.Duration(dur * float64(time.Second))
				if tdDuration > minPollDuration {
					pollDuration = tdDuration
				}
				if tdDuration > maxPollDuration {
					pollDuration = maxPollDuration
				}
			}
			handleSegments(ctx, imageChan, u, mediapl)
		}
		timer := time.NewTimer(pollDuration)
		fmt.Println("processPlaylist sleeping", pollDuration)
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}
