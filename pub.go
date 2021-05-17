package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/ThreeDotsLabs/watermill"
	wm "github.com/ThreeDotsLabs/watermill-http/pkg/http"
)

func NewPub() *wm.Publisher {
	t := &rt{Transport: http.DefaultTransport}
	client := &http.Client{
		Transport: t,
	}
	pub, err := wm.NewPublisher(wm.PublisherConfig{
		MarshalMessageFunc: wm.DefaultMarshalMessageFunc,
		Client:             client,
	}, watermill.NewStdLogger(false, false))
	if err != nil {
		panic(err)
	}
	return pub
}

type rt struct{ Transport http.RoundTripper }

var _ http.RoundTripper = &rt{}

func (t *rt) RoundTrip(r *http.Request) (*http.Response, error) {
	var hostURL, _ = url.Parse("http://127.0.0.1:8001")
	hostURL.Path = "/" + r.URL.Path
	r.URL = hostURL

	if *flagDumpHttp {
		if s, err := httputil.DumpRequest(r, false); err != nil {
			panic(err)
		} else {
			log.Println(string(s))
		}
	}

	return t.Transport.RoundTrip(r)
}
