package stl

import (
	"fmt"
	"image/png"
	"log"
	"os"
	"testing"

	"github.com/chaos-io/depth2STL/depth"
	"github.com/chaos-io/depth2STL/util"
	"github.com/segmentio/ksuid"
)

func genSTL(name string) error {
	myImage, err := util.OpenImage(name)
	if err != nil {
		return fmt.Errorf("faild to open image, %v", err)
	}

	got := depth.GenerateDepthMap(myImage, 1, false)
	dir := "../output/" + ksuid.New().String()
	f, err := os.Create(dir + "_depth.png")
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

	err = GenerateSTL2(got, dir+".stl", 50, 5, 2)
	if err != nil {
		return fmt.Errorf("faild to generate STL, %v", err)
	}

	log.Printf("generated stl %s", f.Name())
	return nil
}

var dawnbreaker = "../testdata/Dota_2_Monster_Hunter_codex_dawnbreaker_gameasset.png"

func TestGenerateSTL(t *testing.T) {
	defer util.Trace("gen stl")
	err := genSTL(dawnbreaker)
	if err != nil {
		t.Errorf("faild to generate STL, %v", err)
	}
}
