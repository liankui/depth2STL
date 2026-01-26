package stl

import (
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaos-io/depth2STL/depth"
	"github.com/chaos-io/depth2STL/util"
)

var (
	modelWidth, modelThickness, baseThickness float64 = 30, 2, 1
)

func genSTL(name string) error {
	myImage, err := util.OpenImage(name)
	if err != nil {
		return fmt.Errorf("faild to open image (%s), %v", name, err)
	}

	outDir := "../output"
	stlDir := filepath.Join(outDir, fmt.Sprintf("%s-%.f-%.f-%.f", "stl", modelWidth, modelThickness, baseThickness))
	_ = os.MkdirAll(outDir, os.ModePerm)
	_ = os.MkdirAll(stlDir, os.ModePerm)

	got := depth.GenerateDepthMap(myImage, 1, false)
	pngPath := filepath.Join(outDir, filepath.Base(name))
	f, err := os.Create(pngPath)
	if err != nil {
		return fmt.Errorf("faild to create output image, %v", err)
	}
	defer func() {
		_ = f.Close()
	}()

	err = png.Encode(f, got)
	if err != nil {
		return fmt.Errorf("png encode error = %v", err)
	}

	stlPath := filepath.Join(stlDir, filepath.Base(name))
	stlPath = strings.Replace(stlPath, ".png", ".stl", 1)
	err = GenerateSTL2(got, stlPath, modelWidth, modelThickness, baseThickness)
	if err != nil {
		return fmt.Errorf("faild to generate STL, %v", err)
	}

	log.Printf("generated stl %s", f.Name())
	return nil
}

var dawnbreaker = "../testdata/Dota_2_Monster_Hunter_codex_dawnbreaker_gameasset.png"
var my = "../testdata/my_image1.png"
var my1 = "../testdata/ComfyUI_temp_ipggf_00006_.png"
var my11 = "../testdata/ComfyUI_temp_ipggf_00007_.png"
var my20 = "../testdata/Dota_2_Monster_Hunter_codex_centaur_warrunner_gameasset.png"
var my21 = "../images/Dota_2_Monster_Hunter_codex_anti-mage_persona_gameasset.png"

func TestGenerateSTL(t *testing.T) {
	defer util.Trace("gen stl")
	err := genSTL(my21)
	if err != nil {
		t.Errorf("faild to generate STL, %v", err)
	}
}

func TestBatchGenerateSTL(t *testing.T) {
	defer util.Trace("batch gen stl")

	srcDir := "../images"
	// count := 0
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		// count++
		// if count > 1 {
		// 	return fmt.Errorf("exceeded %d images", count)
		// }

		err = genSTL(path)
		if err != nil {
			log.Printf("faild to generate STL (%s), %v", info.Name(), err)
		}
		return nil
	})
	if err != nil {
		t.Errorf("faild to generate STL, %v", err)
	}
}
