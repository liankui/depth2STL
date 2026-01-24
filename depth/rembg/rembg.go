package rembg

import (
	"context"
	"image"
)

type Remover interface {
	Remove(ctx context.Context, img image.Image) (image.Image, error)
}

type DefaultRemBG struct{}

func NewDefaultRemBG() *DefaultRemBG {
	return &DefaultRemBG{}
}

func (d *DefaultRemBG) Remove(ctx context.Context, img image.Image) (image.Image, error) {
	return img, nil
}
