package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/segmentio/ksuid"
	"golang.org/x/image/draw"
)

/*
- `image_path`：要转换的图像本地路径或 URL
- `model_width`：3D 模型的宽度（毫米，默认：50.0）
- `model_thickness`：3D 模型的最大厚度/高度（毫米，默认：5.0）
- `base_thickness`：底座厚度（毫米，默认：2.0）
- `skip_depth`：是否直接使用图像或生成深度图（默认：true）
- `invert_depth`：是否反转浮雕（明亮区域变低而不是高，默认：false）
- `detail_level`：控制处理图像的分辨率（默认：1.0）。当 detail_level = 1.0 时，图像以 320px 分辨率处理，生成的 STL 文件通常在 100MB 以内。较高的值可以提高细节质量，但会显著增加处理时间和 STL 文件大小。例如，将 detail_level 值加倍可能会使文件大小增加 4 倍或更多，请谨慎使用。
*/
var (
	imagePath      = flag.String("imagePath", "", "local path or web URL to the input image file")
	modelWidth     = flag.Float64("modelWidth", 50.0, "width of the 3D model in mm (default: 50.0)")
	modelThickness = flag.Float64("modelThickness", 5.0, "maximum thickness/height of the 3D model in mm (default: 5.0)")
	baseThickness  = flag.Float64("baseThickness", 2.0, "tickness of the base in mm (default: 2.0)")
	skipDepth      = flag.Bool("skipDepth", true, "whether to use the image directly or generate a depth map (default: true)")
	invertDepth    = flag.Bool("invertDepth", false, "invert the relief (bright areas become low instead of high) (default: false)")
	detailLevel    = flag.Float64("detailLevel", 1.0, "level of detail level (default: 1.0)")
)

func main() {
	flag.Parse()

	inputPath := *imagePath
	outputDir := "./output"
	_ = os.MkdirAll(outputDir, os.ModePerm)

	var img image.Image
	var err error
	if strings.HasPrefix(inputPath, "http://") || strings.HasPrefix(inputPath, "https://") {
		img, err = downloadImage(inputPath)
	} else {
		img, err = openImage(inputPath)
	}
	if err != nil {
		slog.Error("failed to load image", "error", err)
		return
	}

	var depthMap *image.Gray
	if *skipDepth { // 直接使用灰度图
		depthMap = convertToGray(img)
	} else { // 调用统一的深度图生成函数
		depthMap = generateDepthMap(img, *detailLevel, *invertDepth)
	}

	uid := ksuid.New().String()
	depthPath := filepath.Join(outputDir, uid+"_depth_map.png")
	depthFile, err := os.Create(depthPath)
	if err != nil {
		slog.Error("failed to open depth_map.png", "error", err)
		return
	}
	defer func() {
		_ = depthFile.Close()
	}()

	err = png.Encode(depthFile, depthMap)
	if err != nil {
		slog.Error("failed to encode depth map", "error", err)
		return
	}

	stlPath := filepath.Join(outputDir, uid+".stl")
	err = generateSTL(depthMap, stlPath, *modelWidth, *modelThickness, *baseThickness)
	if err != nil {
		slog.Error("failed to generate stl", "error", err)
		return
	}

	slog.Info("generated stl", "depth path", depthPath, "stl path", stlPath)
}

// Helper: 下载图片
func downloadImage(url string) (image.Image, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	imgData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(imgData))
	return img, err
}

// Helper: 打开本地图片
func openImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	img, _, err := image.Decode(file)
	return img, err
}

