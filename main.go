package main

import (
	"fmt"
	"log"
	"github.com/AlexEidt/Vidio"
	// "net/http"
	"os"
	"image"
	// "image/jpeg" // Registers JPEG decoder
	"image/png"
	"gonum.org/v1/gonum/mat"
	"math/cmplx"
)
// type Pixel struct {
// 	[3]uint8 // RGB
// }

type Frame struct {
	pixels []float64 // [r0, g0, b0, a0, r1, ...]
	width int
	height int
	channels int
}

func makeFrame(width int, height int, channels int) *Frame {
	pixels := make([]float64, width*height*channels)
	frm := Frame{pixels, width, height, channels}
	return &frm
}

func getGrayscaleImg(frame *Frame) image.Image {
	img := image.NewGray(image.Rect(0, 0, frame.GetWidth(), frame.GetHeight()))	
	intPixels := make([]uint8, len(frame.pixels))
	for i := range intPixels {
		intPixels[i] = uint8(frame.pixels[i]*255)
	}

	copy(img.Pix, intPixels)
	return img
}

func saveToPNG(img image.Image, outputPath string) {
	f, err := os.Create(outputPath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// Encode and save to file system
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
}

func (frm *Frame) GetHeight() int {
	return frm.height
}

func (frm *Frame) GetWidth() int {
	return frm.width
}

func (frm *Frame) GetChannels() int {
	return frm.channels
}

func (frm *Frame) GetPixelAt(x int, y int) []float64{
	index := y * frm.GetWidth() + x
	channels := frm.GetChannels()
	return frm.pixels[index*channels:index*channels+channels]
}

func (frm *Frame) SetPixelAt(x int, y int, value []float64){
	index := y * frm.GetWidth() + x
	channels := frm.GetChannels()
	copy(frm.pixels[index*channels:index*channels+channels], value) 
}

func (frm *Frame) Grayscale() *Frame {
	height := frm.GetHeight()
	width := frm.GetWidth()
	channels := 1
	grayfrm := makeFrame(width, height, channels)

	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			pixel := frm.GetPixelAt(x, y)
			grayValue := []float64{float64(0.299)*float64(pixel[0]) + float64(0.587)*float64(pixel[1]) + float64(0.114)*float64(pixel[2])}
			grayfrm.SetPixelAt(x, y, grayValue)
		}
	}
	return grayfrm
}

func (frm *Frame) FillFrame(bytes []float64) {
	for i, pixel := range bytes {
		frm.pixels[i] = pixel
	}
}

func (frm *Frame) Gradient(vertical bool) *Frame {
	width := frm.GetWidth()
	height := frm.GetHeight()
	channels := frm.GetChannels()
	gradFrm := makeFrame(width, height, channels)
	for c := 0; c < channels; c++ {
		if c == 3 {
			break
		}
		for x := 1; x < width-1; x++ {
			for y := 1; y < height-1; y++ {
				if vertical {
					Gy := (
						frm.GetPixelAt(x-1, y-1)[c] +
						2*frm.GetPixelAt(x, y-1)[c] +
						frm.GetPixelAt(x+1, y-1)[c] +
						-1*frm.GetPixelAt(x-1, y+1)[c] +
						-2*frm.GetPixelAt(x, y+1)[c] +
						-1*frm.GetPixelAt(x+1, y+1)[c])
					currentPixel := gradFrm.GetPixelAt(x,y)
					currentPixel[c] = Gy
					gradFrm.SetPixelAt(x,y,currentPixel) // should be unnecessary
				} else {
					Gx := (
						-1*frm.GetPixelAt(x-1, y-1)[c] +
						-2*frm.GetPixelAt(x-1, y)[c] +
						-1*frm.GetPixelAt(x-1, y+1)[c] +
						frm.GetPixelAt(x+1, y-1)[c] +
						2*frm.GetPixelAt(x+1, y)[c] +
						frm.GetPixelAt(x+1, y+1)[c])
					currentPixel := gradFrm.GetPixelAt(x,y)
					currentPixel[c] = Gx
					gradFrm.SetPixelAt(x,y,currentPixel) // should be unnecessary
				}
			}
		}
	}
	return gradFrm
}


