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
	"sort"
	"math"
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

func (frm *Frame) GetPixelAt(x, y int) []float64{
	index := y * frm.GetWidth() + x
	channels := frm.GetChannels()
	return frm.pixels[index*channels:index*channels+channels]
}

func (frm *Frame) InterpolatePixelAt(x, y float64) []float64{
	// interpolate x value along top and bottom y
	// interpolate y value along those x values
	leftX := int(x)
	rightX := int(math.Ceil(x))
	if rightX == leftX {
		rightX++
	}
	topY := int(y)
	bottomY := int(math.Ceil(y))
	if bottomY == topY {
		bottomY++
	}

	topLeftValue := frm.GetPixelAt(leftX, topY)
	topRightValue := frm.GetPixelAt(rightX, topY)
	bottomLeftValue := frm.GetPixelAt(leftX, bottomY)
	bottomRightValue := frm.GetPixelAt(rightX, bottomY)

	c := frm.GetChannels()
	topInterpolated := make([]float64, c)
	bottomInterpolated := make([]float64, c)
	fullInterpolated := make([]float64, c)
	
	if c == 4 {
		topInterpolated[3] = 1
		bottomInterpolated[3] = 1
		fullInterpolated[3] = 1
	}


	xRatio := (x-float64(leftX))/(float64(rightX-leftX)) // always 1.0 on the bottom
	yRatio := (y-float64(topY))/(float64(bottomY-topY))
	
	for i := 0; i < c; i++ {
		if i == 3 {
			break
		}
		topInterpolated[i] = topLeftValue[i]*(1-xRatio) + topRightValue[i]*xRatio
		bottomInterpolated[i] = bottomLeftValue[i]*(1-xRatio) + bottomRightValue[i]*xRatio
		fullInterpolated[i] = topInterpolated[i]*(1-yRatio) + bottomInterpolated[i]*yRatio
	}
	// fmt.Printf("x %v y %v fullInterp %v\n",x, y, fullInterpolated)
	return fullInterpolated
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


func shiTomasi(frm *Frame, windowWidth, windowHeight int) [][2]float64 {
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

	thresholdR := float64(maxR * 0.1)
	fmt.Printf("threshold %v\n", thresholdR)
	
	intermediateCoords := make([][3]float64, 0)

	for wx := 0; wx < frameWidth/windowWidth; wx++ {
		for wy := 0; wy < frameHeight/windowHeight; wy++ {
			midX := wx*windowWidth + windowWidth/2
			midY := wy*windowHeight + windowHeight/2
			rValue := rFrame.GetPixelAt(midX, midY)[0] 
			if rValue >= thresholdR {
				intermediateCoords = append(intermediateCoords, [3]float64{float64(midX), float64(midY), rValue})
			}
		}
	}

	sort.Slice(intermediateCoords, func(i,j int) bool {
		return intermediateCoords[i][2] < intermediateCoords[j][2]
	})

	n := min(100, len(intermediateCoords))
	outputCoords := make([][2]float64, n)

	for i, c := range intermediateCoords[:n] {
		outputCoords[i] = [2]float64{c[0], c[1]}
	}
	// var outputCoords [][2]float64 = [][2]float64(intermediateCoords[:100][:2]) 
	return outputCoords

}

func lucasKanade(oldFrm, newFrm *Frame, windowWidth, windowHeight int, points [][2]float64) [][2]float64 {
	// will need to redo this and shiTomasi to have less repeated code
	/* for each window
	get the sum(Ix2), Sum(Iy2), sum(IxIy) from the old frame
	get the I_t by doing newFrame-oldFrame
	left matrix is sIx2, sIxIy, sIxIy, sIy2
	vector is u,v
	right matrix is -(sIxIt, sIyIt)
	solve for u,v; u gives x change, v gives y change
	use these to update the point information */
	grayOld := oldFrm.Grayscale()
	grayFrm := newFrm.Grayscale()

	Ix := grayFrm.Gradient(false)
	Iy := grayFrm.Gradient(true)
	
	IxIt := makeFrame(grayFrm.GetWidth(), grayFrm.GetHeight(), grayFrm.GetChannels())
	IyIt := makeFrame(grayFrm.GetWidth(), grayFrm.GetHeight(), grayFrm.GetChannels())
	Ix2 := makeFrame(grayFrm.GetWidth(), grayFrm.GetHeight(), grayFrm.GetChannels())
	Iy2 := makeFrame(grayFrm.GetWidth(), grayFrm.GetHeight(), grayFrm.GetChannels())
	IxIy := makeFrame(grayFrm.GetWidth(), grayFrm.GetHeight(), grayFrm.GetChannels())
	for x := range grayFrm.GetWidth() {
		for y := range grayFrm.GetHeight() {
			oldValue := grayOld.GetPixelAt(x,y)[0]
			newValue := grayFrm.GetPixelAt(x,y)[0]
			IxValue := Ix.GetPixelAt(x,y)[0]
			IyValue := Iy.GetPixelAt(x,y)[0]
			Ix2.SetPixelAt(x,y,[]float64{IxValue*IxValue})
			Iy2.SetPixelAt(x,y,[]float64{IyValue*IyValue})
			IxIy.SetPixelAt(x,y,[]float64{IxValue*IyValue})
			IxIt.SetPixelAt(x,y, []float64{(newValue-oldValue)*IxValue})
			IyIt.SetPixelAt(x,y, []float64{(newValue-oldValue)*IyValue})
		}
	}

	outputPoints := make([][2]float64, 0)

	for _, point := range points {
		x := point[0]
		y := point[1]
		sumIx2 := 0.0
		sumIy2 := 0.0
		sumIxIy := 0.0
		sumIxIt := 0.0
		sumIyIt := 0.0

		for wx := -windowWidth/2; wx <= windowWidth/2; wx++ {
			for wy := -windowHeight/2; wy <= windowHeight/2; wy++ {
				sumIx2 += Ix.InterpolatePixelAt(x,y)[0]
				sumIy2 += Iy.InterpolatePixelAt(x,y)[0]
				sumIxIy += IxIy.InterpolatePixelAt(x,y)[0]
				sumIxIt += IxIt.InterpolatePixelAt(x,y)[0]
				sumIyIt += IyIt.InterpolatePixelAt(x,y)[0]
			}
		}

		A := mat.NewDense(2,2, []float64{sumIx2, sumIxIy, sumIxIy, sumIy2})
		b := mat.NewVecDense(2, []float64{sumIxIt, sumIyIt})
		// fmt.Printf("A %v b %v\n", A, b)
		var uv mat.VecDense
		err := uv.SolveVec(A, b)
		if err != nil {
			// fmt.Printf("Error solving system: %v\n", err)
			continue
		}
		xChange := uv.RawVector().Data[0]
		yChange := uv.RawVector().Data[1]

		// fmt.Printf("uvRaw %v xChange %v yChange %v\n",uv.RawVector(), xChange, yChange)

		newX := x + xChange
		newY := y + yChange
		outputPoints = append(outputPoints, [2]float64{newX, newY})
	}

	return outputPoints
	
	
}

func displayST(frm *Frame, coords [][2]float64, windowWidth, windowHeight int) {
	withCorners := makeFrame(frm.GetWidth(), frm.GetHeight(), frm.GetChannels())
	withCorners.FillFrame(frm.pixels)
	for _, coord := range coords {
		midX := int(coord[0])
		midY := int(coord[1])
		for wx := -windowWidth/2; wx <= windowWidth/2; wx++ {
			for wy := -windowHeight/2; wy <= windowHeight/2; wy++ {
				withCorners.SetPixelAt(midX+wx, midY+wy, []float64{1,1,1,1})
			}
		}
	}

	grayFrame := withCorners.Grayscale()
	grayImg := getGrayscaleImg(grayFrame)
	saveToPNG(grayImg, "./display.png")

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
	counter := 0
	var oldFrm *Frame
	allCorners := make([][][2]float64,0)
	for video.Read() {
		// FrameBuffer returns a byte array of the frame in row-major order (RGBA format)
		rawFrame := video.FrameBuffer()
		rawFrameFloats := make([]float64, len(rawFrame))
		for i := range rawFrameFloats {
			rawFrameFloats[i] = float64(rawFrame[i]) / 255.0
		}
		frame := makeFrame(frame_width, frame_height, frame_channels)
		frame.FillFrame(rawFrameFloats)

		// if frame 0, get corners using shiTomasi; save f0 as "oldFrame" and continue
		// otherwise, use shiTomasi to get corners, do thisFrame-oldFrame for I_t
		// still using the Ix and Iy of the previous frame though
		if counter == 0 {
			corners := shiTomasi(frame, 5, 5)
			allCorners = append(allCorners, corners)
			// displayST(frame, corners, 5,5)
			oldFrm = frame

		} else {
			fmt.Printf("calling lk with corners %v\n", allCorners)
			corners := lucasKanade(oldFrm, frame, 5, 5, allCorners[len(allCorners)-1])
			fmt.Printf("new corners %v\n", corners)
			break
		}
		counter++


		


		// 
	}
}