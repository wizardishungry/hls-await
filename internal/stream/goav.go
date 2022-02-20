package stream

import (
	"context"
	"fmt"
	"time"
	"unsafe"

	"github.com/charlestamz/goav/avcodec"
	"github.com/charlestamz/goav/avformat"
	"github.com/charlestamz/goav/avutil"
	"github.com/charlestamz/goav/swscale"
	old_avutil "github.com/giorgisio/goav/avutil"
)

func init() {
	// avformat.AvRegisterAll()
	avcodec.AvcodecRegisterAll()

}

func (s *Stream) ProcessSegment(ctx context.Context, file string) {
	defer fmt.Println("ProcessSegment")
	fmt.Println("start ProcessSegment")

	pFormatContext := avformat.AvformatAllocContext()
	// if avformat.AvformatOpenInput(&pFormatContext, "x.ts", nil, nil) != 0 {
	if avformat.AvformatOpenInput(&pFormatContext, file, nil, nil) != 0 {
		log.Println("Error: Couldn't open file.")
		return
	}
	defer avformat.AvformatCloseInput(pFormatContext)
	// Retrieve stream information
	if pFormatContext.AvformatFindStreamInfo(nil) < 0 {
		log.Println("Error: Couldn't find stream information.")
		return
	}

	// Dump information about file onto standard error
	if s.flags.VerboseDecoder {
		pFormatContext.AvDumpFormat(0, file, 0)
	}

	// Find the first video stream
FRAME_LOOP:
	for i := 0; i < int(pFormatContext.NbStreams()); i++ {
		switch pFormatContext.Streams()[i].CodecParameters().CodecType() {
		case avcodec.AVMEDIA_TYPE_VIDEO:

			// Get a pointer to the codec context for the video stream
			pCodecCtxOrig := pFormatContext.Streams()[i].Codec()
			fmt.Println(pFormatContext.Streams()[i].CodecParameters().CodecType())

			// Find the decoder for the video stream
			pCodec := avcodec.AvcodecFindDecoder(avcodec.CodecId(pCodecCtxOrig.GetCodecId()))
			if pCodec == nil {
				log.Println("Unsupported codec!")
				return
			}

			// Copy context
			pCodecCtx := pCodec.AvcodecAllocContext3()
			defer pCodecCtx.AvcodecClose()
			// params := &avcodec.CodecParameters{}
			// if avcodec.AvcodecParametersFromContext(params, (*avcodec.Context)(unsafe.Pointer(pCodecCtxOrig))) != 0 {
			// 	log.Println("Couldn't copy codec context: AvcodecParametersFromContext")
			// 	return
			// }
			if avcodec.AvcodecParametersToContext(pCodecCtx, pFormatContext.Streams()[i].CodecParameters()) != 0 {
				log.Println("Couldn't copy codec context: AvcodecParametersToContext")
				return
			}

			// Open codec
			if pCodecCtx.AvcodecOpen2(pCodec, nil) < 0 {
				log.Println("Could not open codec")
				return
			}

			// Allocate video frame
			pFrame := avutil.AvFrameAlloc()
			if pFrame == nil {
				log.Println("Unable to allocate Frame")
				return
			}
			defer avutil.AvFrameFree(pFrame)

			// Allocate an AVFrame structure
			pFrameRGB := avutil.AvFrameAlloc()
			if pFrameRGB == nil {
				log.Println("Unable to allocate RGB Frame")
				return
			}
			defer avutil.AvFrameFree(pFrameRGB)

			fmt.Println("xxx", pCodecCtx.Width(),
				pCodecCtx.Height())
			// Determine required buffer size and allocate buffer
			numBytes := avutil.AvImageGetBufferSize(avutil.AV_PIX_FMT_RGB24, pCodecCtx.Width(),
				pCodecCtx.Height(), 1)
			// buffer := avutil.AvMalloc(numBytes) //avutil.AvAllocateImageBuffer?
			buffer := avutil.AvAllocateImageBuffer(numBytes)
			defer avutil.AvFreeImageBuffer(buffer)

			// Assign appropriate parts of buffer to image planes in pFrameRGB
			// Note that pFrameRGB is an AVFrame, but AVFrame is a superset
			// of AVPicture
			// avp := (*avcodec.Picture)(unsafe.Pointer(pFrameRGB))
			data := (*[8]*uint8)(unsafe.Pointer(pFrameRGB.DataItem(0)))
			lineSize := (*[8]int32)(unsafe.Pointer(pFrameRGB.LinesizePtr()))

			avutil.AvImageFillArrays(*data, *lineSize, buffer, avutil.AV_PIX_FMT_RGB24, pCodecCtx.Width(), pCodecCtx.Height(), 1)
			// avp.AvpictureFill((*uint8)(buffer), avu til.AV_PIX_FMT_RGB24, pCodecCtx.Width(), pCodecCtx.Height())
			// initialize SWS context for software scaling

			swsCtx := swscale.SwsGetcontext(
				pCodecCtx.Width(),
				pCodecCtx.Height(),
				pCodecCtx.PixFmt(),
				pCodecCtx.Width(),
				pCodecCtx.Height(),
				avutil.AV_PIX_FMT_RGB24,
				swscale.SWS_BILINEAR,
				nil,
				nil,
				nil,
			)
			defer swscale.SwsFreecontext(swsCtx)

			// Read frames and save first five frames to disk
			frameNumber := 1
			packet := avcodec.AvPacketAlloc() // TODO sync.Pool
			defer avcodec.AvPacketFree(packet)

			log.Println("pFormatContext.AvReadFrame")

			for pFormatContext.AvReadFrame(packet) >= 0 {
				// Is this a packet from the video stream?
				if packet.StreamIndex() == i {
					// Decode video frame
					response := avcodec.AvcodecSendPacket(pCodecCtx, packet)
					if response < 0 {
						log.Printf("Error while sending a packet to the decoder: %s\n", avutil.AvStrerr(response))
					}
					for response >= 0 {
						response = avcodec.AvcodecReceiveFrame(pCodecCtx, pFrame)
						if response == avutil.AVERROR_EAGAIN || response == avutil.AVERROR_EOF {
							break
						} else if response < 0 {
							//log.Printf("Error while receiving a frame from the decoder: %s\n", avutil.ErrorFromCode(response))
							// return
							time.Sleep(time.Millisecond) // only seen as helpful on linux
							continue
						}

						if frameNumber <= 5000000000000000000 {
							// Convert the image from its native format to RGB
							data := (*[8]*uint8)(unsafe.Pointer(pFrame.DataItem(0)))
							lineSize := (*[8]int32)(unsafe.Pointer(pFrame.LinesizePtr()))
							dataDst := (*[8]*uint8)(unsafe.Pointer(pFrameRGB.DataItem(0)))
							lineSizeDst := (*[8]int32)(unsafe.Pointer(pFrameRGB.LinesizePtr()))

							response := swscale.SwsScale(swsCtx, *data,
								*lineSize, 0, 0,
								*dataDst, *lineSizeDst)
							if response < 0 {
								log.Printf("Error while SwsScale: %s\n", avutil.AvStrerr(response))
							}

							// Save the frame to disk
							// log.Printf("Writing frame %d\n", frameNumber)
							//SaveFrame(pFrameRGB, pCodecCtx.Width(), pCodecCtx.Height(), frameNumber)
							tmp := (*old_avutil.Frame)(unsafe.Pointer(pFrame))
							img, err := old_avutil.GetPicture(tmp)
							if err != nil {
								log.Println("get pic error", err)
							} else {
								// dst := image.NewRGBA(img.Bounds()) // TODO use sync pool
								// draw.Draw(dst, dst.Bounds(), img, image.ZP, draw.Over)
								select {
								case <-ctx.Done():
									return
								case s.imageChan <- img:
								}
							}
						}
						frameNumber++
					}
				}
			}
			log.Println("got some frames", frameNumber)
			// Stop after saving frames of first video straem
			break FRAME_LOOP

		default:
			log.Println("Didn't find a video stream")

		}
	}
}
