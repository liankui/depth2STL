package depth

import "image"

type BackgroundRemover interface {
	Remove(img image.Image) (image.Image, error)
}

type DefaultRemBG struct{}

func NewDefaultRemBG() *DefaultRemBG {
	return &DefaultRemBG{}
}

func (d *DefaultRemBG) Remove(img image.Image) (image.Image, error) {
	// TODO: 通过 HTTP 调用 BiRefNet rembg 推理
	return img, nil
}
