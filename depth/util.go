package depth

import (
	"image"
	"math"

	"github.com/nfnt/resize"
	"golang.org/x/image/draw"
)

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

// resizeWithinMax 缩放（最长边 <= maxSize）
func resizeWithinMax(img *image.NRGBA, maxSize int) *image.NRGBA {
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
