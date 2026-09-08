package ebiten

import (
	"image"
	"image/color"
	"testing"
)

func TestCropAlphaTrimsTransparentPadding(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	for y := 3; y < 6; y++ {
		for x := 2; x < 5; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	got := cropAlpha(src)
	if got.Bounds().Dx() != 3 || got.Bounds().Dy() != 3 {
		t.Fatalf("cropped = %v, want 3x3", got.Bounds())
	}
}

func TestScaleNRGBACapsMaxDimension(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 10, A: 255})
		}
	}
	got := scaleNRGBA(src, 50)
	if got.Bounds().Dx() != 50 || got.Bounds().Dy() != 25 {
		t.Fatalf("scaled = %dx%d, want 50x25", got.Bounds().Dx(), got.Bounds().Dy())
	}
}

func TestAnimFrameWraps(t *testing.T) {
	if animFrame(nil, 10, 1) != nil {
		t.Fatal("empty frames should yield nil")
	}
}
