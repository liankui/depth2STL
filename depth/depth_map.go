package depth

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/draw"
)

// PNG → *image.NRGBA
//
// JPEG → *image.YCbCr
//
// 灰度图 → *image.Gray

// ConvertToGray 直接把图像转换为灰度图
func ConvertToGray(img image.Image) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.Set(x, y, color.GrayModel.Convert(img.At(x, y)))
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

func GenerateDepthMap3(img image.Image, detailLevel float64, invert bool) *image.Gray {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	// ---------- 缩放 ----------
	base := math.Max(1, 320.0*detailLevel)
	ratio := math.Min(base/float64(w), base/float64(h))
	nw, nh := max(1, int(float64(w)*ratio)), max(1, int(float64(h)*ratio))

	// ---------- 灰度 ----------
	gray := image.NewGray(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(x+b.Min.X, y+b.Min.Y).RGBA()
			v := uint8((299*r + 587*g + 114*b) / 1000 >> 8)
			gray.Pix[y*gray.Stride+x] = v
		}
	}

	// ---------- 缩放 ----------
	resized := image.NewGray(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(resized, resized.Bounds(), gray, gray.Bounds(), draw.Over, nil)

	// ---------- 轻模糊 ----------
	blur := image.NewGray(resized.Bounds())
	for y := 1; y < nh-1; y++ {
		for x := 1; x < nw-1; x++ {
			sum := int(resized.GrayAt(x, y).Y)*4 +
				int(resized.GrayAt(x-1, y).Y) +
				int(resized.GrayAt(x+1, y).Y) +
				int(resized.GrayAt(x, y-1).Y) +
				int(resized.GrayAt(x, y+1).Y)
			blur.SetGray(x, y, color.Gray{Y: uint8(sum / 8)})
		}
	}
	resized = blur

	// =========================================================
	// 🔥 关键：百分位对比拉伸（忽略黑背景）
	// =========================================================

	hist := make([]int, 256)
	for _, v := range resized.Pix {
		hist[v]++
	}

	total := len(resized.Pix)
	lowCut := total * 2 / 100   // 2%
	highCut := total * 98 / 100 // 98%

	sum := 0
	minVal, maxVal := 0, 255

	for i := 0; i < 256; i++ {
		sum += hist[i]
		if sum >= lowCut {
			minVal = i
			break
		}
	}

	sum = 0
	for i := 255; i >= 0; i-- {
		sum += hist[i]
		if sum >= (total - highCut) {
			maxVal = i
			break
		}
	}

	if maxVal <= minVal {
		maxVal = minVal + 1
	}

	scale := 255.0 / float64(maxVal-minVal)

	// ---------- 拉伸 + gamma ----------
	out := image.NewGray(resized.Bounds())

	gamma := 0.7 // 🔥 提亮暗部（关键）

	for i, v := range resized.Pix {
		nv := float64(v-uint8(minVal)) * scale
		if nv < 0 {
			nv = 0
		}
		if nv > 255 {
			nv = 255
		}

		// gamma
		nv = math.Pow(nv/255.0, gamma) * 255

		val := uint8(nv + 0.5)

		if invert {
			val = 255 - val
		}

		out.Pix[i] = val
	}

	// ---------- Z量化 ----------
	step := uint8(256 / levels)
	for i, v := range out.Pix {
		out.Pix[i] = (v / step) * step
	}

	return out
}

func GenerateDepthMap4(img image.Image, invert bool) *image.Gray {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	// ---------- 缩放 ----------
	baseSize := math.Max(1, 320.0)
	ratio := math.Min(baseSize/float64(w), baseSize/float64(h))
	nw, nh := max(1, int(float64(w)*ratio)), max(1, int(float64(h)*ratio))

	// ---------- 灰度 ----------
	gray := image.NewGray(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(x+b.Min.X, y+b.Min.Y).RGBA()
			v := uint8((299*r + 587*g + 114*b) / 1000 >> 8)

			// 🔥 背景抑制（关键）
			if v < 8 {
				v = 0
			}

			gray.Pix[y*gray.Stride+x] = v
		}
	}

	// ---------- 缩放 ----------
	resized := image.NewGray(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(resized, resized.Bounds(), gray, gray.Bounds(), draw.Over, nil)

	// =========================================================
	// 1️⃣ 低频（Base）= 模糊
	// =========================================================
	base := image.NewGray(resized.Bounds())
	for y := 1; y < nh-1; y++ {
		for x := 1; x < nw-1; x++ {
			sum := 0
			for ky := -1; ky <= 1; ky++ {
				for kx := -1; kx <= 1; kx++ {
					sum += int(resized.GrayAt(x+kx, y+ky).Y)
				}
			}
			base.SetGray(x, y, color.Gray{Y: uint8(sum / 9)})
		}
	}

	// =========================================================
	// 2️⃣ 高频（Detail）= Laplacian
	// =========================================================
	detail := image.NewGray(resized.Bounds())
	for y := 1; y < nh-1; y++ {
		for x := 1; x < nw-1; x++ {
			c := int(resized.GrayAt(x, y).Y) * 4
			sum := c -
				int(resized.GrayAt(x-1, y).Y) -
				int(resized.GrayAt(x+1, y).Y) -
				int(resized.GrayAt(x, y-1).Y) -
				int(resized.GrayAt(x, y+1).Y)

			sum = sum/2 + 128 // 中心化

			if sum < 0 {
				sum = 0
			}
			if sum > 255 {
				sum = 255
			}

			detail.SetGray(x, y, color.Gray{Y: uint8(sum)})
		}
	}

	// =========================================================
	// 3️⃣ 百分位拉伸（对 base 做）
	// =========================================================
	hist := make([]int, 256)
	for _, v := range base.Pix {
		hist[v]++
	}

	total := len(base.Pix)
	lowCut := total * 2 / 100
	highCut := total * 98 / 100

	sum := 0
	minVal, maxVal := 0, 255

	for i := 0; i < 256; i++ {
		sum += hist[i]
		if sum >= lowCut {
			minVal = i
			break
		}
	}

	sum = 0
	for i := 255; i >= 0; i-- {
		sum += hist[i]
		if sum >= (total - highCut) {
			maxVal = i
			break
		}
	}

	if maxVal <= minVal {
		maxVal = minVal + 1
	}

	scale := 255.0 / float64(maxVal-minVal)

	// =========================================================
	// 4️⃣ 融合 Base + Detail
	// =========================================================
	out := image.NewGray(base.Bounds())

	detailStrength := 0.6 // 🔥 控制雕刻强度
	gamma := 0.7          // 🔥 提亮暗部

	for i := range base.Pix {
		// base 拉伸
		bv := float64(base.Pix[i]-uint8(minVal)) * scale
		if bv < 0 {
			bv = 0
		}
		if bv > 255 {
			bv = 255
		}

		// detail [-128,128]
		dv := float64(int(detail.Pix[i]) - 128)

		// 融合
		v := bv + dv*detailStrength

		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}

		// gamma
		v = math.Pow(v/255.0, gamma) * 255

		val := uint8(v + 0.5)

		if invert {
			val = 255 - val
		}

		out.Pix[i] = val
	}

	// =========================================================
	// 5️⃣ Z量化
	// =========================================================
	step := uint8(256 / levels)
	for i, v := range out.Pix {
		out.Pix[i] = (v / step) * step
	}

	return out
}
