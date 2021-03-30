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

var oneShot = make(chan struct{}, 1)

func processPlaylist(ctx context.Context, u *url.URL) {
	defer globalWG.Done()
	imageChan := make(chan image.Image)

	globalWG.Add(1)
	go consumeImages(ctx, imageChan, oneShot)
	defer close(imageChan)

	pollDuration := minPollDuration
	for {
		start := time.Now()
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
		elapsed := time.Now().Sub(start)
		sleepFor := pollDuration - elapsed
		if sleepFor < minPollDuration {
			sleepFor = minPollDuration
		}
		timer := time.NewTimer(pollDuration)
		fmt.Println("processPlaylist elapsed time", elapsed)
		fmt.Println("processPlaylist pollDuration", pollDuration)
		fmt.Println("processPlaylist sleeping for", sleepFor)
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}
