// Package appicon draws the rimworld-go app icon procedurally (stdlib only)
// and writes every size the packaging and the game need: packaging PNGs, a
// Windows .ico, an .iconset folder for macOS iconutil, and the runtime window
// icons under assets/generated/ui/app_icon. The art is original: a limbless
// colonist (round head, body capsule, like the in-game pawns) in the UI gold
// accent on a charcoal rounded square above a dark-green hill. Colours come
// from assets/palettes/ui.yaml (ui.space, ui.horizon, ui.accent). No outline
// on the hill: it is terrain, not a contoured plant.
//
// Regenerate with `make app-icon` (cmd/iconpack plus iconutil for the .icns).
package appicon

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// Write renders every size into the packaging dir (PNGs, .ico, .iconset) and
// the runtime window-icon dir.
func Write(pack, rt string) error {
	for _, d := range []string{pack, rt, filepath.Join(pack, "rimworld-go.iconset")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	cache := map[int]*image.RGBA{}
	get := func(n int) *image.RGBA {
		if cache[n] == nil {
			cache[n] = Render(n)
		}
		return cache[n]
	}
	for _, n := range []int{48, 128, 256, 512, 1024} {
		if err := writePNG(filepath.Join(pack, fmt.Sprintf("rimworld-go-%d.png", n)), get(n)); err != nil {
			return err
		}
	}
	for _, n := range []int{16, 32, 48, 256} {
		if err := writePNG(filepath.Join(rt, fmt.Sprintf("icon_%d.png", n)), get(n)); err != nil {
			return err
		}
	}
	for _, n := range []int{16, 32, 128, 256, 512} {
		if err := writePNG(filepath.Join(pack, "rimworld-go.iconset", fmt.Sprintf("icon_%dx%d.png", n, n)), get(n)); err != nil {
			return err
		}
		if err := writePNG(filepath.Join(pack, "rimworld-go.iconset", fmt.Sprintf("icon_%dx%d@2x.png", n, n)), get(2*n)); err != nil {
			return err
		}
	}
	var imgs []*image.RGBA
	for _, n := range []int{16, 24, 32, 48, 64, 128, 256} {
		imgs = append(imgs, get(n))
	}
	ico, err := EncodeICO(imgs)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(pack, "rimworld-go.ico"), ico, 0o644)
}

func writePNG(path string, img image.Image) error {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// EncodeICO packs square images (each at most 256 px) as PNG-compressed
// entries of a Windows icon file.
func EncodeICO(imgs []*image.RGBA) ([]byte, error) {
	var head, data bytes.Buffer
	binary.Write(&head, binary.LittleEndian, [3]uint16{0, 1, uint16(len(imgs))})
	offset := 6 + 16*len(imgs)
	for _, im := range imgs {
		var p bytes.Buffer
		if err := png.Encode(&p, im); err != nil {
			return nil, err
		}
		n := im.Bounds().Dx()
		if n > 256 {
			return nil, fmt.Errorf("ico entry %d px exceeds 256", n)
		}
		w := byte(n) // 256 is stored as 0
		head.Write([]byte{w, w, 0, 0})
		binary.Write(&head, binary.LittleEndian, [2]uint16{1, 32})
		binary.Write(&head, binary.LittleEndian, [2]uint32{uint32(p.Len()), uint32(offset)})
		offset += p.Len()
		data.Write(p.Bytes())
	}
	return append(head.Bytes(), data.Bytes()...), nil
}

type rgb struct{ r, g, b float64 }

func hex(r, g, b uint8) rgb { return rgb{float64(r), float64(g), float64(b)} }

func mix(a, b rgb, t float64) rgb {
	return rgb{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

var (
	skyTop = hex(0x3A, 0x3E, 0x45) // lighter than ui.horizon for a soft sky
	skyBot = hex(0x24, 0x26, 0x2A) // ui.horizon
	hill   = hex(0x1F, 0x3A, 0x22) // dark green, darker than ui.good
	gold   = hex(0xD9, 0xB4, 0x5B) // ui.accent
	goldLo = hex(0xB8, 0x92, 0x3E) // shaded gold for the body
)

func sdCircle(x, y, cx, cy, r float64) float64 { return math.Hypot(x-cx, y-cy) - r }

func sdCapsule(x, y, ax, ay, bx, by, r float64) float64 {
	px, py := x-ax, y-ay
	dx, dy := bx-ax, by-ay
	h := math.Max(0, math.Min(1, (px*dx+py*dy)/(dx*dx+dy*dy)))
	return math.Hypot(px-dx*h, py-dy*h) - r
}

func sdRoundRect(x, y, m, r float64) float64 {
	c := 0.5
	hx := c - m - r
	qx, qy := math.Abs(x-c)-hx, math.Abs(y-c)-hx
	return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - r
}

// shade returns the colour and coverage of the icon at unit coordinates.
func shade(x, y, px float64) (rgb, float64) {
	cov := clamp(0.5 - sdRoundRect(x, y, 0.04, 0.22)/px)
	if cov == 0 {
		return rgb{}, 0
	}
	c := mix(skyTop, skyBot, clamp(y/0.8))
	c = mix(c, hill, clamp(0.5-sdCircle(x, y, 0.5, 1.38, 0.72)/px))
	body := sdCapsule(x, y, 0.5, 0.58, 0.5, 0.70, 0.17)
	head := sdCircle(x, y, 0.5, 0.33, 0.155)
	c = mix(c, mix(gold, goldLo, clamp((y-0.55)/0.3)), clamp(0.5-body/px))
	c = mix(c, gold, clamp(0.5-head/px))
	return c, cov
}

func clamp(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// Render draws the icon at n x n pixels (4x4 supersampled).
func Render(n int) *image.RGBA {
	const ss = 4
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	px := 1 / float64(n)
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			var r, g, b, a float64
			for sj := 0; sj < ss; sj++ {
				for si := 0; si < ss; si++ {
					x := (float64(i) + (float64(si)+0.5)/ss) * px
					y := (float64(j) + (float64(sj)+0.5)/ss) * px
					c, cov := shade(x, y, px)
					r, g, b, a = r+c.r*cov, g+c.g*cov, b+c.b*cov, a+cov
				}
			}
			if a == 0 {
				continue
			}
			img.SetRGBA(i, j, color.RGBA{uint8(r/a + 0.5), uint8(g/a + 0.5), uint8(b/a + 0.5), uint8(a/(ss*ss)*255 + 0.5)})
		}
	}
	return img
}
