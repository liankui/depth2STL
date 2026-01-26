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

const (
	// base   = 320.0 // XY 分辨率（影响 STL 面数）
	base   = 320.0 // XY 分辨率（影响 STL 面数）
	levels = 36    // Z 台阶数（影响 STL 高度层次）
)

func GenerateDepthMap2(img image.Image, detailLevel float64, invert bool) *image.Gray {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	// ---------- XY 分辨率：温和降级 ----------
	base := math.Max(1, base*detailLevel)
	ratio := math.Min(base/float64(w), base/float64(h))
	nw, nh := max(1, int(float64(w)*ratio)), max(1, int(float64(h)*ratio))

	// ---------- 线性灰度 ----------
	gray := image.NewGray(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(x+b.Min.X, y+b.Min.Y).RGBA()
			gray.Pix[y*gray.Stride+x] = uint8((299*r + 587*g + 114*b) / 1000 >> 8)
		}
	}

	// ---------- 缩放 ----------
	resized := image.NewGray(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(resized, resized.Bounds(), gray, gray.Bounds(), draw.Over, nil)

	// ---------- 轻度高斯模糊（仅消噪） ----------
	blur := image.NewGray(resized.Bounds())
	k := [3][3]int{
		{1, 2, 1},
		{2, 4, 2},
		{1, 2, 1},
	}

	for y := 1; y < nh-1; y++ {
		for x := 1; x < nw-1; x++ {
			sum := 0
			for ky := -1; ky <= 1; ky++ {
				for kx := -1; kx <= 1; kx++ {
					sum += int(resized.Pix[(y+ky)*resized.Stride+x+kx]) * k[ky+1][kx+1]
				}
			}
			blur.Pix[y*blur.Stride+x] = uint8(sum >> 4)
		}
	}
	resized = blur

	// ---------- 轻 S 曲线（保形体） ----------
	var lut [256]uint8
	for i := 0; i < 256; i++ {
		x := float64(i) / 255.0
		y := x * x * (3 - 2*x) // smoothstep
		lut[i] = uint8(y*255 + 0.5)
	}

	// ---------- Z 量化（细节保留版） ----------
	step := uint8(256 / levels)

	out := image.NewGray(resized.Bounds())
	for i, v := range resized.Pix {
		v = lut[v]
		q := (v / step) * step
		if invert {
			q = 255 - q
		}
		out.Pix[i] = q
	}

	return out
}
