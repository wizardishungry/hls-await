package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
)

var client *http.Client

func init() {
	var err error
	client = &http.Client{}
	client.Jar, err = cookiejar.New(nil)
	if err != nil {
		panic(err)
	}

}

func httpGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://kcnawatch.org/korea-central-tv-livestream/")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Cookie", " __qca=P0-44019880-1616793366216; _ga=GA1.2.978268718.1616793363; _gid=GA1.2.523786624.1616793363")
	req.Header.Set("Accept-Language", "en-us")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0.1 Safari/605.1.15")
	// req.Header.Set("X-Playback-Session-Id", "F896728B-8636-4BB1-B4FF-1B235EB4ED9E")

	if s, err := httputil.DumpRequest(req, false); err != nil {
		panic(err)
	} else {
		fmt.Println(string(s))
	}

	resp, err := client.Do(req)
	if resp.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("bad http code %d", resp.StatusCode)
	}
	fmt.Println(resp.Header.Get("content-type"))

	if s, err := httputil.DumpResponse(resp, false); err != nil {
		panic(err)
	} else {
		fmt.Println(string(s))
	}
	return resp, err
}
