package stl

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"math"
	"os"
)

const (
	maxTrianglesL1 = 250_000
	maxTrianglesL2 = 600_000
	maxTrianglesL3 = 1_000_000
)

func triangleBudgetByDetailLevel(detailLevel int) int {
	switch {
	case detailLevel <= 1:
		return maxTrianglesL1
	case detailLevel == 2:
		return maxTrianglesL2
	default:
		return maxTrianglesL3
	}
}

func gridSizeByStep(length int, step float64) int {
	if length < 2 {
		return 0
	}
	if step <= 0 {
		step = 1
	}
	// Always include both ends: 0 and length-1.
	return int(math.Ceil(float64(length-1)/step)) + 1
}

func faceCountByStep(w, h int, step float64) int {
	if w < 2 || h < 2 {
		return 0
	}
	if step <= 0 {
		step = 1
	}

	gridW := gridSizeByStep(w, step)
	gridH := gridSizeByStep(h, step)
	if gridW < 2 || gridH < 2 {
		return 0
	}

	topFaces := (gridW - 1) * (gridH - 1) * 2
	bottomFaces := max(0, 2*(gridW+gridH)-4)
	sideFaces := (gridW-1)*4 + (gridH-1)*4

	return topFaces + bottomFaces + sideFaces
}

// findStepForTriangleBudget returns the smallest step that keeps faces <= budget.
func findStepForTriangleBudget(w, h int, preferredStep float64, budget int) float64 {
	if preferredStep <= 0 {
		preferredStep = 1
	}
	maxStep := float64(min(w-1, h-1))
	if maxStep < 1 {
		maxStep = 1
	}
	if preferredStep > maxStep {
		preferredStep = maxStep
	}
	if budget <= 0 || faceCountByStep(w, h, preferredStep) <= budget {
		return preferredStep
	}

	low := preferredStep
	high := preferredStep
	for faceCountByStep(w, h, high) > budget {
		high *= 2
		if high >= maxStep {
			return maxStep
		}
	}

	for i := 0; i < 32; i++ {
		mid := (low + high) / 2
		if faceCountByStep(w, h, mid) > budget {
			low = mid
		} else {
			high = mid
		}
	}
	return high
}

func buildAxisSamples(length int, step float64) []float64 {
	if length < 2 {
		return nil
	}
	size := gridSizeByStep(length, step)
	samples := make([]float64, size)
	maxV := float64(length - 1)
	for i := 0; i < size; i++ {
		v := float64(i) * step
		if v > maxV {
			v = maxV
		}
		samples[i] = v
	}
	return samples
}

func buildHeightField(depthMap *image.Gray, xSamples, ySamples []float64, modelThickness float64) []float64 {
	w, h := len(xSamples), len(ySamples)
	height := make([]float64, w*h)
	imgW := depthMap.Bounds().Dx()
	imgH := depthMap.Bounds().Dy()

	get := func(x, y int) float64 {
		return float64(depthMap.Pix[y*depthMap.Stride+x]) / 255.0
	}

	for gy, y := range ySamples {
		y0 := int(math.Floor(y))
		y1 := min(y0+1, imgH-1)
		fy := y - float64(y0)
		for gx, x := range xSamples {
			x0 := int(math.Floor(x))
			x1 := min(x0+1, imgW-1)
			fx := x - float64(x0)

			z00 := get(x0, y0)
			z10 := get(x1, y0)
			z01 := get(x0, y1)
			z11 := get(x1, y1)

			z0 := z00*(1-fx) + z10*fx
			z1 := z01*(1-fx) + z11*fx
			z := z0*(1-fy) + z1*fy
			z = math.Pow(z, 0.7)
			height[gy*w+gx] = z * modelThickness
		}
	}

	return height
}

func buildModelCoordinates(xSamples, ySamples []float64, pixel float64, h int) ([]float32, []float32) {
	pixel32 := float32(pixel)
	xModel := make([]float32, len(xSamples))
	for i, xs := range xSamples {
		xModel[i] = float32(xs) * pixel32
	}

	yModel := make([]float32, len(ySamples))
	for i, ys := range ySamples {
		yModel[i] = float32(float64(h)-ys-1) * pixel32
	}

	return xModel, yModel
}

type binarySTLWriter struct {
	w      *bufio.Writer
	record [50]byte
}

func newBinarySTLWriter(w *bufio.Writer) *binarySTLWriter {
	return &binarySTLWriter{w: w}
}

func (bw *binarySTLWriter) writeHeader(totalFaces int) error {
	header := make([]byte, 80)
	copy(header, []byte("Relief STL Binary (GenerateSTL5)"))
	if _, err := bw.w.Write(header); err != nil {
		return err
	}
	return binary.Write(bw.w, binary.LittleEndian, uint32(totalFaces))
}

