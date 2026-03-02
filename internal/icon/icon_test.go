package icon

import (
	"image/color"
	"testing"
)

func TestDraw64(t *testing.T) {
	img := Draw(64)
	if img == nil {
		t.Fatal("Draw(64) returned nil")
	}
	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("expected 64x64, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestDraw16(t *testing.T) {
	img := Draw(16)
	bounds := img.Bounds()
	if bounds.Dx() != 16 || bounds.Dy() != 16 {
		t.Errorf("expected 16x16, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestDrawHasOrange(t *testing.T) {
	img := Draw(64)
	// Top-center pixel (inside circle, above the "N" letter).
	r, g, b, a := img.At(32, 8).RGBA()
	orange := color.RGBA{R: 234, G: 138, B: 0, A: 255}
	or, og, ob, oa := orange.RGBA()
	if r != or || g != og || b != ob || a != oa {
		t.Errorf("pixel (32,8) = (%d,%d,%d,%d), want orange (%d,%d,%d,%d)", r, g, b, a, or, og, ob, oa)
	}
}

func TestDrawCornersTransparent(t *testing.T) {
	img := Draw(64)
	// Corner pixel (0,0) should be outside the circle — transparent (zero alpha).
	_, _, _, a := img.At(0, 0).RGBA()
	if a != 0 {
		t.Errorf("corner pixel alpha = %d, want 0 (transparent)", a)
	}
}
