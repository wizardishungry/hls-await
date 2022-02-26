module github.com/WIZARDISHUNGRY/hls-await

go 1.18

require (
	github.com/charlestamz/goav v1.5.4
	github.com/corona10/goimagehash v1.0.3
	github.com/dghubble/go-twitter v0.0.0-20211115160449-93a8679adecb
	github.com/dghubble/oauth1 v0.7.1
	github.com/die-net/lrucache v0.0.0-20190707192454-883874fe3947
	github.com/eliukblau/pixterm v1.3.1
	github.com/giorgisio/goav v0.1.0
	github.com/grafov/m3u8 v0.11.1
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79
	github.com/joho/godotenv v1.4.0
	github.com/looplab/fsm v0.2.0
	github.com/mattn/go-tty v0.0.4
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1
	golang.org/x/exp v0.0.0-20220218215828-6cf2b201936e
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20220222200937-f2425489ef4c
	golang.org/x/tools v0.1.9
	jonwillia.ms/roku v1.3.2
)

require (
	github.com/cenkalti/backoff v2.1.1+incompatible // indirect
	github.com/dghubble/sling v1.4.0 // indirect
	github.com/disintegration/imaging v1.6.2 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/koron/go-ssdp v0.0.2 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.10 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	golang.org/x/image v0.0.0-20210504121937-7319ad40d33e // indirect
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd // indirect
)

// https://github.com/dghubble/go-twitter/pull/148
replace github.com/dghubble/go-twitter => github.com/janisz/go-twitter v0.0.0-20201206102041-3fe237ed29f3
