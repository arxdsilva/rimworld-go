package ebiten

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// DefaultMenuSplashRel is the original main-menu colonist illustration,
// relative to the repo root.
const DefaultMenuSplashRel = "assets/source/raster/ui/main_menu_colonist.png"

const (
	menuRailW   = 300
	menuRailPad = 40
	splashTop   = 24
	debugCharW  = 7
)

func menuRailX(sw int) float32 {
	return float32(sw - menuRailPad - menuRailW)
}

func menuTitleX(sw int, title string) int {
	return (sw - len(title)*debugCharW) / 2
}

// LocateMenuSplash walks parents of cwd looking for DefaultMenuSplashRel.
func LocateMenuSplash() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 12; i++ {
		cand := filepath.Join(dir, DefaultMenuSplashRel)
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("menu splash %q not found from cwd", DefaultMenuSplashRel)
}

func loadPNG(path string) (*ebiten.Image, error) {
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

func (a *App) loadSplash() {
	if a.splash != nil || a.cfg.LoadSplash == nil {
		return
	}
	img, err := a.cfg.LoadSplash()
	if err != nil || img == nil {
		return
	}
	a.splash = img
}

func (a *App) drawMenuSplash(screen *ebiten.Image, sw, sh int) {
	if a.splash == nil {
		return
	}
	iw, ih := a.splash.Bounds().Dx(), a.splash.Bounds().Dy()
	if iw < 1 || ih < 1 {
		return
	}
	h := float64(sh - splashTop)
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
	drawImage(screen, a.splash, 0, y, w, h, false)
}