func (bw *binarySTLWriter) writeTri(v1, v2, v3 [3]float32) error {
	nx, ny, nz := calcNormal(v1, v2, v3)
	off := 0
	putF32 := func(v float32) {
		binary.LittleEndian.PutUint32(bw.record[off:off+4], math.Float32bits(v))
		off += 4
	}
	putF32(nx)
	putF32(ny)
	putF32(nz)
	putF32(v1[0])
	putF32(v1[1])
	putF32(v1[2])
	putF32(v2[0])
	putF32(v2[1])
	putF32(v2[2])
	putF32(v3[0])
	putF32(v3[1])
	putF32(v3[2])
	binary.LittleEndian.PutUint16(bw.record[48:50], 0)
	_, err := bw.w.Write(bw.record[:])
	return err
}

type point2 struct {
	x float32
	y float32
}

func buildBottomBoundary(xModel, yModel []float32) []point2 {
	gridW, gridH := len(xModel), len(yModel)
	boundary := make([]point2, 0, max(0, 2*(gridW+gridH)-4))

	for x := 0; x < gridW; x++ {
		boundary = append(boundary, point2{x: xModel[x], y: yModel[0]})
	}
	for y := 1; y < gridH; y++ {
		boundary = append(boundary, point2{x: xModel[gridW-1], y: yModel[y]})
	}
	for x := gridW - 2; x >= 0; x-- {
		boundary = append(boundary, point2{x: xModel[x], y: yModel[gridH-1]})
	}
	for y := gridH - 2; y > 0; y-- {
		boundary = append(boundary, point2{x: xModel[0], y: yModel[y]})
	}

	return boundary
}

