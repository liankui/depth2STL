package depth

import (
	"errors"
	"image"
	"image/draw"
	"math"

	"github.com/nfnt/resize"
)

type Preprocessor struct {
	RemBG BackgroundRemover
}

func NewPreprocessor() *Preprocessor {
	return &Preprocessor{
		RemBG: NewDefaultRemBG(),
	}
}

// ImagePreprocess 把任意输入图片变成
//
//	尺寸 ≤ 1024
//	只有主体（背景被移除或用 alpha）
//	主体被裁成正方形并居中
//	输出为 RGB、黑底、已乘 alpha（premultiplied alpha）
func (p *Preprocessor) ImagePreprocess(input image.Image) (image.Image, error) {
	// 转为 NRGBA，方便统一处理
	src := toNRGBA(input)

	// 1. 判断是否已有有效 Alpha
	hasAlpha := hasUsefulAlpha(src)

	// 2. 缩放（最长边 <= 1024）
	src = resizeIfNeeded(src, 1024)

	var output *image.NRGBA
	var err error

	// 3. 背景去除
	if hasAlpha {
		output = src
	} else {
		bgRemoved, err := p.RemBG.Remove(src)
		if err != nil {
			return nil, err
		}

		output = toNRGBA(bgRemoved)
	}

	// 4. Alpha Bounding Box
	bbox, err := alphaBBox(output, 0.8)
	if err != nil {
		return nil, err
	}

	// 5. 正方形中心裁剪
	output = cropSquare(output, bbox)

	// 6. 预乘 Alpha
	premultiply(output)

	return output, nil
}

// hasUsefulAlpha 检查 alpha 通道是否 真的包含透明信息
// 只要存在非 255（非完全不透明），就认为“已有抠图”
func hasUsefulAlpha(img *image.NRGBA) bool {
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 255 {
			return true
		}
	}
	return false
}

func resizeIfNeeded(img *image.NRGBA, maxSize int) *image.NRGBA {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	longest := max(w, h)

	if longest <= maxSize {
		return img
	}

	scale := float64(maxSize) / float64(longest)
	newW := int(float64(w) * scale)
	newH := int(float64(h) * scale)

	resized := resize.Resize(uint(newW), uint(newH), img, resize.Lanczos3)
	return toNRGBA(resized)
}

// alphaBBox 从 alpha 通道计算主体 bounding box
// 把 alpha > threshold * 255 的像素当作“主体”，找所有主体像素的坐标
func alphaBBox(img *image.NRGBA, threshold float64) (image.Rectangle, error) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	th := uint8(threshold * 255)

	minX, minY := w, h
	maxX, maxY := 0, 0
	found := false

	for y := 0; y < h; y++ {
		row := y * img.Stride
		for x := 0; x < w; x++ {
			a := img.Pix[row+x*4+3]
			if a > th {
				found = true
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	if !found {
		return image.Rectangle{}, errors.New("未检测到前景区域")
	}

	return image.Rect(minX, minY, maxX+1, maxY+1), nil
}

// cropSquare 正方形裁剪（中心对齐）
// 计算主体中心点
// 用最长边作为正方形边长
// 保证输出是 正方形
func cropSquare(img *image.NRGBA, bbox image.Rectangle) *image.NRGBA {
	cx := (bbox.Min.X + bbox.Max.X) / 2
	cy := (bbox.Min.Y + bbox.Max.Y) / 2
	size := int(math.Max(float64(bbox.Dx()), float64(bbox.Dy())))

	half := size / 2
	rect := image.Rect(
		cx-half, cy-half,
		cx+half, cy+half,
	).Intersect(img.Bounds())

	dst := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(dst, dst.Bounds(), img, rect.Min, draw.Src)
	return dst
}

// premultiply 预乘 Alpha，RGB × alpha，得到 premultiplied alpha
// 例如：红色半透明 (1,0,0,0.5) → (0.5,0,0)，背景自然变黑
// 目的：去除白边 / 透明边缘污染，保证 encoder 看到的是“干净物体”
func premultiply(img *image.NRGBA) {
	for i := 0; i < len(img.Pix); i += 4 {
		a := float64(img.Pix[i+3]) / 255.0
		img.Pix[i] = uint8(float64(img.Pix[i]) * a)
		img.Pix[i+1] = uint8(float64(img.Pix[i+1]) * a)
		img.Pix[i+2] = uint8(float64(img.Pix[i+2]) * a)
	}
}

func toNRGBA(img image.Image) *image.NRGBA {
	if nrgba, ok := img.(*image.NRGBA); ok {
		return nrgba
	}
	b := img.Bounds()
	dst := image.NewNRGBA(b)
	draw.Draw(dst, b, img, b.Min, draw.Src)
	return dst
}
