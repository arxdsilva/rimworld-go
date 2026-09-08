package ebiten

import (
	"fmt"
	"image"
	"image/png"
	"os"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/domain/sim/entity"
)

const (
	pawnMaxPx = 128
	tileMaxPx = 64
)

type spriteSet struct {
	idle []*ebiten.Image
	walk []*ebiten.Image
}

type atlas struct {
	colonists []*spriteSet
	ground    []*ebiten.Image
	rocks     []*ebiten.Image
	crosshair *ebiten.Image
}

func loadAtlas(p Pack) (*atlas, error) {
	a := &atlas{}
	for _, ch := range p.Colonists {
		ss, err := loadCharacter(ch)
		if err != nil {
			return nil, err
		}
		a.colonists = append(a.colonists, ss)
	}
	for _, path := range p.Ground {
		img, err := loadSprite(path, tileMaxPx)
		if err != nil {
			return nil, err
		}
		a.ground = append(a.ground, img)
	}
	for _, path := range p.Rocks {
		img, err := loadSprite(path, tileMaxPx)
		if err != nil {
			return nil, err
		}
		a.rocks = append(a.rocks, img)
	}
	if p.Crosshair != "" {
		img, err := loadSprite(p.Crosshair, tileMaxPx)
		if err != nil {
			return nil, err
		}
		a.crosshair = img
	}
	if len(a.colonists) == 0 {
		return nil, fmt.Errorf("sprite pack has no colonist characters")
	}
	return a, nil
}

func loadCharacter(ch Character) (*spriteSet, error) {
	ss := &spriteSet{}
	for _, path := range ch.Anims["idle"] {
		img, err := loadSprite(path, pawnMaxPx)
		if err != nil {
			return nil, err
		}
		ss.idle = append(ss.idle, img)
	}
	for _, path := range ch.Anims["walk"] {
		img, err := loadSprite(path, pawnMaxPx)
		if err != nil {
			return nil, err
		}
		ss.walk = append(ss.walk, img)
	}
	if len(ss.idle) == 0 {
		return nil, fmt.Errorf("character %s has no idle frames", ch.ID)
	}
	return ss, nil
}

func loadSprite(path string, maxDim int) (*ebiten.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	cropped := cropAlpha(src)
	scaled := scaleNRGBA(cropped, maxDim)
	return ebiten.NewImageFromImage(scaled), nil
}

func cropAlpha(m image.Image) *image.NRGBA {
	b := alphaBounds(m)
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.Set(x, y, m.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func alphaBounds(m image.Image) image.Rectangle {
	b := m.Bounds()
	minX, minY := b.Max.X, b.Max.Y
	maxX, maxY := b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := m.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if maxX < minX {
		return b
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

func scaleNRGBA(src *image.NRGBA, maxDim int) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	if w < 1 || h < 1 {
		return src
	}
	if w <= maxDim && h <= maxDim {
		return src
	}
	s := float64(maxDim) / float64(w)
	if h > w {
		s = float64(maxDim) / float64(h)
	}
	nw := int(float64(w)*s + 0.5)
	nh := int(float64(h)*s + 0.5)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy := y * h / nh
		for x := 0; x < nw; x++ {
			sx := x * w / nw
			dst.SetNRGBA(x, y, src.NRGBAAt(src.Bounds().Min.X+sx, src.Bounds().Min.Y+sy))
		}
	}
	return dst
}

func (a *atlas) colonist(id entity.EntityID) *spriteSet {
	if a == nil || len(a.colonists) == 0 {
		return nil
	}
	i := int((uint64(id) - 1) % uint64(len(a.colonists)))
	return a.colonists[i]
}

func tileHash(x, y int) uint64 {
	return uint64(uint32(x))*0x9E3779B97F4A7C15 ^ uint64(uint32(y))*0xBF58476D1CE4E5B9
}

func (a *atlas) groundAt(x, y int) *ebiten.Image {
	if a == nil || len(a.ground) == 0 {
		return nil
	}
	return a.ground[int(tileHash(x, y)%uint64(len(a.ground)))]
}

func (a *atlas) rockAt(x, y int) *ebiten.Image {
	if a == nil || len(a.rocks) == 0 {
		return nil
	}
	h := tileHash(x, y)
	if h%7 != 0 {
		return nil
	}
	return a.rocks[int(h%uint64(len(a.rocks)))]
}

func walkHold(present, lastMove uint64, sustain uint64) bool {
	return lastMove > 0 && present-lastMove < sustain
}

func animFrame(frames []*ebiten.Image, tick uint64, period uint64) *ebiten.Image {
	if len(frames) == 0 {
		return nil
	}
	if period < 1 {
		period = 1
	}
	return frames[int((tick/period)%uint64(len(frames)))]
}

func drawImage(dst, src *ebiten.Image, x, y, w, h float64, flipX bool) {
	if src == nil {
		return
	}
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw < 1 || sh < 1 {
		return
	}
	op := &ebiten.DrawImageOptions{}
	sx := w / float64(sw)
	sy := h / float64(sh)
	if flipX {
		op.GeoM.Scale(-sx, sy)
		op.GeoM.Translate(w, 0)
	} else {
		op.GeoM.Scale(sx, sy)
	}
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(src, op)
}
