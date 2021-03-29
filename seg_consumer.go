package main

import (
	"net/url"
)

var segmentMap map[url.URL]struct{} = make(map[url.URL]struct{})

func consumeSegments(c <-chan url.URL) {
}
