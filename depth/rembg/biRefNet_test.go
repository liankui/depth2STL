package rembg

import (
	"context"
	"testing"
)

var dawnbreaker = "../../testdata/Dota_2_Monster_Hunter_codex_dawnbreaker_gameasset.png"
var centaur = "../../testdata/Dota_2_Monster_Hunter_codex_centaur_warrunner_gameasset.png"

func TestBiRefNetRemBG_uploadImage(t *testing.T) {
	b := NewBiRefNetRemBG(centaur)
	err := b.uploadImage(context.Background())
	if err != nil {
		t.Errorf("upload image error = %v", err)
	}
}

func TestBiRefNetRemBG_prompt(t *testing.T) {
	b := NewBiRefNetRemBG(centaur)
	err := b.prompt(context.Background())
	if err != nil {
		t.Errorf("prompt error = %v", err)
	}
}
