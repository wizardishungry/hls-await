package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/die-net/lrucache"
	"github.com/gregjones/httpcache"
)

const ttl = time.Hour
const maxBytes = 1024 * 1024 * 1024 * 1024 // 1gig

func NewSingleHostReverseProxy(ctx context.Context, target *url.URL) (*url.URL, error) {
	rp := httputil.NewSingleHostReverseProxy(target)
	old := rp.Director
	director := func(req *http.Request) {
		req.Header = make(http.Header)
		// TODO factor out
		req.Header.Set("Referer", "https://kcnawatch.org/korea-central-tv-livestream/") // TODO flag
		req.Header.Set("Accept", "*/*")
		// req.Header.Set("Cookie", " __qca=P0-44019880-1616793366216; _ga=GA1.2.978268718.1616793363; _gid=GA1.2.523786624.1616793363")
		req.Header.Set("Accept-Language", "en-us")
		req.Header.Set("Accept-Encoding", "identity")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0.1 Safari/605.1.15")
		// req.Header.Set("X-Playback-Session-Id", "F896728B-8636-4BB1-B4FF-1B235EB4ED9E")
		req.Header.Set("host", target.Host)
		fmt.Println(target, req.Host)
		req.Host = target.Host
		if /* *flagDumpHttp */ true { // TODO
			if s, err := httputil.DumpRequest(req, false); err != nil {
				panic(err)
			} else {
				fmt.Println(string(s))
			}
		}

		old(req)
	}
	rp.Director = director

	c := lrucache.New(maxBytes, int64(ttl.Seconds()))

	go func() {
		size := int64(-1)
		for ctx.Err() == nil {
			newSize := c.Size()
			time.Sleep(time.Second)
			if size != newSize {
				fmt.Printf("in memory cache: %d -> %d\n", size, newSize)
				size = newSize
			}
		}
	}()

	rp.Transport = httpcache.NewTransport(c)
	l, err := net.Listen("tcp", "127.0.0.1:") // TODO use outgoing socket addr
	if err != nil {
		return nil, err
	}
	go func() { http.Serve(l, rp) }()
	go func() {
		<-ctx.Done()
		l.Close()
	}()
	u := *target
	a := l.Addr().(*net.TCPAddr)
	u.Host = fmt.Sprintf("%s:%d", a.IP.String(), a.Port)
	u.Scheme = "http"
	return &u, nil
}
