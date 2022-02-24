# hls-await

This is a work in progress monitor for [HLS](https://en.wikipedia.org/wiki/HTTP_Live_Streaming) streams
to detect activity via [perceptual hashing](https://en.wikipedia.org/wiki/Perceptual_hashing).

Currently it monitors a feed of [North Korea TV](https://kcnawatch.org/korea-central-tv-livestream/)
and [posts screencaps to Twitter](https://twitter.com/KCTV_bot).

## Features

- [ ] [Roku Stream Tester](http://devtools.web.roku.com/stream_tester/html/index.html) launching *partially*
    - [x] HTTP cache for segments *saves bandwidth*
- [x] Twitter
    - [ ] Command via DM *Increases rate, kill switch, change perceptual hashing threshold*
    - [ ] Resume threads on restart.
- [x] Process separation of ffmpeg / CGO. *This deserves a writeup!*
- [ ] Find the most interesting images *avoid blank screens*


## State Machine

![alt](./fsm.svg)