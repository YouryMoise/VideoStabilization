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
	if frm.GetChannels() == 1 {
		return frm
	}
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

func (frm *Frame) Convolve(kernel [][]float64) *Frame {
	// changes size
	kernelWidth := len(kernel[0])
	kernelHeight := len(kernel)
	outputWidth := frm.GetWidth()
	outputHeight := frm.GetHeight()
	outputChannels := frm.GetChannels()
	outputFrm := makeFrame(outputWidth, outputHeight, outputChannels)
	for x := 0; x < outputWidth; x++ {
		for y := 0; y < outputHeight; y++ {
			outputValue := []float64{0.0, 0.0, 0.0, 0.0}
			for wx := -kernelWidth/2; wx <= kernelWidth/2; wx++ {
				targetX := x + wx
				if targetX < 0 {
					targetX += outputWidth
				}
				if targetX >= outputWidth {
					targetX -= outputWidth
				}
				for wy := -kernelHeight/2; wy <= kernelHeight/2; wy++ {
					targetY := y + wy
					if targetY < 0 {
						targetY += outputHeight
					}
					if targetY >= outputHeight {
						targetY -= outputHeight
					}
					// fmt.Printf("x %v y %v\n", targetX, targetY)
					originalPixel := frm.GetPixelAt(targetX, targetY)
					for c := 0; c < int(math.Min(float64(outputChannels), 3)); c++ {
						outputValue[c] += float64(originalPixel[c])*kernel[wy+kernelHeight/2][wx+kernelWidth/2]
					}
					
				}
			}
			outputFrm.SetPixelAt(x,y,outputValue)
		}
	}
	return outputFrm
}

func (frm *Frame) Blur() *Frame {
	// hardcoded 5x5
	kernel := [][]float64{
		[]float64{0.0038, 0.015, 0.0238, 0.015, 0.0038},
		[]float64{0.015, 0.0599, 0.0949, 0.0599, 0.015},
		[]float64{0.0238, 0.0949, 0.1503, 0.0949, 0.0238},
		[]float64{0.015, 0.0599, 0.0949, 0.0599, 0.0150},
		[]float64{0.0038, 0.015, 0.0238, 0.015, 0.0038},
	}

	return frm.Convolve(kernel)
}

func (frm *Frame) Sharpen() *Frame {
	kernel := [][]float64{
		[]float64{-0.0039, -0.0156, -0.0234, -0.0156, -0.0039},
		[]float64{-0.0156, -0.0625, -0.0938, -0.0625, -0.0156},
		[]float64{-0.0234, -0.0938, 1.8598, -0.0938, -0.0234},
		[]float64{-0.0156, -0.0625, -0.0938, -0.0625, -0.0156},
		[]float64{-0.0039, -0.0156, -0.0234, -0.0156, -0.0039},
	}
	return frm.Convolve(kernel)
}

