package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
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

const streamURL = "https://tv.nknews.org/tvhls/stream.m3u8"

func main() {
	mk, cleanup, err := MkFIFOFactory()
	if err != nil {
		panic(err)
	}
	defer func() {
		err := cleanup()
		if err != nil {
			fmt.Println("MkFIFOFactory()cleanup()", err)
		}
	}()

	imageChan := make(chan image.Image)
	go consumeImages(imageChan)
	defer close(imageChan)

	segmentChan := make(chan url.URL)
	go consumeSegments(segmentChan)
	defer close(segmentChan)

	u, err := url.Parse(streamURL)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	mediapl, err := doPlaylist(ctx, streamURL)
	if err != nil {
		panic(err)
	}
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
		path, cleanup, err := mk()
		if err != nil {
			fmt.Println("mkfifo", err)
			return
		}
		go func() {
			out, err := os.Create(path)
			if err != nil {
				fmt.Println("fifo os.Create", err)
				return
			}
			if i, err := io.Copy(out, tsResp.Body); err != nil {
				fmt.Println("fifo io.Copy", i, err)
				// return
			}
		}()
		fmt.Println("frame ", path)
		ProcessFrame(imageChan, path)
		if err := cleanup(); err != nil {
			fmt.Println("mkfifo cleanup", err)
		}
		tsResp.Body.Close()
	}
}

func init() {
	avformat.AvRegisterAll()
}

func ProcessFrame(imageChan chan image.Image, file string) {
	pFormatContext := avformat.AvformatAllocContext()
	// if avformat.AvformatOpenInput(&pFormatContext, "x.ts", nil, nil) != 0 {
	if avformat.AvformatOpenInput(&pFormatContext, file, nil, nil) != 0 {
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
	// pFormatContext.AvDumpFormat(0, "x.ts", 0)
	pFormatContext.AvDumpFormat(0, file, 0)

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
			packet := avcodec.AvPacketAlloc() // TODO sync.Pool
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

						if frameNumber <= 5000000000000000000 {
							// Convert the image from its native format to RGB
							swscale.SwsScale2(swsCtx, avutil.Data(pFrame),
								avutil.Linesize(pFrame), 0, pCodecCtx.Height(),
								avutil.Data(pFrameRGB), avutil.Linesize(pFrameRGB))

							// Save the frame to disk
							// fmt.Printf("Writing frame %d\n", frameNumber)
							//SaveFrame(pFrameRGB, pCodecCtx.Width(), pCodecCtx.Height(), frameNumber)
							img, err := avutil.GetPicture(pFrame)
							if err != nil {
								fmt.Println("get pic", err)
							} else {
								// dst := image.NewRGBA(img.Bounds()) // TODO use sync pool
								// draw.Draw(dst, dst.Bounds(), img, image.ZP, draw.Over)
								imageChan <- img

								// fmt.Println("dim", img.Rect)
								// ansi, err := ansimage.New(40, 120, color.Black, ansimage.DitheringWithBlocks)
								if frameNumber < 10 {
									ansi, err := ansimage.NewFromImage(img, color.Black, ansimage.DitheringWithBlocks)
									if err != nil {
										fmt.Println(err)
									} else {
										ansi.Draw()
									}
								}
							}
						} else {
							//goto DONE
						}
						frameNumber++
					}
				}

				// Free the packet that was allocated by av_read_frame
				packet.AvFreePacket()
			}
			avutil.AvFree(buffer)
			avutil.AvFrameFree(pFrameRGB)

			// Free the YUV frame
			avutil.AvFrameFree(pFrame)

			// Close the codecs
			pCodecCtx.AvcodecClose()
			(*avcodec.Context)(unsafe.Pointer(pCodecCtxOrig)).AvcodecClose()

			// Close the video file
			//pFormatContext.AvformatCloseInput() // TODO commented

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
