package perf

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

// Shot is the screenshot state.
type Shot struct {
	Pending bool
	// Dir overrides the folder (tests write to a temp dir).
	Dir  func() (string, error)
	Last string // path of the last screenshot
}

// dir is where screenshots go.
func (m *Meter) dir() (string, error) {
	if m.Shot.Dir != nil {
		return m.Shot.Dir()
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find config dir: %w", err)
	}
	return filepath.Join(base, "rimworld-go", "screenshots"), nil
}

// name is rimworld-go-<year>-<quadrum>-<day>-<hhmmss>.png (the colony's
// date; "menu" outside a game).
func (m *Meter) name() string {
	clock := m.sh.Now().Format("150405")
	env := m.sh.Env()
	if !env.InGame {
		return "rimworld-go-menu-" + clock + ".png"
	}
	d := env.Date
	return fmt.Sprintf("rimworld-go-%d-%d-%d-%s.png", d.Year, d.Quadrum, d.Day, clock)
}

// TakeScreenshot reads a w x h frame through read and writes it out.
func (m *Meter) TakeScreenshot(read func(*image.RGBA), w, h int) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	read(img)
	path, err := m.write(img)
	if err != nil {
		m.sh.Toast("Screenshot failed: " + err.Error())
		return
	}
	m.Shot.Last = path
	m.sh.Toast("Screenshot saved")
}

func (m *Meter) write(img image.Image) (string, error) {
	dir, err := m.dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, m.name())
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
