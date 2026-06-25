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
)
// type Pixel struct {
// 	[3]uint8 // RGB
// }

type Frame struct {
	pixels []float64
	width int
	height int
	channels uint8
}

func makeFrame(width int, height int, channels uint8) *Frame {
	pixels := make([]float64, width*height*int(channels))
	frm := Frame{pixels, width, height, channels}
	return &frm
}

func getGrayscaleImg(frame *Frame) image.Image {
	img := image.NewGray(image.Rect(0, 0, frame.GetWidth(), frame.GetHeight()))
	
	// Fast memory copy since image.Gray uses an internal []byte slice
	intPixels := make([]uint8, len(frame.pixels))
	for i := range intPixels {
		// if (i-1)%4 == 0 {
		// 	intPixels[i] = 1
		// 	continue
		// }
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

func (frm *Frame) GetChannels() uint8 {
	return frm.channels
}

func (frm *Frame) GetPixelAt(x int, y int) []float64{
	index := y * frm.GetWidth() + x
	channels := frm.GetChannels()
	return frm.pixels[index*int(channels):index*int(channels)+int(channels)]
}

func (frm *Frame) SetPixelAt(x int, y int, value []float64){
	index := y * frm.GetWidth() + x
	channels := frm.GetChannels()
	copy(frm.pixels[index*int(channels):index*int(channels)+int(channels)], value) 
}

func (frm *Frame) Grayscale() *Frame {
	height := frm.GetHeight()
	width := frm.GetWidth()
	var channels uint8 = 1
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
	for c := uint8(0); c < channels; c++ {
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
					// difference := []float64{top[0]-bottom[0], top[1]-bottom[1], top[2]-bottom[2], bottom[3]} // we don't want to change opacity
					currentPixel := gradFrm.GetPixelAt(x,y)
					currentPixel[c] = Gy
					gradFrm.SetPixelAt(x,y,currentPixel) // should be unnecessary
					// gradFrm.SetPixelAt(x,y, difference)
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
	Ix := frm.Gradient(false)
	Iy := frm.Gradient(true)

	// use grayscale frame to track R values
	rFrame := makeFrame(frameWidth, frameHeight, 1)

	maxR := float64(0)

	for wx := 0; wx < frameWidth/windowWidth; wx++ {
		for wy := 0; wy < frameHeight/windowHeight; wy++ {
			startX := wx*windowWidth
			startY := wy*windowHeight
			IxSum := 0.0
			IySum := 0.0
			for x := startX; x < startX + windowWidth; x++ {
				for y := startY; y < startY + windowHeight; y++ {
					IxSum += Ix.GetPixelAt(x, y)[0]
					IySum += Iy.GetPixelAt(x, y)[0]
				}
			}
			raw_matrix := []float64{IxSum*IxSum, IxSum*IySum, IxSum*IySum, IySum*IySum}
			matrix := mat.NewDense(2, 2, raw_matrix)

			var eig mat.Eigen
			if ok := eig.Factorize(matrix, mat.EigenRight); !ok {
				log.Fatal("Eigendecomposition failed to converge")
			}
			eigenvalues := eig.Values(nil)
			// e1 is x changes (vertical edges), e0 is y changes (horizontal edges)
			e0 := real(eigenvalues[0])
			e1 := real(eigenvalues[1])
			// currentR := []float64{e0}
			currentR := []float64{min(e0, e1)}
			// currentR := []float64{min(e0, e1)}
			maxR = max(maxR, currentR[0])

			for x := startX; x < startX + windowWidth; x++ {
				for y := startY; y < startY + windowHeight; y++ {
					rFrame.SetPixelAt(x,y, currentR)
				}
			}
			// fmt.Printf("eigen: %v\n", eigenvalues)
		}
	}

	thresholdR := float64(maxR * 0.1)
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




	// find Iy Frame

	// for each window (parameterized size), sum Ix^2, Iy^2, IxIy for 2x2 matrix

	// get Eigenvalues of the matrix and say R = min eigenvalue

	// mark all pixels as corners if R is high enough
}


func main() {
	// Open the video file
	video, err := vidio.NewVideo("media/kitchen.mp4")
	if err != nil {
		log.Fatalf("Failed to open video: %v", err)
	}
	defer video.Close()
	fmt.Printf("Hi\n %v %v\n", video.Width(), video.Height())
	frame_width := video.Width()
	frame_height := video.Height()
	var frame_channels uint8 = 4

	// Loop through every frame in the video
	for video.Read() {
		// FrameBuffer returns a byte array of the frame in row-major order (RGB format)
		rawFrame := video.FrameBuffer()
		// frame is []uint8
		// [R0, G0, B0, A0, R1, ...]
		rawFrameFloats := make([]float64, len(rawFrame))
		for i := range rawFrameFloats {
			// if (i-1)%4 == 0 {
			// 	rawFrameFloats[i] = 1.0
			// 	continue
			// }
			rawFrameFloats[i] = float64(rawFrame[i]) / 255.0
		}
		frame := makeFrame(frame_width, frame_height, frame_channels)
		// grayFrm := frm.Grayscale()
		

		frame.FillFrame(rawFrameFloats)

		withCorners := shiTomasi(frame, 5, 5)

		// grayFrame := frame.Grayscale()
		grayImg := getGrayscaleImg(withCorners)
		saveToPNG(grayImg, "./first.png")
		break


		// Process your frame here...
		// fmt.Printf("Frame read. Size: %d bytes\n", len(frame))
		// fmt.Printf("0 %v 1 %v 2 %v\n", frame[0], frame[1], frame[2])
		// // fmt.Printf("%T\n", frame)
		// break

	}
}