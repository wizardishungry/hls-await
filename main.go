package main

import (
	"bufio"
	"context"
	"fmt"
	"image/color"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"os"
	"unsafe"

	"github.com/eliukblau/pixterm/pkg/ansimage"
	"github.com/giorgisio/goav/avcodec"
	"github.com/giorgisio/goav/avformat"
	"github.com/giorgisio/goav/avutil"
	"github.com/giorgisio/goav/swscale"
	"github.com/grafov/m3u8"
)

func init() {
	avformat.AvRegisterAll()

}
func main() {

	const streamURL = "https://tv.nknews.org/tvhls/stream.m3u8"

	u, err := url.Parse(streamURL)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	client := &http.Client{}
	client.Jar, err = cookiejar.New(nil)
	if err != nil {
		panic(err)
	}

	httpGet := func(ctx context.Context, url string) (*http.Response, error) {
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
	resp, err := httpGet(ctx, streamURL)

	p, listType, err := m3u8.DecodeFrom(bufio.NewReader(resp.Body), true)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()
	fmt.Println("After 1st request:")
	for _, cookie := range client.Jar.Cookies(u) {
		fmt.Printf("  %s: %s\n", cookie.Name, cookie.Value)
	}

	switch listType {
	case m3u8.MEDIA:
		mediapl := p.(*m3u8.MediaPlaylist)
		var seg *m3u8.MediaSegment

		for _, mySeg := range mediapl.Segments {
			if mySeg == nil {
				continue
			}
			seg = mySeg
		}
		if seg != nil {
			tsURL, err := u.Parse(seg.URI)
			if err != nil {
				panic(err)
			}

			tsResp, err := httpGet(ctx, tsURL.String())
			if err != nil {
				panic(err)
			}
			ProcessFrame()
			tsResp.Body.Close()
			break
		}
	default:
		panic(listType)
	}
}

func ProcessFrame() {
	pFormatContext := avformat.AvformatAllocContext()
	if avformat.AvformatOpenInput(&pFormatContext, "x.ts", nil, nil) != 0 {
		log.Println("Error: Couldn't open file.")
		return
	}
	defer pFormatContext.AvformatCloseInput()

	// Retrieve stream information
	if pFormatContext.AvformatFindStreamInfo(nil) < 0 {
		log.Println("Error: Couldn't find stream information.")

		// Close input file and free context
		pFormatContext.AvformatCloseInput()
		return
	}

	// Dump information about file onto standard error
	pFormatContext.AvDumpFormat(0, "x.ts", 0)

	// Find the first video stream
	for i := 0; i < int(pFormatContext.NbStreams()); i++ {
		switch pFormatContext.Streams()[i].CodecParameters().AvCodecGetType() {
		case avformat.AVMEDIA_TYPE_VIDEO:

			// Get a pointer to the codec context for the video stream
			pCodecCtxOrig := pFormatContext.Streams()[i].Codec()
			// Find the decoder for the video stream
			pCodec := avcodec.AvcodecFindDecoder(avcodec.CodecId(pCodecCtxOrig.GetCodecId()))
			if pCodec == nil {
				fmt.Println("Unsupported codec!")
				os.Exit(1)
			}
			// Copy context
			pCodecCtx := pCodec.AvcodecAllocContext3()
			if pCodecCtx.AvcodecCopyContext((*avcodec.Context)(unsafe.Pointer(pCodecCtxOrig))) != 0 {
				fmt.Println("Couldn't copy codec context")
				os.Exit(1)
			}

			// Open codec
			if pCodecCtx.AvcodecOpen2(pCodec, nil) < 0 {
				fmt.Println("Could not open codec")
				os.Exit(1)
			}

			// Allocate video frame
			pFrame := avutil.AvFrameAlloc()

			// Allocate an AVFrame structure
			pFrameRGB := avutil.AvFrameAlloc()
			if pFrameRGB == nil {
				fmt.Println("Unable to allocate RGB Frame")
				os.Exit(1)
			}

			// Determine required buffer size and allocate buffer
			numBytes := uintptr(avcodec.AvpictureGetSize(avcodec.AV_PIX_FMT_RGB24, pCodecCtx.Width(),
				pCodecCtx.Height()))
			buffer := avutil.AvMalloc(numBytes)

			// Assign appropriate parts of buffer to image planes in pFrameRGB
			// Note that pFrameRGB is an AVFrame, but AVFrame is a superset
			// of AVPicture
			avp := (*avcodec.Picture)(unsafe.Pointer(pFrameRGB))
			avp.AvpictureFill((*uint8)(buffer), avcodec.AV_PIX_FMT_RGB24, pCodecCtx.Width(), pCodecCtx.Height())

			// initialize SWS context for software scaling
			swsCtx := swscale.SwsGetcontext(
				pCodecCtx.Width(),
				pCodecCtx.Height(),
				(swscale.PixelFormat)(pCodecCtx.PixFmt()),
				pCodecCtx.Width(),
				pCodecCtx.Height(),
				avcodec.AV_PIX_FMT_RGB24,
				avcodec.SWS_BILINEAR,
				nil,
				nil,
				nil,
			)

			// Read frames and save first five frames to disk
			frameNumber := 1
			packet := avcodec.AvPacketAlloc()
			for pFormatContext.AvReadFrame(packet) >= 0 {
				// Is this a packet from the video stream?
				if packet.StreamIndex() == i {
					// Decode video frame
					response := pCodecCtx.AvcodecSendPacket(packet)
					if response < 0 {
						fmt.Printf("Error while sending a packet to the decoder: %s\n", avutil.ErrorFromCode(response))
					}
					for response >= 0 {
						response = pCodecCtx.AvcodecReceiveFrame((*avcodec.Frame)(unsafe.Pointer(pFrame)))
						if response == avutil.AvErrorEAGAIN || response == avutil.AvErrorEOF {
							break
						} else if response < 0 {
							fmt.Printf("Error while receiving a frame from the decoder: %s\n", avutil.ErrorFromCode(response))
							return
						}

						if frameNumber <= 5 {
							// Convert the image from its native format to RGB
							swscale.SwsScale2(swsCtx, avutil.Data(pFrame),
								avutil.Linesize(pFrame), 0, pCodecCtx.Height(),
								avutil.Data(pFrameRGB), avutil.Linesize(pFrameRGB))

							// Save the frame to disk
							fmt.Printf("Writing frame %d\n", frameNumber)
							//SaveFrame(pFrameRGB, pCodecCtx.Width(), pCodecCtx.Height(), frameNumber)
							img, err := avutil.GetPicture(pFrame)
							if err != nil {
								fmt.Println(err)
							} else {
								fmt.Println("dim", img.Rect)
								// ansi, err := ansimage.New(40, 120, color.Black, ansimage.DitheringWithBlocks)
								ansi, err := ansimage.NewFromImage(img, color.Black, ansimage.DitheringWithBlocks)
								if err != nil {
									fmt.Println(err)
								} else {
									ansi.Draw()
								}
							}
						} else {
							return
						}
						frameNumber++
					}
				}

				// Free the packet that was allocated by av_read_frame
				packet.AvFreePacket()
			}

			// Free the RGB image
			avutil.AvFree(buffer)
			avutil.AvFrameFree(pFrameRGB)

			// Free the YUV frame
			avutil.AvFrameFree(pFrame)

			// Close the codecs
			pCodecCtx.AvcodecClose()
			(*avcodec.Context)(unsafe.Pointer(pCodecCtxOrig)).AvcodecClose()

			// Close the video file
			pFormatContext.AvformatCloseInput()

			// Stop after saving frames of first video straem
			break

		default:
			fmt.Println("Didn't find a video stream")

		}
	}
}

// SaveFrame writes a single frame to disk as a PPM file
func SaveFrame(frame *avutil.Frame, width, height, frameNumber int) {
	// Open file
	fileName := fmt.Sprintf("frame%d.ppm", frameNumber)
	file, err := os.Create(fileName)
	if err != nil {
		log.Println("Error Reading")
	}
	defer file.Close()

	// Write header
	header := fmt.Sprintf("P6\n%d %d\n255\n", width, height)
	file.Write([]byte(header))

	// Write pixel data
	for y := 0; y < height; y++ {
		data0 := avutil.Data(frame)[0]
		buf := make([]byte, width*3)
		startPos := uintptr(unsafe.Pointer(data0)) + uintptr(y)*uintptr(avutil.Linesize(frame)[0])
		for i := 0; i < width*3; i++ {
			element := *(*uint8)(unsafe.Pointer(startPos + uintptr(i)))
			buf[i] = element
		}
		file.Write(buf)
	}
}
