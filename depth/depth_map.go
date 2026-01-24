package depth

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/draw"
)

// ConvertToGray 直接把图像转换为灰度图（没有缩放和模糊）
func ConvertToGray(img image.Image) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			val := uint8((0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256)
			gray.SetGray(x, y, color.Gray{Y: val})
		}
	}
	return gray
}

// GenerateDepthMap 生成深度图：灰度 + 缩放 + 高斯模糊 + 可反转
func GenerateDepthMap(img image.Image, detailLevel float64, invert bool) *image.Gray {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 缩放尺寸
	baseSize := 320.0 * detailLevel
	ratio := math.Min(baseSize/float64(width), baseSize/float64(height))
	newWidth := int(float64(width) * ratio)
	newHeight := int(float64(height) * ratio)

	// 灰度化 + gamma 校正
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			val := math.Pow((0.299*float64(r)+0.587*float64(g)+0.114*float64(b))/65535.0, 1.5) * 255
			gray.SetGray(x, y, color.Gray{Y: uint8(val)})
		}
	}

	// 缩放
	resized := image.NewGray(image.Rect(0, 0, newWidth, newHeight))
	draw.CatmullRom.Scale(resized, resized.Bounds(), gray, gray.Bounds(), draw.Over, nil)

	// 高斯模糊 (3x3 高斯卷积)
	kernel := [3][3]float64{
		{1 / 16.0, 2 / 16.0, 1 / 16.0},
		{2 / 16.0, 4 / 16.0, 2 / 16.0},
		{1 / 16.0, 2 / 16.0, 1 / 16.0},
	}
	blur := image.NewGray(resized.Bounds())
	for y := 1; y < newHeight-1; y++ {
		for x := 1; x < newWidth-1; x++ {
			var sum float64
			for ky := -1; ky <= 1; ky++ {
				for kx := -1; kx <= 1; kx++ {
					sum += float64(resized.GrayAt(x+kx, y+ky).Y) * kernel[ky+1][kx+1]
				}
			}
			val := uint8(sum)
			if invert {
				val = 255 - val
			}
			blur.SetGray(x, y, color.Gray{Y: val})
		}
	}

	return blur
}

func GenerateDepthMap2(img image.Image, detailLevel float64, invert bool) *image.Gray {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	// 防止 0 尺寸
	base := math.Max(1, 320.0*detailLevel)
	ratio := math.Min(base/float64(w), base/float64(h))
	nw, nh := max(1, int(float64(w)*ratio)), max(1, int(float64(h)*ratio))

	// Gamma LUT
	var gammaLUT [256]uint8
	for i := 0; i < 256; i++ {
		gammaLUT[i] = uint8(math.Pow(float64(i)/255.0, 1.5)*255.0 + 0.5)
	}

	// 灰度化
	gray := image.NewGray(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(x+b.Min.X, y+b.Min.Y).RGBA()
			y8 := uint8((299*r + 587*g + 114*b + 500) / 1000 >> 8)
			gray.Pix[y*gray.Stride+x] = gammaLUT[y8]
		}
	}

	// 缩放
	resized := image.NewGray(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(resized, resized.Bounds(), gray, gray.Bounds(), draw.Over, nil)

	// 高斯模糊（整数）
	out := image.NewGray(resized.Bounds())
	k := [9]int{1, 2, 1, 2, 4, 2, 1, 2, 1}

	for y := 1; y < nh-1; y++ {
		for x := 1; x < nw-1; x++ {
			sum := 0
			idx := 0
			for ky := -1; ky <= 1; ky++ {
				row := (y+ky)*resized.Stride + (x - 1)
				for kx := 0; kx < 3; kx++ {
					sum += int(resized.Pix[row+kx]) * k[idx]
					idx++
				}
			}
			v := uint8(sum >> 4)
			if invert {
				v = 255 - v
			}
			out.Pix[y*out.Stride+x] = v
		}
	}

	return out
}
