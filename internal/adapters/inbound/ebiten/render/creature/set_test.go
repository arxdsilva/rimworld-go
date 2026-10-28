package creature

import (
	"image/color"
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeHost is a Host over fixed answers: no figure art, no blood, a
// facing per creature, and the cues and sprays it was asked for.
type fakeHost struct {
	visuals map[entity.EntityID]Visual
	cues    []string
}

func (h *fakeHost) FigureLayers(dst []Layer, p game.PawnView, fs string) []Layer {
	return append(dst, Layer{Key: "pawn/body_" + fs}, Layer{Key: "pawn/hair_" + fs})
}
func (h *fakeHost) PawnBlood(game.PawnView) float32          { return 0 }
func (h *fakeHost) AnimalBlood(game.AnimalView) float32      { return 0 }
func (h *fakeHost) LayerBlood(_ string, b float32) float32   { return b }
func (h *fakeHost) Visual(id entity.EntityID) Visual         { return h.visuals[id] }
func (h *fakeHost) ColonyCloths() []string                   { return nil }
func (h *fakeHost) ItemSprite(kind string) string            { return "item/" + kind }
func (h *fakeHost) HealthColor(int) color.RGBA               { return color.RGBA{A: 255} }
func (h *fakeHost) TurretMuzzle(entity.EntityID) (Vec, bool) { return Vec{}, false }
func (h *fakeHost) BurstCue(x, y float64)                    { h.cues = append(h.cues, "burst") }
func (h *fakeHost) TraceSound(game.TraceView)                { h.cues = append(h.cues, "trace") }
func (h *fakeHost) WorkCue(act string, _, _ float64)         { h.cues = append(h.cues, "work:"+act) }
func (h *fakeHost) HostileMarker(*ebiten.Image, game.PawnView, float64, float64, float64, float64) {
}
func (h *fakeHost) StoneItem(key, _ string, cs sprite.ColorScale) (string, sprite.ColorScale) {
	return key, cs
}
func (h *fakeHost) BuildingAt(game.WorldView, int, int) (game.BuildingView, bool) {
	return game.BuildingView{}, false
}
func (h *fakeHost) Spray(game.WorldView, entity.EntityID, int) (PartKind, int, float64) {
	return PartBlood, 4, BloodSpeed
}

// testSet is a creature layer over a fake host, its own motion state and
// light, with the view at zoom 1 (64 px tiles).
func testSet() (*Set, *render.View, *fakeHost) {
	h := &fakeHost{visuals: map[entity.EntityID]Visual{}}
	c := New(&sprite.Painter{}, &motion.State[Body]{}, &light.Lit{})
	c.Host = h
	v := &render.View{Cam: camera.Camera{Zoom: 1}, Present: 100, Fx: 1}
	return c, v, h
}

// markTrace is one mark the overlays drew (render.Trace.AnimalMark).
type markTrace struct {
	kind       string
	a, b, c, d float32
}

// traceMarks records the marks drawn through v.
func traceMarks(v *render.View) *[]markTrace {
	var got []markTrace
	v.Trace = &render.Trace{AnimalMark: func(kind string, a, b, c, d float32) {
		got = append(got, markTrace{kind, a, b, c, d})
	}}
	return &got
}

func TestChaseLineFromTheChaserToItsPawn(t *testing.T) {
	t.Parallel()
	c, v, _ := testSet()
	v.Pawns = []game.PawnView{{ID: 1, X: 6, Y: 5}}
	w := game.WorldView{Animals: []game.AnimalView{
		{ID: 10, X: 9, Y: 5, Hostile: true, TargetID: 1},
		{ID: 11, X: 3, Y: 8, Hostile: true},
	}}
	marks := traceMarks(v)
	cull := Cull{}
	c.DrawChaseLines(v, &cull, ebiten.NewImage(64, 64), w, v.Cam.Tile())
	if len(*marks) != 1 || (*marks)[0].kind != "chase-line" {
		t.Fatalf("marks %+v, want one chase line", *marks)
	}
	x0, y0 := v.WorldToScreenF(9.5, 5.5)
	x1, y1 := v.WorldToScreenF(6.5, 5.5)
	if m := (*marks)[0]; m.a != x0 || m.b != y0 || m.c != x1 || m.d != y1 {
		t.Fatalf("line %+v, want (%v,%v) -> (%v,%v)", m, x0, y0, x1, y1)
	}
	// both ends far outside the view: culled
	*marks = nil
	cull = Cull{ViewSet: true, X0: 40, Y0: 40, X1: 60, Y1: 60}
	c.DrawChaseLines(v, &cull, ebiten.NewImage(64, 64), w, v.Cam.Tile())
	if len(*marks) != 0 {
		t.Fatalf("off screen: %+v", *marks)
	}
}

func TestThreatPauseCentresBetweenTheChase(t *testing.T) {
	t.Parallel()
	c, v, _ := testSet()
	pawns := []game.PawnView{{ID: 1, X: 40, Y: 5}}
	w := game.WorldView{
		Animals:  []game.AnimalView{{ID: 10, X: 50, Y: 5, TargetID: 1}},
		Messages: []game.MessageView{{Danger: true, Tick: 7, Focus: 10}},
	}
	vp := camera.Viewport{X1: 640, Y1: 480}
	x, y, ok := c.OnThreatPause(v, w, pawns, 7, vp)
	if !ok || x != 45.5 || y != 5.5 {
		t.Fatalf("centre (%v, %v, %v), want (45.5, 5.5)", x, y, ok)
	}
	if _, _, ok := c.OnThreatPause(v, w, pawns, 8, vp); ok {
		t.Fatal("another tick's danger moved the camera")
	}
	if _, _, ok := c.OnThreatPause(v, w, pawns, 7, camera.Viewport{}); ok {
		t.Fatal("no viewport: the camera stays")
	}
	near := []game.PawnView{{ID: 1, X: 2, Y: 2}}
	w.Animals[0].X, w.Animals[0].Y = 3, 3
	if _, _, ok := c.OnThreatPause(v, w, near, 7, vp); ok {
		t.Fatal("an on-screen chase moved the camera")
	}
}

// The body player never flaps (AGENTS.md hysteresis): a view flipping
// between grazing, standing and sleeping every few ticks changes the
// drawn frame at most once per BodyMinHold, and lies down only from
// standing.
func TestBodyPlayerDoesNotFlap(t *testing.T) {
	t.Parallel()
	frame := func(b *Body) string {
		s := b.show()
		if s.Rest {
			return "rest"
		}
		return s.Suffix
	}
	for _, flip := range []int{2, 3, 5, 7} {
		var b Body
		an := game.AnimalView{ID: 4}
		last, since, changes := "?", math.Inf(-1), 0
		for tick := 0; tick < 900; tick++ {
			an.Action, an.Sleeping = "", false
			switch tick / flip % 3 {
			case 0:
				an.Action = game.ActionGraze
			case 1:
				an.Sleeping = true
			}
			want, t0 := bodyWant(an, motion.MoveState{}), float64(tick)
			if !b.Ok {
				b.start(an.ID, want, motion.MoveState{}, t0)
			} else {
				b.advance(an.ID, want, motion.MoveState{}, GaitFallback, t0)
			}
			f := frame(&b)
			if f == last {
				continue
			}
			if last != "?" {
				changes++
				if changes > 1 && t0-since < BodyMinHold {
					t.Fatalf("flip %d: frame %q -> %q after %.0f ticks", flip, last, f, t0-since)
				}
				if f == "rest" && last != "" {
					t.Fatalf("flip %d: lay down from %q", flip, last)
				}
			}
			last, since = f, t0
		}
		if changes < 2 {
			t.Fatalf("flip %d: the pose never changed", flip)
		}
	}
}

func TestStaticShowFollowsTheView(t *testing.T) {
	t.Parallel()
	walk := motion.MoveState{Blend: 1, Stride: 0.4}
	if s := StaticShow(game.AnimalView{ID: 1}, walk, 0, GaitFallback); !s.Walk || s.Suffix == "" {
		t.Fatalf("walking: %+v", s)
	}
	if s := StaticShow(game.AnimalView{ID: 1, Sleeping: true}, motion.MoveState{}, 0, GaitFallback); !s.Rest {
		t.Fatalf("asleep: %+v", s)
	}
	seen := map[string]bool{}
	for tk := 0.0; tk < 1200; tk++ {
		seen[StaticShow(game.AnimalView{ID: 1, Action: game.ActionGraze}, motion.MoveState{}, tk, GaitFallback).Suffix] = true
	}
	if !seen["_graze1"] || !seen["_graze2"] || !seen[""] {
		t.Fatalf("a graze bout shows reach, down and look: %v", seen)
	}
}

func TestBurstsAreDeterministicAndAge(t *testing.T) {
	t.Parallel()
	c, v, h := testSet()
	evs := []BurstEvent{{ID: 4, X: 6, Y: 7, Tick: 90}}
	var tr BurstTracker
	tr.Emit(c, v, evs)
	n := len(*c.Parts)
	if n == 0 || len(h.cues) != 1 {
		t.Fatalf("a burst throws particles (%d) and bangs once (%v)", n, h.cues)
	}
	tr.Emit(c, v, evs)
	if len(*c.Parts) != n {
		t.Fatal("the same stamp bursts twice")
	}
	c2, v2, _ := testSet()
	var tr2 BurstTracker
	tr2.Emit(c2, v2, evs)
	for i := range *c2.Parts {
		if (*c2.Parts)[i] != (*c.Parts)[i] {
			t.Fatal("bursts are deterministic in the stamp")
		}
	}
	v.Present += 200
	c.AgeParticles(v)
	if len(*c.Parts) != 0 {
		t.Fatalf("%d particles outlived their life", len(*c.Parts))
	}
}

func TestEffectsDetailThinsBursts(t *testing.T) {
	t.Parallel()
	c, v, _ := testSet()
	c.Burst(v, PartDust, 1, 1, 10, 3)
	full := len(*c.Parts)
	c2, v2, _ := testSet()
	v2.Fx = 0.5
	c2.Burst(v2, PartDust, 1, 1, 10, 3)
	if half := len(*c2.Parts); half != 5 || full != 10 {
		t.Fatalf("detail 0.5 spawns %d of %d", half, full)
	}
	for len(*c.Parts) < MaxParticles {
		c.Burst(v, PartChip, 1, 1, 50, 9)
	}
	c.Burst(v, PartChip, 1, 1, 50, 9)
	if len(*c.Parts) != MaxParticles {
		t.Fatalf("the cap holds: %d", len(*c.Parts))
	}
}

func TestItemsPopOnlyAfterTheFirstSync(t *testing.T) {
	t.Parallel()
	c, v, _ := testSet()
	c.SyncItems(v, game.WorldView{Items: []game.ItemView{{ID: 1}}})
	if born := (*c.ItemBorn)[1]; born != 0 {
		t.Fatalf("an item there at the start pops: %d", born)
	}
	v.Present = 140
	c.SyncItems(v, game.WorldView{Items: []game.ItemView{{ID: 1}, {ID: 2}}})
	if (*c.ItemBorn)[2] != 140 || (*c.ItemBorn)[1] != 0 {
		t.Fatalf("a new drop pops from its frame: %v", *c.ItemBorn)
	}
	c.SyncItems(v, game.WorldView{Items: []game.ItemView{{ID: 2}}})
	if _, ok := (*c.ItemBorn)[1]; ok {
		t.Fatal("a gone item is forgotten")
	}
}

func TestTracesOutliveTheSimForMinFrames(t *testing.T) {
	t.Parallel()
	c, v, h := testSet()
	tr := game.TraceView{FromX: 1, FromY: 1, ToX: 4, ToY: 1, Hit: true, Damage: 6}
	w := game.WorldView{Traces: []game.TraceView{tr}}
	v.Tick = 50
	c.AnimateCombat(v, w, nil)
	if len(h.cues) != 1 || len(*c.Floaters) != 1 || len(*c.Parts) == 0 {
		t.Fatalf("a new hit sounds (%v), floats a number (%d) and sprays (%d)", h.cues, len(*c.Floaters), len(*c.Parts))
	}
	c.AnimateCombat(v, w, nil)
	if len(h.cues) != 1 || len(*c.Floaters) != 1 {
		t.Fatal("a seen trace spawns its effects once")
	}
	// the sim drops it: still listed, aged by frames
	v.Present += 3
	c.TrackReactions(nil, 51, v.Present)
	rs := c.TraceList(v, game.WorldView{})
	if len(rs) != 1 || !math.IsInf(rs[0].Sim, 1) || rs[0].Frames != 3 {
		t.Fatalf("tracked after the sim dropped it: %+v", rs)
	}
	v.Present += 3*minReactionFrames + 1
	c.TrackReactions(nil, 52, v.Present)
	if rs := c.Reactions(v.Present, nil); len(rs) != 0 {
		t.Fatalf("forgotten after its frames: %+v", rs)
	}
}

func TestWorkBarsOnePerTile(t *testing.T) {
	t.Parallel()
	c, v, _ := testSet()
	at := func(id entity.EntityID, k float64) game.PawnView {
		return game.PawnView{ID: id, X: 4, Y: 5, ActX: 5, ActY: 5, Action: game.ActionBuild, ActionProgress: k}
	}
	pawns := []game.PawnView{at(1, 0.3), at(2, 0.6), {ID: 3, Action: game.ActionEat, ActionProgress: 0.5}}
	cull := Cull{}
	bars := c.WorkBarRects(v, &cull, pawns, 64, nil)
	if len(bars) != 1 || bars[0].K != 0.6 {
		t.Fatalf("bars %+v, want one at 0.6", bars)
	}
	if bars := c.WorkBarRects(v, &cull, pawns, WorkBarMinTile-1, nil); len(bars) != 0 {
		t.Fatal("no bar zoomed out")
	}
}

func TestFigurePoseReadsTheVisual(t *testing.T) {
	t.Parallel()
	c, v, h := testSet()
	h.visuals[3] = Visual{Facing: art.FaceWest, LyingSince: 90}
	fp := c.FigurePoseOf(v, game.PawnView{ID: 3, Lying: true})
	if fp.Vis.Facing != art.FaceWest || fp.Vis.LyingSince != 90 {
		t.Fatalf("lying pose keeps the visual: %+v", fp.Vis)
	}
	if mark := c.mark(); mark != 0 {
		t.Fatalf("the layer stack leaked %d layers", mark)
	}
	rig := c.RigOf(v, game.PawnView{ID: 3}, art.FaceWest)
	if !rig.Mirror || rig.Face != art.FaceEast || len(rig.Layers) != 2 || c.mark() != 0 {
		t.Fatalf("rig %+v (stack %d)", rig, c.mark())
	}
}
