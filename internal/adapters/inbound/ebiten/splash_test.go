package ebiten

import (
	"image/png"
	"os"
	"strings"
	"testing"
)

func TestMainMenuButtonsRightRail(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	mid := float32(a.sw) / 2
	found := 0
	for _, b := range a.mainMenuButtons(a.sw, a.sh) {
		if !strings.HasPrefix(b.ID, "menu.") {
			continue
		}
		found++
		if b.X <= mid {
			t.Fatalf("%s x=%v, want > %v", b.ID, b.X, mid)
		}
	}
	if found < 8 {
		t.Fatalf("menu buttons = %d, want at least 8", found)
	}
}

func TestMenuTitleCentered(t *testing.T) {
	t.Parallel()
	title := "rimworld-go"
	sw := DefaultWindowWidth
	x := menuTitleX(sw, title)
	mid := sw / 2
	textMid := x + int(textW(title, tsHuge))/2
	if textMid < mid-1 || textMid > mid+1 {
		t.Fatalf("title midpoint %d, want ~%d (x=%d)", textMid, mid, x)
	}
}

func TestMenuSplashPNGDecodes(t *testing.T) {
	t.Parallel()
	path, err := LocateMenuSplash()
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() < 64 || b.Dy() < 64 {
		t.Fatalf("splash size %dx%d, want a usable bust", b.Dx(), b.Dy())
	}
	if b.Dy() <= b.Dx() {
		t.Fatalf("splash %dx%d, want portrait", b.Dx(), b.Dy())
	}
	transparent := false
	for y := b.Min.Y; y < b.Max.Y && !transparent; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a == 0 {
				transparent = true
				break
			}
		}
	}
	if !transparent {
		t.Fatal("splash PNG should have transparent pixels so the menu backdrop shows through")
	}
	lastY := b.Max.Y - 1
	bottomOpaque := false
	for x := b.Min.X; x < b.Max.X; x++ {
		_, _, _, a := img.At(x, lastY).RGBA()
		if a != 0 {
			bottomOpaque = true
			break
		}
	}
	if !bottomOpaque {
		t.Fatal("splash PNG last row should meet the bust, not empty transparency")
	}
}

// TestMenuSplashKeepsItsRightEdgeFree (ART-029): the splash is anchored
// bottom-left, so a figure may run off the left and bottom edges but not
// the right: a cut there shows as a straight edge against the backdrop.
// The last columns of the PNG must be fully transparent.
func TestMenuSplashKeepsItsRightEdgeFree(t *testing.T) {
	t.Parallel()
	path, err := LocateMenuSplash()
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	for x := b.Max.X - 2; x < b.Max.X; x++ {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 8<<8 {
				t.Fatalf("splash column %d (of %d) is opaque at y=%d: the figure is cut on the right; extend the source with margin", x, b.Max.X, y)
			}
		}
	}
}