func shiTomasi(frm *Frame, windowWidth int, windowHeight int) *Frame {
	// convert to grayscale
	grayFrm := frm.Grayscale()

	frameWidth := grayFrm.GetWidth()
	frameHeight := grayFrm.GetHeight()

	// find Ix Frame
	Ix := grayFrm.Gradient(false)
	Iy := grayFrm.Gradient(true)
	IxIy := makeFrame(frameWidth, frameHeight, 1)
	Ix2 := makeFrame(frameWidth, frameHeight, 1)
	Iy2 := makeFrame(frameWidth, frameHeight, 1)
	for x := 0; x < frameWidth; x++ {
		for y := 0; y < frameHeight; y++ {
			IxValue := Ix.GetPixelAt(x,y)[0]
			IyValue := Iy.GetPixelAt(x,y)[0]
			Ix2.SetPixelAt(x,y, []float64{IxValue*IxValue})
			Iy2.SetPixelAt(x,y, []float64{IyValue*IyValue})
			IxIy.SetPixelAt(x,y, []float64{IxValue*IyValue})
		}
	}

	// use grayscale frame to track R values
	rFrame := makeFrame(frameWidth, frameHeight, 1)

	maxR := float64(0)

	for wx := 0; wx < frameWidth/windowWidth; wx++ {
		for wy := 0; wy < frameHeight/windowHeight; wy++ {
			startX := wx*windowWidth
			startY := wy*windowHeight
			Ix2Sum := 0.0
			Iy2Sum := 0.0
			IxIySum := 0.0
			for x := startX; x < startX + windowWidth; x++ {
				for y := startY; y < startY + windowHeight; y++ {
					Ix2Sum += Ix2.GetPixelAt(x, y)[0]
					Iy2Sum += Iy2.GetPixelAt(x, y)[0]
					IxIySum += IxIy.GetPixelAt(x, y)[0]
				}
			}
			raw_matrix := []float64{Ix2Sum, IxIySum, IxIySum, Iy2Sum}
			matrix := mat.NewDense(2, 2, raw_matrix)

			var eig mat.Eigen
			if ok := eig.Factorize(matrix, mat.EigenRight); !ok {
				log.Fatal("Eigendecomposition failed to converge")
			}
			eigenvalues := eig.Values(nil)
			// e1 is x changes (vertical edges), e0 is y changes (horizontal edges)
			e0 := cmplx.Abs(eigenvalues[0])
			e1 := cmplx.Abs(eigenvalues[1])
			currentR := []float64{min(e0, e1)}
			maxR = max(maxR, currentR[0])

			for x := startX; x < startX + windowWidth; x++ {
				for y := startY; y < startY + windowHeight; y++ {
					rFrame.SetPixelAt(x,y, currentR)
				}
			}
		}
	}

	thresholdR := float64(maxR * 0.01)
	fmt.Printf("threshold %v\n", thresholdR)
	
	outputFrm := makeFrame(frameWidth, frameHeight, 1)
	outputFrm.FillFrame(grayFrm.pixels)
	for x := 0; x < outputFrm.GetWidth(); x++ {
		for y := 0; y < outputFrm.GetHeight(); y++ {
			if rFrame.GetPixelAt(x,y)[0] >= thresholdR {
				outputFrm.SetPixelAt(x,y, []float64{1, 1, 1, 1})
			}
		}
	}

	return outputFrm
}


func main() {
	// Open the video file
	// video, err := vidio.NewVideo("media/paper2.mp4")
	video, err := vidio.NewVideo("media/kitchen.mp4")
	if err != nil {
		log.Fatalf("Failed to open video: %v", err)
	}
	defer video.Close()
	fmt.Printf("Hi\n %v %v\n", video.Width(), video.Height())
	frame_width := video.Width()
	frame_height := video.Height()
	fmt.Printf("%v %v\n", frame_width, frame_height)
	frame_channels := 4

	// Loop through every frame in the video
	for video.Read() {
		// FrameBuffer returns a byte array of the frame in row-major order (RGB format)
		rawFrame := video.FrameBuffer()
		// frame is []uint8
		// [R0, G0, B0, A0, R1, ...]
		rawFrameFloats := make([]float64, len(rawFrame))
		for i := range rawFrameFloats {
			rawFrameFloats[i] = float64(rawFrame[i]) / 255.0
		}
		frame := makeFrame(frame_width, frame_height, frame_channels)
		frame.FillFrame(rawFrameFloats)

		withCorners := shiTomasi(frame, 5, 5)
		// grayFrame := frame.Grayscale()
		grayImg := getGrayscaleImg(withCorners)
		// saveToPNG(grayImg, "./second.png")
		saveToPNG(grayImg, "./first.png")
		break
	}
}