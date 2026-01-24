package depth

import (
	_ "embed"
	"fmt"
	"image/png"
	"log"
	"os"
	"testing"

	"github.com/chaos-io/depth2STL/util"
	"github.com/segmentio/ksuid"
)

func genDepthMap(name string) error {
	myImage, err := util.OpenImage(name)
	if err != nil {
		return fmt.Errorf("faild to open image, %v", err)
	}

	got := GenerateDepthMap2(myImage, 1, false)
	f, err := os.Create("../output/" + ksuid.New().String() + "_depth.png")
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

	log.Printf("generated depth map %s", f.Name())
	return nil
}

var dawnbreaker = "../testdata/Dota_2_Monster_Hunter_codex_dawnbreaker_gameasset.png"

func TestGenerateDepthMap(t *testing.T) {
	defer util.Trace("gen depth_map")()
	err := genDepthMap(dawnbreaker)
	if err != nil {
		t.Errorf("faild to generate depth map, %v", err)
	}
}
