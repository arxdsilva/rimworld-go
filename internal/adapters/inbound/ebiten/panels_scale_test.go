package ebiten

import (
	"fmt"
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	inspectorpkg "rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// The inspector family and its small panels draw through the canvas
// (PLAY-012 B.4): at s = 1.5 every logical rect lands scaled in physical
// pixels, and a logical button centre is hit by the logical cursor.

func TestPanelPrimitivesScaleAt1_5(t *testing.T) {
	withUIScale(t, 1.5)
	got := traceCanvas(t)
	img := ebiten.NewImage(800, 600)

	inspectorpkg.DrawFracBar(img, 10, 20, 100, 8, 0.5)
	if len(*got) != 2 || (*got)[0] != "fill 15,30 150x12" || (*got)[1] != "fill 15,30 75x12" {
		t.Fatalf("frame bar drew %q", *got)
	}

	*got = nil
	windows.DrawGoodwillChart(img, gfx.Rect{X: 100, Y: 100, W: 200, H: 100}, []int{-20, 10, 40}, color.RGBA{A: 255})
	if len(*got) == 0 || !strings.HasPrefix((*got)[0], "fill 150,150 300x150") {
		t.Fatalf("goodwill chart drew %q", *got)
	}

	*got = nil
	inspectorpkg.DrawAnimalIcon(img, &art.Catalog{}, game.AnimalView{Species: "nosuch"}, 10, 10, 20)
}

func TestTroughAndFloatersAtScale(t *testing.T) {
	withUIScale(t, 1.5)
	got := traceCanvas(t)
	img := ebiten.NewImage(800, 600)
	r := New(&stubSim{w: 4, h: 4})
	r.floaters = []creature.Floater{{X: 1, Y: 1, Text: "-12", Clr: color.RGBA{R: 200, A: 255}, Born: 0}}
	r.present = 1
	r.drawFloaters(img, 48)
	if len(*got) != 1 || !strings.HasPrefix((*got)[0], "fill ") {
		t.Fatalf("floater backing drew %q", *got)
	}
	sx, sy := r.worldToScreenF(1, 1-float64(1)/creature.FloaterLife*0.7)
	var x, y, w, h float32
	if _, err := fmt.Sscanf((*got)[0], "fill %g,%g %gx%g", &x, &y, &w, &h); err != nil {
		t.Fatal(err)
	}
	// the world anchor (physical) is converted to logical, then scaled back.
	if cx := x + w/2; cx < sx-1 || cx > sx+1 {
		t.Errorf("floater centre x %v, world anchor %v", cx, sx)
	}
	if cy := y + h/2; cy < sy-1 || cy > sy+1 {
		t.Errorf("floater centre y %v, world anchor %v", cy, sy)
	}
}

func TestEmblemFallbackAtScale(t *testing.T) {
	withUIScale(t, 1.5)
	a := testApp(&stubSim{w: 4, h: 4})
	a.art = &art.Catalog{}
	img := ebiten.NewImage(200, 200)
	a.wiz.DrawEmblem(img, "nosuch", color.NRGBA{R: 200, A: 255}, 10, 10, 24) // plain dot, no panic
}

func TestRoomCardAtScale(t *testing.T) {
	withUIScale(t, 1.5)
	if lines := hud.RoomCardLines(game.RoomView{Role: "bedroom", Score: 12}); len(lines) == 0 {
		t.Fatal("no card lines")
	}
}
