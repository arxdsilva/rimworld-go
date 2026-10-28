package mainmenu

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/platform/assetroot"
)

// DefaultMenuSplashRel is the original main-menu colonist illustration,
// relative to the repo root.
const DefaultMenuSplashRel = "assets/source/raster/ui/main_menu_colonist.png"

const (
	// RailW is the button rail's width.
	RailW     = 300
	railPad   = 40
	splashTop = 24
)

// RailX is the rail's left edge on a screen sw wide.
func RailX(sw int) float32 {
	return float32(sw - railPad - RailW)
}

// TitleX is the left edge that centres title on a screen sw wide.
func TitleX(sw int, title string) int {
	return (sw - int(gfx.TextW(title, gfx.Huge))) / 2
}

// LocateMenuSplash walks parents of cwd looking for DefaultMenuSplashRel.
func LocateMenuSplash() (string, error) {
	p, err := assetroot.Find(filepath.FromSlash(DefaultMenuSplashRel), false)
	if err != nil {
		return "", fmt.Errorf("menu splash: %w", err)
	}
	return p, nil
}

// LoadPNG reads a PNG file as an image.
func LoadPNG(path string) (*ebiten.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return ebiten.NewImageFromImage(src), nil
}

// LoadSplash reads the hero illustration once; a failure leaves the menu
// without it.
func (s *Screen) LoadSplash() {
	if s.splash != nil {
		return
	}
	img, err := s.sh.LoadSplash()
	if err != nil || img == nil {
		return
	}
	s.splash = img
}

func (s *Screen) drawSplash(screen *ebiten.Image, sw, sh int) {
	if s.splash == nil {
		return
	}
	// Full-screen background: physical size, unscaled.
	if b := screen.Bounds(); b.Dx() > 0 && b.Dy() > 0 {
		sw, sh = b.Dx(), b.Dy()
	}
	iw, ih := s.splash.Bounds().Dx(), s.splash.Bounds().Dy()
	if iw < 1 || ih < 1 {
		return
	}
	h := float64(sh) - splashTop*gfx.UIScale()
	w := h * float64(iw) / float64(ih)
	maxW := float64(sw) * 0.55
	if w > maxW {
		w = maxW
		h = w * float64(ih) / float64(iw)
	}
	y := float64(sh) - h
	if y < 0 {
		y = 0
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(w/float64(iw), h/float64(ih))
	op.GeoM.Translate(0, y)
	screen.DrawImage(s.splash, op)
}
