package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
)

func main() {
	inputPath := "input/Dota_2_Monster_Hunter_codex_centaur_warrunner_gameasset.png" // 可替换为命令行参数
	outputDir := "./output"
	os.MkdirAll(outputDir, os.ModePerm)

	var img image.Image
	var err error
	if strings.HasPrefix(inputPath, "http://") || strings.HasPrefix(inputPath, "https://") {
		img, err = downloadImage(inputPath)
	} else {
		img, err = openImage(inputPath)
	}
	if err != nil {
		log.Fatal("Failed to load image:", err)
	}

	depthMap := generateDepthMap(img, 1.0, false)

	depthPath := filepath.Join(outputDir, "depth_map.png")
	depthFile, _ := os.Create(depthPath)
	err = png.Encode(depthFile, depthMap)
	if err != nil {
		log.Fatal("Failed to encode image:", err)
	}
	depthFile.Close()

	stlPath := filepath.Join(outputDir, "model.stl")
	err = generateSTL(depthMap, stlPath, 50.0, 5.0, 2.0)
	if err != nil {
		log.Fatal("Failed to generate STL:", err)
	}

	log.Println("Done! Depth map:", depthPath, "STL:", stlPath)
}

// Helper: 下载图片
func downloadImage(url string) (image.Image, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
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
	defer file.Close()
	img, _, err := image.Decode(file)
	return img, err
}

// 生成深度图（灰度图 + 高斯模糊）
func generateDepthMap(img image.Image, detailLevel float64, invert bool) *image.Gray {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	baseSize := int(320 * detailLevel)
	ratio := math.Min(float64(baseSize)/float64(width), float64(baseSize)/float64(height))
	newWidth := int(float64(width) * ratio)
	newHeight := int(float64(height) * ratio)

	// 调整尺寸
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	// 转灰度
	gray := image.NewGray(dst.Bounds())
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			r, g, b, _ := dst.At(x, y).RGBA()
			// 转换为 0-255
			grayVal := uint8(math.Pow((0.299*float64(r)+0.587*float64(g)+0.114*float64(b))/65535.0, 1.5) * 255)
			if invert {
				grayVal = 255 - grayVal
			}
			gray.SetGray(x, y, color.Gray{Y: grayVal})
		}
	}

	// 高斯模糊（简单均值模糊代替高斯）
	blurred := image.NewGray(gray.Bounds())
	kernel := []int{-1, 0, 1}
	for y := 1; y < newHeight-1; y++ {
		for x := 1; x < newWidth-1; x++ {
			var sum int
			for _, ky := range kernel {
				for _, kx := range kernel {
					sum += int(gray.GrayAt(x+kx, y+ky).Y)
				}
			}
			blurred.SetGray(x, y, color.Gray{Y: uint8(sum / 9)})
		}
	}
	return blurred
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
	defer f.Close()

	fmt.Fprintln(f, "solid relief_model")

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

	fmt.Fprintln(f, "endsolid relief_model")
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
	fmt.Fprintf(f, "  facet normal %f %f %f\n", normal[0], normal[1], normal[2])
	fmt.Fprintln(f, "    outer loop")
	fmt.Fprintf(f, "      vertex %f %f %f\n", v1[0], v1[1], v1[2])
	fmt.Fprintf(f, "      vertex %f %f %f\n", v2[0], v2[1], v2[2])
	fmt.Fprintf(f, "      vertex %f %f %f\n", v3[0], v3[1], v3[2])
	fmt.Fprintln(f, "    endloop")
	fmt.Fprintln(f, "  endfacet")
}
