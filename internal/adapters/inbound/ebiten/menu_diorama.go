package ebiten

import (
	"bufio"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/platform/config"
)

// The main-menu background: a dusk colony rendered in Blender
// (assets/source/blender/artlib/scenes/menu.py, make art-menu) with
// presentation-only motion on top: a slow pan and zoom, lamp and fire
// glow layers, the campfire flame, embers and dust. No simulation runs.

// menuDioramaMetaRel is the diorama's metadata, relative to the assets root.
const menuDioramaMetaRel = "metadata/" + menuMetaFamily + "/menu_diorama.yaml"

// menuMetaFamily is the metadata family of menu art; the sprite catalog
// skips it.
const menuMetaFamily = "ui_menu"

const (
	menuDriftPeriod          = 90 * 60 // ticks per zoom cycle
	menuZoomMin, menuZoomMax = 1.04, 1.10
	menuMotes, menuEmbers    = 36, 8
	menuScrimDiv             = 4 // scrim resolution divisor
)

type menuDiorama struct {
	levels       []*ebiten.Image // full, 1/2, 1/4: no shimmer when the pan minifies
	lamp, fire   *ebiten.Image   // additive glow, half resolution
	w, h         int
	tilePx       float64
	fires, lamps [][2]float64 // base-image pixels
	scrim        *ebiten.Image
	scrimW       int
	scrimH       int
}

// loadMenuDiorama reads the diorama metadata and images under root (the
// assets directory).
func loadMenuDiorama(root string) (*menuDiorama, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(menuDioramaMetaRel)))
	if err != nil {
		return nil, err
	}
	m, err := parseArtMeta(string(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", menuDioramaMetaRel, err)
	}
	img := func(rel string) (*ebiten.Image, error) {
		if rel == "" {
			return nil, nil
		}
		return loadPNG(filepath.Join(root, filepath.FromSlash(rel)))
	}
	base, err := img(m.Image)
	if err != nil {
		return nil, err
	}
	d := &menuDiorama{levels: []*ebiten.Image{base}, w: m.Canvas[0], h: m.Canvas[1], tilePx: artTilePx}
	if d.lamp, err = img(metaScalar(string(raw), "glow_lamp")); err != nil {
		return nil, err
	}
	if d.fire, err = img(metaScalar(string(raw), "glow_fire")); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(m.Attach))
	for k := range m.Attach {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch {
		case strings.HasPrefix(k, "fire"):
			d.fires = append(d.fires, m.Attach[k])
		case strings.HasPrefix(k, "lamp"):
			d.lamps = append(d.lamps, m.Attach[k])
		}
	}
	for i := 1; i < 3; i++ {
		prev := d.levels[i-1]
		w, h := prev.Bounds().Dx()/2, prev.Bounds().Dy()/2
		if w < 1 || h < 1 {
			break
		}
		lv := ebiten.NewImage(w, h)
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(0.5, 0.5)
		lv.DrawImage(prev, op)
		d.levels = append(d.levels, lv)
	}
	return d, nil
}

// metaScalar is a top-level "key: value" from flat metadata YAML.
func metaScalar(src, key string) string {
	sc := bufio.NewScanner(strings.NewReader(src))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, " ") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(k) == key {
			return strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return ""
}

// menuView places an iw x ih image over an sw x sh screen at tick t:
// cover-fit, eased zoom, and a Lissajous drift that never uncovers an edge.
func menuView(iw, ih, sw, sh int, t float64) (scale, ox, oy float64) {
	if iw < 1 || ih < 1 {
		return 1, 0, 0
	}
	cover := math.Max(float64(sw)/float64(iw), float64(sh)/float64(ih))
	ph := 2 * math.Pi * t / menuDriftPeriod
	z := menuZoomMin + (menuZoomMax-menuZoomMin)*(0.5-0.5*math.Cos(ph))
	scale = cover * z
	slackX := float64(iw)*scale - float64(sw)
	slackY := float64(ih)*scale - float64(sh)
	ox = -slackX * (0.5 + 0.45*math.Sin(ph*0.73))
	oy = -slackY * (0.5 + 0.45*math.Sin(ph*0.73/1.37+1))
	return scale, ox, oy
}

// menuFlicker is the campfire flicker of fire.go, 0.5..1.
func menuFlicker(t float64, seed int) float64 {
	t += float64(seed) * 17
	return 0.75 + 0.25*math.Sin(t*0.31)*math.Sin(t*0.13+2)
}

// menuLampBreath is a slow lamp pulse, 0.9..1.
func menuLampBreath(t float64) float64 {
	return 0.95 + 0.05*math.Sin(2*math.Pi*t/240)
}

// hashUnit maps a hash to [0, 1).
func hashUnit(h uint64) float64 { return float64(h>>11) / (1 << 53) }

func fract(x float64) float64 { return x - math.Floor(x) }

// menuMote is dust speck i at tick t: drifts right and up, wraps, and
// fades out before the button rail.
func menuMote(i int, t float64, sw, sh int) (x, y, r float32, alpha float64) {
	h := mix64(uint64(i)*0x9E3779B97F4A7C15 + 0x4D)
	bx, by, f := hashUnit(h), hashUnit(mix64(h)), hashUnit(mix64(h^0xA5A5))
	vx := (0.4 + 0.6*f) / 7200
	vy := (0.2 + 0.4*f) / 7200
	fx := fract(bx + t*vx)
	fy := fract(by - t*vy)
	x = float32(fx * float64(sw))
	y = float32(fy*float64(sh) + 6*math.Sin(t/90+f*6))
	r = float32(1 + 1.5*f)
	alpha = 0.10 + 0.14*f
	rail := float64(menuRailX(sw))
	alpha *= clamp01((rail - 20 - float64(x)) / 160)
	return x, y, r, alpha
}

