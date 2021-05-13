module github.com/WIZARDISHUNGRY/hls-await

go 1.16

require (
	github.com/ThreeDotsLabs/watermill v1.1.1
	github.com/ThreeDotsLabs/watermill-http v1.1.3
	github.com/corona10/goimagehash v1.0.3
	github.com/eliukblau/pixterm v1.3.1
	github.com/giorgisio/goav v0.1.0
	github.com/grafov/m3u8 v0.11.1
	github.com/looplab/fsm v0.2.0
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/stretchr/testify v1.7.0
	golang.org/x/image v0.0.0-20210504121937-7319ad40d33e // indirect
	jonwillia.ms/iot v0.0.0-00010101000000-000000000000
)

replace jonwillia.ms/iot => ../iot
