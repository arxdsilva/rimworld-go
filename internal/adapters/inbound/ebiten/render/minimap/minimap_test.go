package minimap

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

type fakeSrc struct{ w, h int }

func (f fakeSrc) Width() int                { return f.w }
func (f fakeSrc) Height() int               { return f.h }
func (f fakeSrc) World() game.WorldView     { return game.WorldView{} }
func (f fakeSrc) Snapshot() []game.PawnView { return nil }
func (f fakeSrc) MinimapLayer(uint64) (game.MinimapView, bool) {
	return game.MinimapView{}, false
}

func TestOnToggleAndColors(t *testing.T) {
	t.Parallel()
	m := New()
	if !m.On(fakeSrc{200, 200}) || m.On(fakeSrc{150, 150}) {
		t.Fatal("only maps over 150 tiles get a minimap")
	}
	m.Toggle()
	if m.On(fakeSrc{200, 200}) {
		t.Fatal("a hidden minimap is on")
	}
	var nilMap *Map
	nilMap.Toggle()
	if nilMap.On(fakeSrc{200, 200}) {
		t.Fatal("a nil minimap is on")
	}
	if Color(game.MinimapStructure|1) != StructureClr {
		t.Fatal("structures take the structure colour")
	}
}

func TestClickReturnsCentringOffsets(t *testing.T) {
	t.Parallel()
	ui := camera.ColonyViewport(1280, 800)
	m, src := New(), fakeSrc{200, 200}
	rv := &render.View{Cam: camera.Camera{Zoom: 1}}
	bx, by, mx, my, mw, mh := m.Box(src, ui)
	ok, ox, oy := m.Click(rv, src, int(mx+mw/2), int(my+mh/2), ui, ui)
	tile := rv.Cam.Tile()
	if d := (ox+(ui.X0+ui.X1)/2)/tile - 100; !ok || d < -1.5 || d > 1.5 {
		t.Fatalf("click ok=%v centres tile %v, want 100", ok, d+100)
	}
	_ = oy
	if ok, _, _ := m.Click(rv, src, int(bx)-5, int(by)+10, ui, ui); ok {
		t.Fatal("a click beside the box was taken")
	}
}
