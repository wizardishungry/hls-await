package segment

import (
	"image"
	"image/draw"
	"image/png"
	"log"
	"sync"
	"time"
	"unsafe"

	"github.com/charlestamz/goav/avcodec"
	"github.com/charlestamz/goav/avformat"
	"github.com/charlestamz/goav/avutil"
	"github.com/charlestamz/goav/swscale"
	old_avutil "github.com/giorgisio/goav/avutil"
	"github.com/pkg/errors"
)

type GoAV struct {
	VerboseDecoder bool
}

var pngEncoder = &png.Encoder{
	CompressionLevel: png.NoCompression,
}

var _ Handler = &GoAV{}

var onceAvcodecRegisterAll sync.Once

func (goav *GoAV) HandleSegment(request *Request, resp *Response) error {

	onceAvcodecRegisterAll.Do(func() {
		avcodec.AvcodecRegisterAll() // only instantiate if we build a GoAV
	})

	if request.Filename == "jon" {
		resp = &Response{}
		resp.RawImages = []*image.RGBA{
			image.NewRGBA(image.Rect(0, 0, 100, 100)),
		}
		resp.Label = "ok"
		return nil
		return errors.New("jon rules")
	}
	var (
		file = request.Filename
	)

	pFormatContext := avformat.AvformatAllocContext()

	// if avformat.AvformatOpenInput(&pFormatContext, "x.ts", nil, nil) != 0 {
	if e := avformat.AvformatOpenInput(&pFormatContext, file, nil, nil); e != 0 {
		return errors.Wrap(goavError(e), "couldn't open file.")
	}
	defer avformat.AvformatCloseInput(pFormatContext)
	// Retrieve stream information
	if e := pFormatContext.AvformatFindStreamInfo(nil); e < 0 {
		return errors.Wrap(goavError(e), "couldn't find stream information.")
	}

	// Dump information about file onto standard error
	if goav.VerboseDecoder {
		pFormatContext.AvDumpFormat(0, file, 0)
	}

	// Find the first video stream
	for i := 0; i < int(pFormatContext.NbStreams()); i++ {
		switch pFormatContext.Streams()[i].CodecParameters().CodecType() {
		case avcodec.AVMEDIA_TYPE_VIDEO:

			// Get a pointer to the codec context for the video stream
			pCodecCtxOrig := pFormatContext.Streams()[i].Codec()
			// fmt.Println(pFormatContext.Streams()[i].CodecParameters().CodecType())

			// Find the decoder for the video stream
			pCodec := avcodec.AvcodecFindDecoder(avcodec.CodecId(pCodecCtxOrig.GetCodecId()))
			if pCodec == nil {
				return errors.New("unsupported codec")
			}

			// Copy context
			pCodecCtx := pCodec.AvcodecAllocContext3()
			defer pCodecCtx.AvcodecClose()

			if e := avcodec.AvcodecParametersToContext(pCodecCtx, pFormatContext.Streams()[i].CodecParameters()); e != 0 {
				return errors.Wrap(goavError(e), "coouldn't copy codec context: AvcodecParametersToContext")
			}

			// Open codec
			if e := pCodecCtx.AvcodecOpen2(pCodec, nil); e < 0 {
				return errors.Wrap(goavError(e), "coouldn't open codec")
			}

			// Allocate video frame
			pFrame := avutil.AvFrameAlloc()
			if pFrame == nil {
				return errors.New("unable to allocate Frame")

			}
			defer avutil.AvFrameFree(pFrame)

			// Allocate an AVFrame structure
			pFrameRGB := avutil.AvFrameAlloc()
			if pFrameRGB == nil {
				return errors.New("unable to allocate RGB Frame")
			}
			defer avutil.AvFrameFree(pFrameRGB)

			// Determine required buffer size and allocate buffer
			numBytes := avutil.AvImageGetBufferSize(avutil.AV_PIX_FMT_RGB24, pCodecCtx.Width(),
				pCodecCtx.Height(), 1)
			buffer := avutil.AvAllocateImageBuffer(numBytes)
			defer avutil.AvFreeImageBuffer(buffer)

			// Assign appropriate parts of buffer to image planes in pFrameRGB
			// Note that pFrameRGB is an AVFrame, but AVFrame is a superset
			// of AVPicture
			data := (*[8]*uint8)(unsafe.Pointer(pFrameRGB.DataItem(0)))
			lineSize := (*[8]int32)(unsafe.Pointer(pFrameRGB.LinesizePtr()))

			if e := avutil.AvImageFillArrays(*data, *lineSize, buffer, avutil.AV_PIX_FMT_RGB24, pCodecCtx.Width(), pCodecCtx.Height(), 1); e < 0 {
				return errors.Wrap(goavError(e), "coouldn't AvImageFillArrays")
			}

			// initialize SWS context for software scaling
			swsCtx := swscale.SwsGetcontext(
				pCodecCtx.Width(),
				pCodecCtx.Height(),
				pCodecCtx.PixFmt(),
				pCodecCtx.Width(),
				pCodecCtx.Height(),
				avutil.AV_PIX_FMT_RGB24,
				swscale.SWS_BILINEAR, nil,
				nil,
				nil,
			)
			defer swscale.SwsFreecontext(swsCtx)

			frameNumber := 0
			packet := avcodec.AvPacketAlloc()
			defer avcodec.AvPacketFree(packet)

			for pFormatContext.AvReadFrame(packet) >= 0 {
				// Is this a packet from the video stream?
				if packet.StreamIndex() == i {
					// Decode video frame
					response := avcodec.AvcodecSendPacket(pCodecCtx, packet)
					if response < 0 {
						return errors.Wrap(goavError(response), "error while sending a packet to the decoder")
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

						if true || frameNumber <= 5000000000000000000 { // TODO remove
							// Convert the image from its native format to RGB
							data := (*[8]*uint8)(unsafe.Pointer(pFrame.DataItem(0)))
							lineSize := (*[8]int32)(unsafe.Pointer(pFrame.LinesizePtr()))
							dataDst := (*[8]*uint8)(unsafe.Pointer(pFrameRGB.DataItem(0)))
							lineSizeDst := (*[8]int32)(unsafe.Pointer(pFrameRGB.LinesizePtr()))

							response := swscale.SwsScale(swsCtx, *data,
								*lineSize, 0, 0,
								*dataDst, *lineSizeDst)
							if response < 0 {
								return errors.Wrap(goavError(response), "error while SwsScale")
							}

							// Save the frame to disk
							// log.Printf("Writing frame %d\n", frameNumber)
							//SaveFrame(pFrameRGB, pCodecCtx.Width(), pCodecCtx.Height(), frameNumber)
							tmp := (*old_avutil.Frame)(unsafe.Pointer(pFrame))
							yimg, err := old_avutil.GetPicture(tmp)
							// img, err := old_avutil.GetPictureRGB(tmp) // Doesn't work

							// convert to RGBA because it serializes quickly
							img := image.NewRGBA(yimg.Rect)
							draw.Draw(img, yimg.Rect, yimg, image.Point{}, draw.Over)

							if err != nil {
								return errors.Wrap(err, "GetPicture")
							}
							resp.RawImages = append(resp.RawImages, img)
						}
						frameNumber++
					}
				}
			}
			log.Println("got some frames", frameNumber)
			// Stop after saving frames of first video straem
			if frameNumber > 0 {
				return nil
			}
		}
	}
	return errors.New("Didn't find a video stream")
}

func goavError(response int) error {
	return errors.New(avutil.AvStrerr(response))
}