func writeTopSurface(w *binarySTLWriter, xModel, yModel []float32, height []float64, gridW, gridH int) error {
	for y := 0; y < gridH-1; y++ {
		row := y * gridW
		nextRow := (y + 1) * gridW
		y0 := yModel[y]
		y1 := yModel[y+1]
		for x := 0; x < gridW-1; x++ {
			x0 := xModel[x]
			x1 := xModel[x+1]
			z00 := float32(height[row+x])
			z10 := float32(height[row+x+1])
			z01 := float32(height[nextRow+x])
			z11 := float32(height[nextRow+x+1])

			if err := w.writeTri(
				[3]float32{x0, y0, z00},
				[3]float32{x1, y0, z10},
				[3]float32{x0, y1, z01},
			); err != nil {
				return err
			}
			if err := w.writeTri(
				[3]float32{x1, y0, z10},
				[3]float32{x1, y1, z11},
				[3]float32{x0, y1, z01},
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeBottomSurfaceFan(w *binarySTLWriter, xModel, yModel []float32, zBase32 float32) error {
	boundary := buildBottomBoundary(xModel, yModel)
	center := [3]float32{
		(xModel[0] + xModel[len(xModel)-1]) * 0.5,
		(yModel[0] + yModel[len(yModel)-1]) * 0.5,
		zBase32,
	}
	for i := 0; i < len(boundary); i++ {
		j := (i + 1) % len(boundary)
		v1 := [3]float32{boundary[i].x, boundary[i].y, zBase32}
		v2 := [3]float32{boundary[j].x, boundary[j].y, zBase32}
		if err := w.writeTri(center, v2, v1); err != nil {
			return err
		}
	}
	return nil
}

func writeSideWalls(w *binarySTLWriter, xModel, yModel []float32, height []float64, gridW, gridH int, zBase32 float32) error {
	for x := 0; x < gridW-1; x++ {
		x0 := xModel[x]
		x1 := xModel[x+1]
		z1 := float32(height[(gridH-1)*gridW+x])
		z2 := float32(height[(gridH-1)*gridW+x+1])
		y := yModel[gridH-1]

		if err := w.writeTri([3]float32{x0, y, zBase32}, [3]float32{x1, y, zBase32}, [3]float32{x0, y, z1}); err != nil {
			return err
		}
		if err := w.writeTri([3]float32{x1, y, zBase32}, [3]float32{x1, y, z2}, [3]float32{x0, y, z1}); err != nil {
			return err
		}
	}

	yMax := yModel[0]
	for x := 0; x < gridW-1; x++ {
		x0 := xModel[x]
		x1 := xModel[x+1]
		z1 := float32(height[x])
		z2 := float32(height[x+1])

		if err := w.writeTri([3]float32{x0, yMax, zBase32}, [3]float32{x0, yMax, z1}, [3]float32{x1, yMax, zBase32}); err != nil {
			return err
		}
		if err := w.writeTri([3]float32{x1, yMax, zBase32}, [3]float32{x0, yMax, z1}, [3]float32{x1, yMax, z2}); err != nil {
			return err
		}
	}

	for y := 0; y < gridH-1; y++ {
		y0 := yModel[y]
		y1 := yModel[y+1]
		z1 := float32(height[y*gridW])
		z2 := float32(height[(y+1)*gridW])

		if err := w.writeTri([3]float32{0, y0, zBase32}, [3]float32{0, y0, z1}, [3]float32{0, y1, zBase32}); err != nil {
			return err
		}
		if err := w.writeTri([3]float32{0, y1, zBase32}, [3]float32{0, y0, z1}, [3]float32{0, y1, z2}); err != nil {
			return err
		}
	}

	xMax := xModel[gridW-1]
	for y := 0; y < gridH-1; y++ {
		y0 := yModel[y]
		y1 := yModel[y+1]
		z1 := float32(height[y*gridW+(gridW-1)])
		z2 := float32(height[(y+1)*gridW+(gridW-1)])

		if err := w.writeTri([3]float32{xMax, y0, zBase32}, [3]float32{xMax, y1, zBase32}, [3]float32{xMax, y0, z1}); err != nil {
			return err
		}
		if err := w.writeTri([3]float32{xMax, y1, zBase32}, [3]float32{xMax, y1, z2}, [3]float32{xMax, y0, z1}); err != nil {
			return err
		}
	}

	return nil
}

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

func GenerateSTL4(depthMap *image.Gray, outputPath string, modelWidth, modelThickness, baseThickness float64) error {
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
	defer f.Close()

	bw := bufio.NewWriter(f)
	defer bw.Flush()

	fmt.Fprintln(bw, "solid relief_model")

	// ---------- 双线性采样 ----------
	zAt := func(x, y float64) float64 {
		x0 := int(math.Floor(x))
		y0 := int(math.Floor(y))
		x1 := min(x0+1, w-1)
		y1 := min(y0+1, h-1)

		fx := x - float64(x0)
		fy := y - float64(y0)

		get := func(ix, iy int) float64 {
			return float64(depthMap.Pix[iy*depthMap.Stride+ix]) / 255.0
		}

		z00 := get(x0, y0)
		z10 := get(x1, y0)
		z01 := get(x0, y1)
		z11 := get(x1, y1)

		z0 := z00*(1-fx) + z10*fx
		z1 := z01*(1-fx) + z11*fx
		z := z0*(1-fy) + z1*fy

		z = math.Pow(z, 0.7) // 非线性增强
		return z * modelThickness
	}

	zBase := -baseThickness

	// =========================================================
	// 1️⃣ 顶面
	// =========================================================
	for y := 0; y < h-1; y++ {
		for x := 0; x < w-1; x++ {

			x0 := float64(x) * pixel
			x1 := float64(x+1) * pixel
			y0 := float64(h-y-1) * pixel
			y1 := float64(h-y-2) * pixel

			z00 := zAt(float64(x), float64(y))
			z10 := zAt(float64(x+1), float64(y))
			z01 := zAt(float64(x), float64(y+1))
			z11 := zAt(float64(x+1), float64(y+1))

			writeFacet2(bw, [3]float64{x0, y0, z00}, [3]float64{x1, y0, z10}, [3]float64{x0, y1, z01})
			writeFacet2(bw, [3]float64{x1, y0, z10}, [3]float64{x1, y1, z11}, [3]float64{x0, y1, z01})
		}
	}

	// =========================================================
	// 2️⃣ 底面（完全封闭）
	// =========================================================
	for y := 0; y < h-1; y++ {
		for x := 0; x < w-1; x++ {

			x0 := float64(x) * pixel
			x1 := float64(x+1) * pixel
			y0 := float64(h-y-1) * pixel
			y1 := float64(h-y-2) * pixel

			writeFacet2(bw, [3]float64{x0, y0, zBase}, [3]float64{x0, y1, zBase}, [3]float64{x1, y1, zBase})
			writeFacet2(bw, [3]float64{x0, y0, zBase}, [3]float64{x1, y1, zBase}, [3]float64{x1, y0, zBase})
		}
	}

	// =========================================================
	// 3️⃣ 四边侧壁（完全闭合）
	// =========================================================

	// ---- 前边 (y=0)
	for x := 0; x < w-1; x++ {
		x0 := float64(x) * pixel
		x1 := float64(x+1) * pixel

		z1 := zAt(float64(x), float64(h-1))
		z2 := zAt(float64(x+1), float64(h-1))

		writeFacet2(bw, [3]float64{x0, 0, zBase}, [3]float64{x0, 0, z1}, [3]float64{x1, 0, zBase})
		writeFacet2(bw, [3]float64{x1, 0, zBase}, [3]float64{x0, 0, z1}, [3]float64{x1, 0, z2})
	}

	// ---- 后边 (y=max)
	yMax := float64(h-1) * pixel
	for x := 0; x < w-1; x++ {
		x0 := float64(x) * pixel
		x1 := float64(x+1) * pixel

		z1 := zAt(float64(x), 0)
		z2 := zAt(float64(x+1), 0)

		writeFacet2(bw, [3]float64{x0, yMax, zBase}, [3]float64{x1, yMax, zBase}, [3]float64{x0, yMax, z1})
		writeFacet2(bw, [3]float64{x1, yMax, zBase}, [3]float64{x1, yMax, z2}, [3]float64{x0, yMax, z1})
	}

	// ---- 左边 (x=0)
	for y := 0; y < h-1; y++ {
		y0 := float64(h-y-1) * pixel
		y1 := float64(h-y-2) * pixel

		z1 := zAt(0, float64(y))
		z2 := zAt(0, float64(y+1))

		writeFacet2(bw, [3]float64{0, y0, zBase}, [3]float64{0, y1, zBase}, [3]float64{0, y0, z1})
		writeFacet2(bw, [3]float64{0, y1, zBase}, [3]float64{0, y1, z2}, [3]float64{0, y0, z1})
	}

	// ---- 右边 (x=max)
	xMax := float64(w-1) * pixel
	for y := 0; y < h-1; y++ {
		y0 := float64(h-y-1) * pixel
		y1 := float64(h-y-2) * pixel

		z1 := zAt(float64(w-1), float64(y))
		z2 := zAt(float64(w-1), float64(y+1))

		writeFacet2(bw, [3]float64{xMax, y0, zBase}, [3]float64{xMax, y0, z1}, [3]float64{xMax, y1, zBase})
		writeFacet2(bw, [3]float64{xMax, y1, zBase}, [3]float64{xMax, y0, z1}, [3]float64{xMax, y1, z2})
	}

	fmt.Fprintln(bw, "endsolid relief_model")
	return nil
}

// GenerateSTL5
// 1. depthMap → heightField（缓存）
// 2. heightField → mesh（避免重复计算）
// 3. mesh → Binary STL（高速输出）
func GenerateSTL5(depthMap *image.Gray, outputPath string, modelWidth, modelThickness, baseThickness float64, detailLevel int) error {
	b := depthMap.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 2 || h < 2 {
		return fmt.Errorf("depth map too small")
	}

	if detailLevel < 1 {
		detailLevel = 1
	}

	preferredStep := 1.0 / float64(detailLevel)
	maxTriangles := triangleBudgetByDetailLevel(detailLevel)
	step := findStepForTriangleBudget(w, h, preferredStep, maxTriangles)
	pixel := modelWidth / float64(w)
	xSamples := buildAxisSamples(w, step)
	ySamples := buildAxisSamples(h, step)
	gridW, gridH := len(xSamples), len(ySamples)
	height := buildHeightField(depthMap, xSamples, ySamples, modelThickness)

	totalFaces := faceCountByStep(w, h, step)
	if totalFaces <= 0 {
		return fmt.Errorf("invalid face count")
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	buffered := bufio.NewWriterSize(f, 1<<20)
	defer buffered.Flush()
	stlWriter := newBinarySTLWriter(buffered)
	if err := stlWriter.writeHeader(totalFaces); err != nil {
		return err
	}

	xModel, yModel := buildModelCoordinates(xSamples, ySamples, pixel, h)
	zBase32 := float32(-baseThickness)

	if err := writeTopSurface(stlWriter, xModel, yModel, height, gridW, gridH); err != nil {
		return err
	}
	if err := writeBottomSurfaceFan(stlWriter, xModel, yModel, zBase32); err != nil {
		return err
	}
	if err := writeSideWalls(stlWriter, xModel, yModel, height, gridW, gridH, zBase32); err != nil {
		return err
	}

	return nil
}

func calcNormal(v1, v2, v3 [3]float32) (float32, float32, float32) {
	ax := v2[0] - v1[0]
	ay := v2[1] - v1[1]
	az := v2[2] - v1[2]

	bx := v3[0] - v1[0]
	by := v3[1] - v1[1]
	bz := v3[2] - v1[2]

	nx := ay*bz - az*by
	ny := az*bx - ax*bz
	nz := ax*by - ay*bx

	length := float32(math.Sqrt(float64(nx*nx + ny*ny + nz*nz)))
	if length < 1e-6 {
		return 0, 0, 1
	}

	return nx / length, ny / length, nz / length
}
