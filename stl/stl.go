package stl

import (
	"bufio"
	"fmt"
	"image"
	"io"
	"math"
	"os"
)

func GenerateSTL(depthMap *image.Gray, outputPath string, modelWidth, modelThickness, baseThickness float64) error {
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

func GenerateSTL2(depthMap *image.Gray, outputPath string, modelWidth, modelThickness, baseThickness float64) error {
	b := depthMap.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 2 || h < 2 {
		return fmt.Errorf("depth map too small")
	}

	pixel := modelWidth / float64(w)

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	bw := bufio.NewWriter(f)
	defer func() {
		_ = bw.Flush()
	}()

	_, _ = fmt.Fprintln(bw, "solid relief_model")

	// 预计算 Y（翻转坐标系，避免循环内重复计算）
	yPos := make([]float64, h)
	for y := 0; y < h; y++ {
		yPos[y] = float64(h-y-1) * pixel
	}

	// 灰度 → Z 高度（即时计算，避免 vertices[][] 占用内存）
	zAt := func(x, y int) float64 {
		return float64(depthMap.Pix[y*depthMap.Stride+x]) / 255.0 * modelThickness
	}

	// ---------------- 顶面 ----------------
	for y := 0; y < h-1; y++ {
		y0, y1 := yPos[y], yPos[y+1]
		for x := 0; x < w-1; x++ {
			x0, x1 := float64(x)*pixel, float64(x+1)*pixel
			z00, z10 := zAt(x, y), zAt(x+1, y)
			z01, z11 := zAt(x, y+1), zAt(x+1, y+1)

			writeFacet2(bw, [3]float64{x0, y0, z00}, [3]float64{x1, y0, z10}, [3]float64{x0, y1, z01})
			writeFacet2(bw, [3]float64{x1, y0, z10}, [3]float64{x1, y1, z11}, [3]float64{x0, y1, z01})
		}
	}

	// ---------------- 底面（固定厚度） ----------------
	zBase := -baseThickness
	for y := 0; y < h-1; y++ {
		y0, y1 := yPos[y], yPos[y+1]
		for x := 0; x < w-1; x++ {
			x0, x1 := float64(x)*pixel, float64(x+1)*pixel
			writeFacet2(bw, [3]float64{x0, y0, zBase}, [3]float64{x1, y1, zBase}, [3]float64{x0, y1, zBase})
			writeFacet2(bw, [3]float64{x0, y0, zBase}, [3]float64{x1, y0, zBase}, [3]float64{x1, y1, zBase})
		}
	}

	// ---------------- 前 / 后边 ----------------
	for x := 0; x < w-1; x++ {
		x0, x1 := float64(x)*pixel, float64(x+1)*pixel

		// 前边（y = 0）
		z1, z2 := zAt(x, h-1), zAt(x+1, h-1)
		writeFacet2(bw, [3]float64{x0, 0, zBase}, [3]float64{x1, 0, zBase}, [3]float64{x0, 0, z1})
		writeFacet2(bw, [3]float64{x1, 0, zBase}, [3]float64{x1, 0, z2}, [3]float64{x0, 0, z1})

		// 后边（y = max）
		y := float64(h-1) * pixel
		z1, z2 = zAt(x, 0), zAt(x+1, 0)
		writeFacet2(bw, [3]float64{x0, y, zBase}, [3]float64{x0, y, z1}, [3]float64{x1, y, zBase})
		writeFacet2(bw, [3]float64{x1, y, zBase}, [3]float64{x1, y, z2}, [3]float64{x0, y, z1})
	}

	// ---------------- 左 / 右边 ----------------
	for y := 0; y < h-1; y++ {
		y0, y1 := yPos[y], yPos[y+1]

		// 左边（x = 0）
		z1, z2 := zAt(0, y), zAt(0, y+1)
		writeFacet2(bw, [3]float64{0, y0, zBase}, [3]float64{0, y0, z1}, [3]float64{0, y1, zBase})
		writeFacet2(bw, [3]float64{0, y1, zBase}, [3]float64{0, y0, z1}, [3]float64{0, y1, z2})

		// 右边（x = max）
		x := float64(w-1) * pixel
		z1, z2 = zAt(w-1, y), zAt(w-1, y+1)
		writeFacet2(bw, [3]float64{x, y0, zBase}, [3]float64{x, y1, zBase}, [3]float64{x, y0, z1})
		writeFacet2(bw, [3]float64{x, y1, zBase}, [3]float64{x, y1, z2}, [3]float64{x, y0, z1})
	}

	_, _ = fmt.Fprintln(bw, "endsolid relief_model")
	return nil
}

func writeFacet2(w io.Writer, v1, v2, v3 [3]float64) {
	// 计算法向量（v1 为原点）
	ax, ay, az := v2[0]-v1[0], v2[1]-v1[1], v2[2]-v1[2]
	bx, by, bz := v3[0]-v1[0], v3[1]-v1[1], v3[2]-v1[2]

	nx := ay*bz - az*by
	ny := az*bx - ax*bz
	nz := ax*by - ay*bx

	// 单位化法向量（STL 规范要求，但很多切片器不严格依赖）
	l := math.Sqrt(nx*nx + ny*ny + nz*nz)
	if l > 0 {
		nx, ny, nz = nx/l, ny/l, nz/l
	}

	_, _ = fmt.Fprintf(w, "  facet normal %f %f %f\n", nx, ny, nz)
	_, _ = fmt.Fprintf(w, "    outer loop\n")
	_, _ = fmt.Fprintf(w, "      vertex %f %f %f\n", v1[0], v1[1], v1[2])
	_, _ = fmt.Fprintf(w, "      vertex %f %f %f\n", v2[0], v2[1], v2[2])
	_, _ = fmt.Fprintf(w, "      vertex %f %f %f\n", v3[0], v3[1], v3[2])
	_, _ = fmt.Fprintf(w, "    endloop\n")
	_, _ = fmt.Fprintf(w, "  endfacet\n")
}