func (frm *Frame) Downsample(factor int) *Frame {
	blurredFrm := frm.Blur()
	inputWidth := blurredFrm.GetWidth()
	inputHeight := blurredFrm.GetHeight()
	inputChannels := blurredFrm.GetChannels()
	outputWidth := inputWidth/factor
	outputHeight := inputHeight/factor
	outputFrm := makeFrame(outputWidth, outputHeight, inputChannels)
	
	
	for x := 0; x < outputWidth; x++ {
		inputX := x*factor
		for y := 0; y < outputHeight; y++ {
			inputY := y*factor
			original := blurredFrm.GetPixelAt(inputX, inputY)
			copied := make([]float64, inputChannels)
			copy(copied, original)
			outputFrm.SetPixelAt(x,y, copied)
		}
	}
	return outputFrm
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

	for x := 0; x < frameWidth; x++ {
		for y := 0; y < frameHeight; y++ {
			Ix2Sum := 0.0
			Iy2Sum := 0.0
			IxIySum := 0.0
			for wx := -windowWidth/2; wx <= windowWidth/2; wx++ {
				currentX := x+wx
				if currentX < 0 || currentX >= frameWidth {
					continue
				}
				for wy := -windowHeight/2; wy <= windowHeight/2; wy++ {
					currentY := y+wy
					if currentY < 0 || currentY >= frameHeight {
						continue
					}

					Ix2Sum += Ix2.GetPixelAt(currentX, currentY)[0]
					Iy2Sum += Iy2.GetPixelAt(currentX, currentY)[0]
					IxIySum += IxIy.GetPixelAt(currentX, currentY)[0]

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
			// if Ix2Value != 0 && Iy2Value != 0 {
			// 	fmt.Printf("x %v y %v Ix2 %v Iy2 %v Ixy %v\n",x,y, Ix2Value, Iy2Value, IxIyValue)
			// 	fmt.Printf("e0 %v e1 %v currentR %v\n", e0, e1, currentR)
			// }
			
			rFrame.SetPixelAt(x,y, currentR)
			
		}
		
	}

	// for wx := 0; wx < frameWidth/windowWidth; wx++ {
	// 	for wy := 0; wy < frameHeight/windowHeight; wy++ {
	// 		startX := wx*windowWidth
	// 		startY := wy*windowHeight
	// 		Ix2Sum := 0.0
	// 		Iy2Sum := 0.0
	// 		IxIySum := 0.0
	// 		for x := startX; x < startX + windowWidth; x++ {
	// 			for y := startY; y < startY + windowHeight; y++ {
	// 				Ix2Sum += Ix2.GetPixelAt(x, y)[0]
	// 				Iy2Sum += Iy2.GetPixelAt(x, y)[0]
	// 				IxIySum += IxIy.GetPixelAt(x, y)[0]
	// 			}
	// 		}
	// 		raw_matrix := []float64{Ix2Sum, IxIySum, IxIySum, Iy2Sum}
	// 		matrix := mat.NewDense(2, 2, raw_matrix)

	// 		var eig mat.Eigen
	// 		if ok := eig.Factorize(matrix, mat.EigenRight); !ok {
	// 			log.Fatal("Eigendecomposition failed to converge")
	// 		}
	// 		eigenvalues := eig.Values(nil)
	// 		// e1 is x changes (vertical edges), e0 is y changes (horizontal edges)
	// 		e0 := cmplx.Abs(eigenvalues[0])
	// 		e1 := cmplx.Abs(eigenvalues[1])
	// 		currentR := []float64{min(e0, e1)}
	// 		maxR = max(maxR, currentR[0])

	// 		for x := startX; x < startX + windowWidth; x++ {
	// 			for y := startY; y < startY + windowHeight; y++ {
	// 				rFrame.SetPixelAt(x,y, currentR)
	// 			}
	// 		}
	// 	}
	// }

	thresholdR := float64(maxR * 0.1)
	// fmt.Printf("max %v threshold %v\n",maxR, thresholdR)
	
	intermediateCoords := make([][3]float64, 0)

	for x := 0; x < frameWidth; x++{
		// fmt.Printf("x %v\n", x)
		for y := 0; y < frameHeight; y++{
			rValue := rFrame.GetPixelAt(x,y)[0]
			valid := true
			if rValue >= thresholdR {
				for wx := -windowWidth/2; wx <= windowWidth/2; wx++ {
					if !valid {
						break
					}
					currentX := x+wx
					if currentX < 0 || currentX >= frameWidth {
						continue
					}
					for wy := -windowHeight/2; wy <= windowHeight/2; wy++ {
						currentY := y+wy
						if currentY < 0 || currentY >= frameHeight {
							continue
						}
						currentValue := rFrame.GetPixelAt(currentX, currentY)[0]
						if rValue < currentValue {
							valid = false
							break
						}
					}

				}
				if valid {

					intermediateCoords = append(intermediateCoords, [3]float64{float64(x), float64(y), rValue})

				}
			}
		}
	}

	// for wx := 0; wx < frameWidth/windowWidth; wx++ {
	// 	for wy := 0; wy < frameHeight/windowHeight; wy++ {
	// 		midX := wx*windowWidth + windowWidth/2
	// 		midY := wy*windowHeight + windowHeight/2
	// 		rValue := rFrame.GetPixelAt(midX, midY)[0] 
	// 		if rValue >= thresholdR {
	// 			intermediateCoords = append(intermediateCoords, [3]float64{float64(midX), float64(midY), rValue})
	// 		}
	// 	}
	// }

	sort.Slice(intermediateCoords, func(i,j int) bool {
		return intermediateCoords[i][2] < intermediateCoords[j][2]
	})

	n := min(100, len(intermediateCoords))
	outputCoords := make([][2]float64, n)

	for i, c := range intermediateCoords[:n] {
		outputCoords[i] = [2]float64{c[0], c[1]}
	}
	// var outputCoords [][2]float64 = [][2]float64(intermediateCoords[:100][:2]) 
	// fmt.Printf("output %v\n", outputCoords)

	return outputCoords

}

func lucasKanade(oldFrm, newFrm *Frame, windowWidth, windowHeight int, points [][2]float64) ([][2]float64, []bool) {
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

	frameWidth := grayFrm.GetWidth()
	frameHeight := grayFrm.GetHeight()
	// frameChannels := grayFrm.GetChannels()


	Ix := grayOld.Gradient(false)
	Iy := grayOld.Gradient(true)
	
	// IxIt := makeFrame(frameWidth, frameHeight, frameChannels)
	// IyIt := makeFrame(frameWidth, frameHeight, frameChannels)
	// Ix := makeFrame(frameWidth, frameHeight, frameChannels)
	// Iy := makeFrame(frameWidth, frameHeight, frameChannels)
	// IxIy := makeFrame(frameWidth, frameHeight, frameChannels)
	// for x := range grayFrm.GetWidth() {
	// 	for y := range grayFrm.GetHeight() {
	// 		// oldValue := grayOld.GetPixelAt(x,y)[0]
	// 		// newValue := grayFrm.GetPixelAt(x,y)[0]
	// 		IxValue := Ix.GetPixelAt(x,y)[0]
	// 		IyValue := Iy.GetPixelAt(x,y)[0]
	// 		Ix.SetPixelAt(x,y,[]float64{IxValue})
	// 		Iy.SetPixelAt(x,y,[]float64{IyValue})
	// 		IxIy.SetPixelAt(x,y,[]float64{IxValue*IyValue})
	// 		// IxIt.SetPixelAt(x,y, []float64{(newValue-oldValue)*IxValue})
	// 		// IyIt.SetPixelAt(x,y, []float64{(newValue-oldValue)*IyValue})
	// 	}
	// }

	
	newPoints := make([][2]float64, len(points))
	copy(newPoints, points)

	invalid := make([]bool, len(points)) // all start off as false

	for index, point := range points {
		for i := 0; i < 20; i++{
		// newPoints := make([][2]float64, 0)
			if invalid[index] {
				break
			}
			x := point[0]
			y := point[1]
			newX := newPoints[index][0]
			newY := newPoints[index][1]

			sumIx2 := 0.0
			sumIy2 := 0.0
			sumIxIy := 0.0
			sumIxIt := 0.0
			sumIyIt := 0.0

			for wx := float64(-windowWidth/2); wx <= float64(windowWidth/2); wx++ {
				if x+wx < 0 || x+wx >= float64(frameWidth-1) ||
					 newX+wx < 0 || newX+wx >= float64(frameWidth-1){
					continue
				}
				for wy := float64(-windowHeight/2); wy <= float64(windowHeight/2); wy++ {
					if y+wy < 0 || y+wy >= float64(frameHeight-1) ||
					newY+wy < 0 || newY+wy >= float64(frameHeight-1) {
						continue
					}
					// fmt.Printf("x %v wx %v y %v wy %v\n", x, wx, y,wy)
					IxValue := Ix.InterpolatePixelAt(x+wx,y+wy)[0]
					IyValue := Iy.InterpolatePixelAt(x+wx,y+wy)[0]
					
					sumIx2 += IxValue*IxValue
					sumIy2 += IyValue*IyValue
					sumIxIy += IxValue*IyValue

					// not sure whether to use newX or X here
					oldValue := grayOld.InterpolatePixelAt(x+wx,y+wy)[0]
					newValue := grayFrm.InterpolatePixelAt(newX+wx,newY+wy)[0]

					sumIxIt += (newValue-oldValue)*IxValue
					sumIyIt += (newValue-oldValue)*IyValue
				}
			}

			A := mat.NewDense(2,2, []float64{sumIx2, sumIxIy, sumIxIy, sumIy2})
			
			var eig mat.Eigen
			if ok := eig.Factorize(A, mat.EigenRight); !ok {
				invalid[index] = true
				log.Fatal("Eigendecomposition failed to converge")
				continue
			}

			// fmt.Printf("x %v y %v: sumIx2 %v sumIy2 %v sumIxy %v sumIxt %v sumIyt %v\n", x, y, sumIx2, sumIy2, sumIxIy, sumIxIt, sumIyIt)
			// eigenvalues := eig.Values(nil)
			
			// // these are both real and positive
			// e0 := cmplx.Abs(eigenvalues[0])
			// e1 := cmplx.Abs(eigenvalues[1])
			// var eMax float64
			// var eMin float64
			// if e0 > e1 {
			// 	eMax = e0
			// 	eMin = e1
			// } else {
			// 	eMax = e1
			// 	eMin = e0
			// }
			// conditionNumber := eMax/eMin
			// if conditionNumber >= 25 {
			// 	fmt.Printf("Continuing due to condition number: %v\n", conditionNumber)
			// 	continue
			// }
			

			b := mat.NewVecDense(2, []float64{-sumIxIt, -sumIyIt})
			// fmt.Printf("A %v b %v\n", A, b)
			var uv mat.VecDense
			err := uv.SolveVec(A, b)
			if err != nil {
				fmt.Printf("Error solving system: %v\n", err)
				invalid[index] = true
				continue
			}
			xChange := uv.RawVector().Data[0]
			yChange := uv.RawVector().Data[1]
			// fmt.Printf("i %v index %v xChange %v yChange %v\n",i, index, xChange, yChange)

			updatedX := newPoints[index][0] + xChange
			updatedY := newPoints[index][1] + yChange

			// fmt.Printf("uvRaw %v xChange %v yChange %v\n",uv.RawVector(), xChange, yChange)

			// newX := x + xChange
			// newY := y + yChange

			if updatedX < 0 || updatedX > float64(newFrm.GetWidth()-1) || updatedY < 0 || updatedY > float64(newFrm.GetHeight()-1) {
				// invalid[index] = true don't know for sure
				continue
			}
			newPoints[index][0] = updatedX
			newPoints[index][1] = updatedY
			if math.Abs(xChange) < 0.05 && math.Abs(yChange) < 0.05 {
				fmt.Printf("Breaking early i %v x %v y %v\n",i, xChange, yChange)
				break
			}

			// outputPoints[index][0] = newX
			// outputPoints[index][1] = newY
			// newPoints = append(newPoints, [2]float64{newX, newY})
		}
		// currentPoints = newPoints

	}

	// fmt.Printf("old points %v\nnew points %v\ninvalid %v", points, newPoints, invalid)
	

	return newPoints, invalid
	
	
}

func pyramidalLucasKanade(oldFrm, newFrm *Frame, windowWidth, windowHeight int, points [][2]float64) [][2]float64 {
	// factors := []float64{2,4,8}
	base := 2.0
	pyramidLevels := 3.0
	for index := pyramidLevels; index >= 1; index-- {
	// for index := len(factors)-1; index >= 0; index-- {
		// factor := factors[index]
		factor := math.Pow(base, index)
		scaledDownPoints := make([][2]float64, len(points))
		for i, point := range points {
			scaledDownPoints[i] = [2]float64{point[0]/factor, point[1]/factor}
		}

		// scaledOldFrm := oldFrm
		// scaledNewFrm := newFrm
		// for i := 0; i < int(index); i++ {
		// 	scaledOldFrm = scaledOldFrm.Downsample(int(base))
		// 	scaledNewFrm = scaledNewFrm.Downsample(int(base))
		// }

		scaledOldFrm := oldFrm.Downsample(int(factor))
		scaledNewFrm := newFrm.Downsample(int(factor))

		newCorners, _ := lucasKanade(scaledOldFrm, scaledNewFrm, 10, 10, scaledDownPoints)
		filteredNewCorners := make([][2]float64, 0)
		for _, point := range newCorners {
			// if invalid[j] {
			// 	continue
			// }
			scaledUpPoint := [2]float64{point[0]*factor, point[1]*factor}
			filteredNewCorners = append(filteredNewCorners, scaledUpPoint)
		}
		points = filteredNewCorners
	}
	return points
}

func shiftImage(oldFrm *Frame, xChange, yChange float64) *Frame {
	outputFrm := makeFrame(oldFrm.GetWidth(), oldFrm.GetHeight(), oldFrm.GetChannels())
	for x := 0; x < outputFrm.GetWidth(); x++ {
		oldX := float64(x) - xChange
		if oldX < 0 || oldX >= float64(outputFrm.GetWidth()-1) {
			continue
		}

		for y := 0; y < outputFrm.GetHeight(); y++ {
			oldY := float64(y) - yChange
			if oldY < 0 || oldY >= float64(outputFrm.GetHeight()-1) {
				continue
			}
			// fmt.Printf("x %v y %v\n", oldX, oldY)
			outputFrm.SetPixelAt(x,y, oldFrm.InterpolatePixelAt(oldX, oldY))
		}
	} 
	return outputFrm


}

func drawRect(frmWidth, frmHeight, width, height, startX, startY int) *Frame {
	frm := makeFrame(frmWidth, frmHeight, 1) // grayscale by default
	for x := startX; x < startX + width; x++ {
		for y := startY; y < startY + height; y++ {
			frm.SetPixelAt(x,y, []float64{0.5})
		}
	}
	return frm

}

func displayST(frm *Frame, coords [][2]float64, windowWidth, windowHeight, counter int, outputFolder string) {
	withCorners := makeFrame(frm.GetWidth(), frm.GetHeight(), frm.GetChannels())
	withCorners.FillFrame(frm.pixels)
	for _, coord := range coords {
		midX := int(math.Round(coord[0]))
		midY := int(math.Round(coord[1]))
		for wx := -windowWidth/2; wx <= windowWidth/2; wx++ {
			if midX + wx < 0 || midX + wx >= frm.GetWidth() {
				continue
			}
			for wy := -windowHeight/2; wy <= windowHeight/2; wy++ {
				if midY + wy < 0 || midY + wy >= frm.GetHeight() {
					continue
				}
				// fmt.Printf("midX+wx %v midY+wx %v\n", midX+wx, midY+wy)
				withCorners.SetPixelAt(midX+wx, midY+wy, []float64{1,1,1,1})
			}
		}
	}

	filename := fmt.Sprintf("./%s/f%03d.png",outputFolder, counter)


	grayFrame := withCorners.Grayscale()
	grayImg := getGrayscaleImg(grayFrame)
	// saveToPNG(grayImg, "./display.png")
	saveToPNG(grayImg, filename)

}


func main() {
	// Open the video file
	// video, err := vidio.NewVideo("media/paper2.mp4")
	video, err := vidio.NewVideo("media/Charger2.mp4")
	// video, err := vidio.NewVideo("media/kitchen.mp4")
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
	// txAverageSum := 0.0
	// tyAverageSum := 0.0

	frameWidth := 500
	frameHeight := 150
	rectWidth := 100
	rectHeight := 100
	startX := 5
	startY := 5

	allTestCorners := make([][][2]float64, 0)
	testTxAverage := 0.0
	testTyAverage := 0.0

	for x := startX; x < frameWidth-rectWidth-10; x++ {
		testCounter := x-startX
		frm := drawRect(frameWidth, frameHeight, rectWidth, rectHeight, x,startY)
		if x == startX {
			testCorners := shiTomasi(frm, 5, 5)
			allTestCorners = append(allTestCorners, testCorners)
			displayST(frm, testCorners, 5, 5, testCounter, "draw")
		} else {
			// testCorners, _ := lucasKanade(oldFrm, frm, 10,10, allTestCorners[len(allTestCorners)-1])
			previousCorners := allTestCorners[len(allTestCorners)-1]
			testCorners := pyramidalLucasKanade(oldFrm, frm, 15,15, previousCorners)
			allTestCorners = append(allTestCorners, testCorners)

			currentTxAverage := 0.0
			currentTyAverage := 0.0
			for index, corner := range testCorners {
				currentTxAverage += corner[0]-previousCorners[index][0]
				currentTyAverage += corner[1]-previousCorners[index][1]
			}
			currentTxAverage /= float64(len(testCorners))
			currentTyAverage /= float64(len(testCorners))

			testTxAverage += currentTxAverage
			testTyAverage += currentTyAverage

			shiftedFrm := shiftImage(frm, -testTxAverage, -testTyAverage)

			displayST(shiftedFrm, testCorners, 5, 5, testCounter, "draw")
			// displayST(frm, testCorners, 5, 5, testCounter, "draw")
		}
		oldFrm = frm
	}
	// saveToPNG(img, "draw.png")

	// displayST(frame, corners, 5,5)
	// displayST(frm, [][2]float64{}, 0, 0, 0, "draw/")
	return

	for video.Read() {
		// FrameBuffer returns a byte array of the frame in row-major order (RGBA format)
		
		
		rawFrame := video.FrameBuffer()
		rawFrameFloats := make([]float64, len(rawFrame))
		for i := range rawFrameFloats {
			rawFrameFloats[i] = float64(rawFrame[i]) / 255.0
		}
		frame := makeFrame(frame_width, frame_height, frame_channels)
		frame.FillFrame(rawFrameFloats)

		// down := frame.Downsample(2)
		// blurredFrame := frame.Blur()
		// sharpenedFrame := blurredFrame.Sharpen()
		// displayST(blurredFrame, [][2]float64{}, 5, 5, 0, "temp")
		// displayST(frame, [][2]float64{}, 5, 5, 1, "temp")
		// displayST(sharpenedFrame, [][2]float64{}, 5, 5, 2, "temp")
		// displayST(down, [][2]float64{}, 5, 5, 2, "temp")
		// break

		// if frame 0, get corners using shiTomasi; save f0 as "oldFrame" and continue
		// otherwise, use shiTomasi to get corners, do thisFrame-oldFrame for I_t
		// still using the Ix and Iy of the previous frame though
		fmt.Printf("Counter %v\n", counter)
		if counter == 0 {
			corners := shiTomasi(frame, 5, 5)
			allCorners = append(allCorners, corners)
			// displayST(frame, corners, 5,5)
			displayST(frame, corners, 5, 5, counter, "chargerOutputFrames")

		} else {
			// fmt.Printf("calling lk with corners %v\n", allCorners[len(allCorners)-1])
			recentPoints := allCorners[len(allCorners)-1]
			recentPoints = pyramidalLucasKanade(oldFrm, frame, 5, 5, recentPoints)
			// factors := []float64{2,4,8}
			// for index := len(factors)-1; index >= 0; index-- {
			// 	factor := factors[index]
			// 	scaledDownPoints := make([][2]float64, len(recentPoints))
			// 	for i, point := range recentPoints {
			// 		scaledDownPoints[i] = [2]float64{point[0]/factor, point[1]/factor}
			// 	}
			// 	scaledOldFrm := oldFrm.Downsample(int(factor))
			// 	scaledNewFrm := frame.Downsample(int(factor))
			// 	newCorners, invalid := lucasKanade(scaledOldFrm, scaledNewFrm, 5, 5, scaledDownPoints)
			// 	filteredNewCorners := make([][2]float64, 0)
			// 	for j, point := range newCorners {
			// 		if invalid[j] {
			// 			continue
			// 		}
			// 		scaledUpPoint := [2]float64{point[0]*factor, point[1]*factor}
			// 		filteredNewCorners = append(filteredNewCorners, scaledUpPoint)
			// 	}
			// 	recentPoints = filteredNewCorners
			// }
			
			// corners, _ := lucasKanade(oldFrm, frame, 5, 5, allCorners[len(allCorners)-1])
			allCorners = append(allCorners, recentPoints)

			// txSum := 0.0
			// tySum := 0.0
			// for index := range corners {
			// 	txSum += (corners[index][0]-allCorners[0][index][0])
			// 	tySum += (corners[index][1]-allCorners[0][index][1])
			// }
			// txAverage := txSum/float64(len(corners))
			// tyAverage := tySum/float64(len(corners))
			
			// txAverageSum += txAverage
			// tyAverageSum += tyAverage

			// shiftedFrame := shiftImage(frame, txAverageSum, tyAverageSum)
			// fmt.Printf("new corners %v\n", corners)
			displayST(frame, recentPoints, 5, 5, counter, "chargerOutputFrames")
			// displayST(shiftedFrame, corners, 5, 5, counter, "chargerOutputFrames")
			// break

		}
		oldFrm = frame
		counter++


		


		// 
	}
}