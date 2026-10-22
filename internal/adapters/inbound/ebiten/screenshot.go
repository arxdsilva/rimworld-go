package ebiten

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

// Screenshots (UI-016): F10 saves the next drawn frame as a PNG under
// <user config dir>/rimworld-go/screenshots, named after the in-game date
// plus a wall-clock time (presentation only, never the simulation).

type shotState struct {
	pending bool
	// dir overrides the folder (tests write to a temp dir).
	dir  func() (string, error)
	last string // path of the last screenshot
}

// screenshotDir is where screenshots go.
func (a *App) screenshotDir() (string, error) {
	if a.shot.dir != nil {
		return a.shot.dir()
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find config dir: %w", err)
	}
	return filepath.Join(base, "rimworld-go", "screenshots"), nil
}

// screenshotName is rimworld-go-<year>-<quadrum>-<day>-<hhmmss>.png (the
// colony's date; "menu" outside a game).
func (a *App) screenshotName() string {
	clock := a.now().Format("150405")
	if a.renderer == nil {
		return "rimworld-go-menu-" + clock + ".png"
	}
	d := a.renderer.date()
	return fmt.Sprintf("rimworld-go-%d-%d-%d-%s.png", d.Year, d.Quadrum, d.Day, clock)
}

// takeScreenshot reads a w x h frame through read and writes it out.
func (a *App) takeScreenshot(read func(*image.RGBA), w, h int) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	read(img)
	path, err := a.writeScreenshot(img)
	if err != nil {
		a.toast("Screenshot failed: " + err.Error())
		return
	}
	a.shot.last = path
	a.toast("Screenshot saved")
}

func (a *App) writeScreenshot(img image.Image) (string, error) {
	dir, err := a.screenshotDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, a.screenshotName())
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return "", err
	}
	return path, f.Close()
}
