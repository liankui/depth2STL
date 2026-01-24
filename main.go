package main

import (
	"flag"
	"image"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/chaos-io/depth2STL/depth"
	"github.com/chaos-io/depth2STL/depth/rembg"
	"github.com/chaos-io/depth2STL/stl"
	"github.com/chaos-io/depth2STL/util"
	"github.com/segmentio/ksuid"
)

/*
  - `image_path`：要转换的图像本地路径或 URL
  - `model_width`：3D 模型的宽度（毫米，默认：50.0）
  - `model_thickness`：3D 模型的最大厚度/高度（毫米，默认：5.0）
  - `base_thickness`：底座厚度（毫米，默认：2.0）
  - `skip_depth`：是否直接使用图像或生成深度图（默认：true）
  - `invert_depth`：是否反转浮雕（明亮区域变低而不是高，默认：false）
  - `detail_level`：控制处理图像的分辨率（默认：1.0）。当 detail_level = 1.0 时，图像以 320px 分辨率处理，生成的 STL 文件通常在 100MB 以内。
    较高的值可以提高细节质量，但会显著增加处理时间和 STL 文件大小。例如，将 detail_level 值加倍可能会使文件大小增加 4 倍或更多，请谨慎使用。
*/
var (
	imagePath       = flag.String("imagePath", "", "local path or web URL to the input image file")
	modelWidth      = flag.Float64("modelWidth", 50.0, "width of the 3D model in mm (default: 50.0)")
	modelThickness  = flag.Float64("modelThickness", 5.0, "maximum thickness/height of the 3D model in mm (default: 5.0)")
	baseThickness   = flag.Float64("baseThickness", 2.0, "tickness of the base in mm (default: 2.0)")
	skipDepth       = flag.Bool("skipDepth", true, "whether to use the image directly or generate a depth map (default: true)")
	preProcessModel = flag.String("preProcessModel", "", "whether to use the rembg model pre process (default: )")
	invertDepth     = flag.Bool("invertDepth", false, "invert the relief (bright areas become low instead of high) (default: false)")
	detailLevel     = flag.Float64("detailLevel", 1.0, "level of detail level (default: 1.0)")
)

func main() {
	flag.Parse()

	Execute()
}

func Execute() {
	inputPath := *imagePath
	outputDir := "./output"
	_ = os.MkdirAll(outputDir, os.ModePerm)

	var img image.Image
	var err error
	if strings.HasPrefix(inputPath, "http://") || strings.HasPrefix(inputPath, "https://") {
		img, err = util.DownloadImage(inputPath)
	} else {
		img, err = util.OpenImage(inputPath)
	}
	if err != nil {
		slog.Error("failed to load image", "error", err)
		return
	}

	if *preProcessModel == rembg.BiRefNetModel {
		p := &depth.Preprocessor{RemBG: rembg.NewBiRefNetRemBG()}
		img, err = p.ImagePreprocess(img)
		if err != nil {
			slog.Error("failed to preprocess image", "error", err)
			return
		}
	}

	var depthMap *image.Gray
	if *skipDepth { // 直接使用灰度图
		depthMap = depth.ConvertToGray(img)
	} else { // 调用统一的深度图生成函数
		depthMap = depth.GenerateDepthMap2(img, *detailLevel, *invertDepth)
	}

	imgId := ksuid.New().String()
	depthPath := filepath.Join(outputDir, imgId+"_depth_map.png")
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

	stlPath := filepath.Join(outputDir, imgId+".stl")
	err = stl.GenerateSTL2(depthMap, stlPath, *modelWidth, *modelThickness, *baseThickness)
	if err != nil {
		slog.Error("failed to generate stl", "error", err)
		return
	}

	slog.Info("generated stl", "stl path", stlPath, "depth path", depthPath)
}
