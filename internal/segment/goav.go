package segment

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/WIZARDISHUNGRY/hls-await/internal/bot"
	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/charlestamz/goav/avcodec"
	"github.com/charlestamz/goav/avformat"
	"github.com/charlestamz/goav/avutil"
	"github.com/charlestamz/goav/swscale"
	old_avutil "github.com/giorgisio/goav/avutil"
	"github.com/pkg/errors"
)

const (
	minImages = 4
)

type GoAV struct {
	Context        context.Context
	VerboseDecoder bool
	RecvUnixMsg    bool // use a secondary unix socket to receive file descriptors in priv sep mode
	RestartOnPanic bool
	FDs            chan uintptr
	DoneCB         func(error)
}

var pngEncoder = &png.Encoder{
	CompressionLevel: png.NoCompression,
}

var _ Handler = &GoAV{}

var onceAvcodecRegisterAll sync.Once

func (goav *GoAV) HandleSegment(req *Request, resp *Response) (err error) {

	defer func() {

	}()

	ctx := goav.Context
	log := logger.Entry(ctx)

	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("unknown panic")
			}
			if err != nil {
				log.WithError(err).Error("HandleSegment panic handler")
			}
		}
		if goav.DoneCB != nil {
			goav.DoneCB(err)
		}
	}()

	defer func() { fractionImages(ctx, resp, err) }()

	onceAvcodecRegisterAll.Do(func() {
		avcodec.AvcodecRegisterAll() // only instantiate if we build a GoAV
		// go func() { // put this in here to test ffmpeg crashes
		// 	time.Sleep(10 * time.Second)
		// 	panic("test restarting")
		// }()
	})

	fd := req.FD
	if goav.RecvUnixMsg {
		var ok bool
		fd, ok = <-goav.FDs
		if !ok {
			return fmt.Errorf("fd channel closed")
		}
	}

	if fd <= 0 {
		return fmt.Errorf("fd is weird %d", fd)
	}

	defer func() {
		f := os.NewFile(fd, "whatever")
		if f != nil {
			f.Close()
		}
	}()

	// file := fmt.Sprintf("pipe:%d", fd)
	file := fmt.Sprintf("/proc/self/fd/%d", fd) // This is a Linuxism, but it works. Otherwise we get like 10% of images (30 instead of 248)

	pFormatContext := avformat.AvformatAllocContext()
	if pFormatContext == nil {
		return errors.New("pFormatContext == nil")
	}

	// if avformat.AvformatOpenInput(&pFormatContext, "x.ts", nil, nil) != 0 {
	if e := avformat.AvformatOpenInput(&pFormatContext, file, nil, nil); e != 0 {
		return errors.Wrap(goavError(e), "couldn't open file.")
	}
	defer avformat.AvformatCloseInput(pFormatContext)

	// Retrieve stream information (TODO: is this needeed)
	// var dict *avutil.Dictionary
	// if e := pFormatContext.AvformatFindStreamInfo(&dict); e < 0 {
	if e := pFormatContext.AvformatFindStreamInfo(nil); e < 0 {
		return errors.Wrap(goavError(e), "couldn't find stream information.")
	}
	// defer avutil.AvDictFree(&dict)

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
			fmt.Println(pFormatContext.Streams()[i].CodecParameters().CodecType())

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
							log.WithError(goavError(response)).Warn("Error while receiving a frame from the decoder")
							time.Sleep(time.Millisecond) // only seen as helpful on linux
							continue
						}

						// TODO: everything below this could be skipped if we could get the count ahead of time
						// pFormatContext.Streams()[i].NbFrames() returns 0 unfortunately

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

						tmp := (*old_avutil.Frame)(unsafe.Pointer(pFrame))
						// img, err := old_avutil.GetPictureRGB(tmp) // Doesn't work
						yimg, err := old_avutil.GetPicture(tmp)

						if err != nil {
							return errors.Wrap(err, "GetPicture")
						}

						const ( // constrain weird green box
							Xdim = 720
							Ydim = 576
						)

						constraint := yimg.Rect
						if constraint.Max.X > Xdim {
							constraint.Max.X = Xdim
						}
						if constraint.Max.Y > Ydim {
							constraint.Max.Y = Ydim
						}
						// convert to RGBA because it serializes easily
						img := image.NewRGBA(constraint) // TODO: no need to convert to RGBA when running in-process & crop with a zero copy (img.SubImage())
						draw.Draw(img, yimg.Rect, yimg, image.Point{}, draw.Over)
						resp.RawImages = append(resp.RawImages, img)

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

func fractionImages(ctx context.Context, resp *Response, err error) {
	log := logger.Entry(ctx)
	if err == nil && resp != nil {
		initLen := len(resp.RawImages)
		limitImageCount := float64(initLen) * bot.ImageFraction
		if limitImageCount <= minImages || limitImageCount >= float64(initLen) {
			return
		}
		mod := int(float64(initLen) / limitImageCount)
		newImages := make([]image.Image, 0, int(limitImageCount))
		for i, img := range resp.RawImages {
			if i%mod == 0 {
				newImages = append(newImages, img)
			}
		}
		resp.RawImages = newImages
		log.Infof("fractionImages %d -> %d", initLen, len(resp.RawImages))
	}
}