func smooth(e0, e1, v float64) float64 {
	k := clamp01((v - e0) / (e1 - e0))
	return k * k * (3 - 2*k)
}

// menuScrimAlpha darkens where the menu chrome sits: the title band, the
// button rail, the bust at the bottom-left, and the corners.
func menuScrimAlpha(x, y float64, sw, sh int) float64 {
	w, h := float64(sw), float64(sh)
	top := 0.55 * (1 - smooth(0, 140, y))
	rail := 0.6 * smooth(w-500, float64(menuRailX(sw)), x)
	bust := 0.3 * (1 - smooth(0, w*0.45, x)) * smooth(h*0.3, h, y)
	dx, dy := (x-w/2)/(w/2), (y-h/2)/(h/2)
	vig := 0.35 * smooth(0.55, 1.45, math.Hypot(dx, dy))
	keep := (1 - top) * (1 - rail) * (1 - bust) * (1 - vig)
	return 1 - keep
}

// buildMenuScrim is menuScrimAlpha at 1/menuScrimDiv resolution, in ui.space.
func buildMenuScrim(sw, sh int) *image.NRGBA {
	w, h := max(1, sw/menuScrimDiv), max(1, sh/menuScrimDiv)
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := menuScrimAlpha((float64(x)+0.5)*menuScrimDiv, (float64(y)+0.5)*menuScrimDiv, sw, sh)
			img.SetNRGBA(x, y, color.NRGBA{uiSpace.R, uiSpace.G, uiSpace.B, uint8(a*255 + 0.5)})
		}
	}
	return img
}

// menuParticles is how many embers per fire and dust motes the menu
// draws under the Options effects and motion settings.
func menuParticles(s config.Settings) (embers, motes int) {
	if s.Accessibility.ReducedMotion {
		return 0, 0
	}
	fx := s.Graphics.FX()
	return int(math.Round(menuEmbers * fx)), int(math.Round(menuMotes * fx))
}

func (a *App) loadMenuScene() {
	if a.menuTried || a.cfg.LoadMenuScene == nil {
		return
	}
	a.menuTried = true
	if d, err := a.cfg.LoadMenuScene(); err == nil {
		a.menu = d
	}
}

// drawMenuDiorama draws the diorama dimmed by dim (0..1); false when
// there is none and the caller should draw the gradient backdrop.
func (a *App) drawMenuDiorama(screen *ebiten.Image, sw, sh int, dim float64) bool {
	d := a.menu
	if d == nil || len(d.levels) == 0 || sw < 1 || sh < 1 {
		return false
	}
	t := float64(a.menuTick)
	embers, motes := menuParticles(a.settings)
	view := t
	if a.settings.Accessibility.ReducedMotion { // still frame; the fire still burns
		view = 0
	}
	s, ox, oy := menuView(d.w, d.h, sw, sh, view)
	lv := 0
	for lv+1 < len(d.levels) && s*float64(int(1)<<(lv+1)) <= 1 {
		lv++
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(s*float64(int(1)<<lv), s*float64(int(1)<<lv))
	op.GeoM.Translate(ox, oy)
	screen.DrawImage(d.levels[lv], op)
	glow := func(img *ebiten.Image, k float64) {
		if img == nil {
			return
		}
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, Blend: ebiten.BlendLighter}
		f := float64(d.w) / float64(img.Bounds().Dx())
		op.GeoM.Scale(s*f, s*f)
		op.GeoM.Translate(ox, oy)
		op.ColorScale.ScaleAlpha(float32(k))
		screen.DrawImage(img, op)
	}
	glow(d.lamp, menuLampBreath(t))
	glow(d.fire, menuFlicker(t, 0))
	tile := d.tilePx * s
	for i, p := range d.fires {
		sx, sy := float32(ox+p[0]*s), float32(oy+p[1]*s)
		drawFlameScreen(screen, sx, sy, t+float64(i)*17, tile, 1)
		for e := 0; e < embers; e++ {
			h := mix64(uint64(i*menuEmbers+e) + 0x51)
			ph := fract(t/150 + hashUnit(h))
			ex := sx + float32(tile*((hashUnit(mix64(h))-0.5)*0.3+0.08*math.Sin(t/40+float64(e))))
			ey := sy - float32(tile*(0.25+1.3*ph))
			vector.FillCircle(screen, ex, ey, float32(math.Max(1, tile*0.022)), fade(color.RGBA{255, 170, 80, 255}, 0.8*(1-ph)), true)
		}
	}
	for i := 0; i < motes; i++ {
		x, y, r, al := menuMote(i, t, sw, sh)
		c := uiAccent
		if i%3 == 0 {
			c = uiDust
		}
		vector.FillCircle(screen, x, y, r, fade(c, al), true)
	}
	if d.scrim == nil || d.scrimW != sw || d.scrimH != sh {
		d.scrim = ebiten.NewImageFromImage(buildMenuScrim(sw, sh))
		d.scrimW, d.scrimH = sw, sh
	}
	drawImage(screen, d.scrim, 0, 0, float64(sw), float64(sh), false)
	if dim > 0 {
		vector.FillRect(screen, 0, 0, float32(sw), float32(sh), fade(uiSpace, dim), false)
	}
	return true
}
