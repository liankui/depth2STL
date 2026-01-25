package depth

import (
	"image/png"
	"os"
	"testing"

	"github.com/chaos-io/depth2STL/util"
	"github.com/segmentio/ksuid"
)

func TestImagePreprocessor_PreprocessImage(t *testing.T) {
	path := "../testdata/my_image1.png"
	myImage, err := util.OpenImage(path)
	if err != nil {
		t.Errorf("faild to open image, %v", err)
		return
	}

	p := NewPreprocessor(path)
	got, err := p.ImagePreprocess(myImage)
	if err != nil {
		t.Errorf("faild to preprocess image, %v", err)
		return
	}

	f, err := os.Create("../output/" + ksuid.New().String() + "_preprocess.png")
	if err != nil {
		t.Errorf("ImagePreprocess() error = %v", err)
		return
	}
	defer func() {
		_ = f.Close()
	}()

	t.Logf("image name: %s", f.Name())
	err = png.Encode(f, got)
	if err != nil {
		t.Errorf("png encode error = %v", err)
	}
}