// convertToGray 直接把图像转换为灰度图（没有缩放和模糊）
func convertToGray(img image.Image) *image.Gray {
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

// generateDepthMap 生成深度图：灰度 + 缩放 + 高斯模糊 + 可反转
func generateDepthMap(img image.Image, detailLevel float64, invert bool) *image.Gray {
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

func generateSTL(depthMap *image.Gray, outputPath string, modelWidth, modelThickness, baseThickness float64) error {
	height := depthMap.Bounds().Dy()
	width := depthMap.Bounds().Dx()
	pixelSize := modelWidth / float64(width)

	// 打开输出文件
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	_, _ = fmt.Fprintln(f, "solid relief_model")

	// 构建顶点高度
	vertices := make([][]float64, height)
	for y := 0; y < height; y++ {
		vertices[y] = make([]float64, width)
		for x := 0; x < width; x++ {
			vertices[y][x] = float64(depthMap.GrayAt(x, y).Y) / 255.0 * modelThickness
		}
	}

	// 顶面三角形
	for y := 0; y < height-1; y++ {
		for x := 0; x < width-1; x++ {
			y0 := float64(height-y-1) * pixelSize
			y1 := float64(height-(y+1)-1) * pixelSize
			x0 := float64(x) * pixelSize
			x1 := float64(x+1) * pixelSize

			z00 := vertices[y][x]
			z01 := vertices[y+1][x]
			z10 := vertices[y][x+1]
			z11 := vertices[y+1][x+1]

			writeFacet(f, [3]float64{x0, y0, z00}, [3]float64{x1, y0, z10}, [3]float64{x0, y1, z01})
			writeFacet(f, [3]float64{x1, y0, z10}, [3]float64{x1, y1, z11}, [3]float64{x0, y1, z01})
		}
	}

	// 底面 (Z = -baseThickness)
	for y := 0; y < height-1; y++ {
		for x := 0; x < width-1; x++ {
			y0 := float64(height-y-1) * pixelSize
			y1 := float64(height-(y+1)-1) * pixelSize
			x0 := float64(x) * pixelSize
			x1 := float64(x+1) * pixelSize

			writeFacet(f, [3]float64{x0, y0, -baseThickness}, [3]float64{x1, y1, -baseThickness}, [3]float64{x0, y1, -baseThickness})
			writeFacet(f, [3]float64{x0, y0, -baseThickness}, [3]float64{x1, y0, -baseThickness}, [3]float64{x1, y1, -baseThickness})
		}
	}

	// 前后边缘
	for x := 0; x < width-1; x++ {
		x0 := float64(x) * pixelSize
		x1 := float64(x+1) * pixelSize
		// 前边 (y=0)
		y0 := 0.0
		z0 := -baseThickness
		z1 := vertices[height-1][x]
		z2 := vertices[height-1][x+1]

		writeFacet(f, [3]float64{x0, y0, z0}, [3]float64{x1, y0, z0}, [3]float64{x0, y0, z1})
		writeFacet(f, [3]float64{x1, y0, z0}, [3]float64{x1, y0, z2}, [3]float64{x0, y0, z1})

		// 后边 (y = height-1)
		y0 = float64(height-1) * pixelSize
		z0 = -baseThickness
		z1 = vertices[0][x]
		z2 = vertices[0][x+1]

		writeFacet(f, [3]float64{x0, y0, z0}, [3]float64{x0, y0, z1}, [3]float64{x1, y0, z0})
		writeFacet(f, [3]float64{x1, y0, z0}, [3]float64{x1, y0, z2}, [3]float64{x0, y0, z1})
	}

	// 左右边缘
	for y := 0; y < height-1; y++ {
		y0 := float64(height-y-1) * pixelSize
		y1 := float64(height-(y+1)-1) * pixelSize
		z0 := -baseThickness
		// 左边 (x=0)
		x0 := 0.0
		z1 := vertices[y][0]
		z2 := vertices[y+1][0]

		writeFacet(f, [3]float64{x0, y0, z0}, [3]float64{x0, y0, z1}, [3]float64{x0, y1, z0})
		writeFacet(f, [3]float64{x0, y1, z0}, [3]float64{x0, y0, z1}, [3]float64{x0, y1, z2})

		// 右边 (x=width-1)
		x0 = float64(width-1) * pixelSize
		z1 = vertices[y][width-1]
		z2 = vertices[y+1][width-1]

		writeFacet(f, [3]float64{x0, y0, z0}, [3]float64{x0, y1, z0}, [3]float64{x0, y0, z1})
		writeFacet(f, [3]float64{x0, y1, z0}, [3]float64{x0, y1, z2}, [3]float64{x0, y0, z1})
	}

	_, _ = fmt.Fprintln(f, "endsolid relief_model")
	return nil
}

// 写入 STL 面
func writeFacet(f *os.File, v1, v2, v3 [3]float64) {
	a := [3]float64{v2[0] - v1[0], v2[1] - v1[1], v2[2] - v1[2]}
	b := [3]float64{v3[0] - v1[0], v3[1] - v1[1], v3[2] - v1[2]}
	normal := [3]float64{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
	norm := math.Sqrt(normal[0]*normal[0] + normal[1]*normal[1] + normal[2]*normal[2])
	if norm > 0 {
		for i := 0; i < 3; i++ {
			normal[i] /= norm
		}
	}
	_, _ = fmt.Fprintf(f, "  facet normal %f %f %f\n", normal[0], normal[1], normal[2])
	_, _ = fmt.Fprintf(f, "    outer loop\n")
	_, _ = fmt.Fprintf(f, "      vertex %f %f %f\n", v1[0], v1[1], v1[2])
	_, _ = fmt.Fprintf(f, "      vertex %f %f %f\n", v2[0], v2[1], v2[2])
	_, _ = fmt.Fprintf(f, "      vertex %f %f %f\n", v3[0], v3[1], v3[2])
	_, _ = fmt.Fprintf(f, "    endloop\n")
	_, _ = fmt.Fprintf(f, "  endfacet\n")
}
