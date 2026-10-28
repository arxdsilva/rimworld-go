package world

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// Source is what the pipeline reads of the simulation and the renderer
// that no layer owns (SOLID_GO.md §6, §7). The renderer's adapter is a
// struct holding one pointer, so it is stored once and never allocates.
// Every method takes and returns values: a pointer to the frame's world
// passed through an interface call would move it to the heap.
type Source interface {
	// World is the simulation's world read model of this frame.
	World() game.WorldView
	// Snapshot is this frame's pawns.
	Snapshot() []game.PawnView
	// Date is the calendar date (the ground cover's season).
	Date() clock.Date
	// ChunkLayer is the read model's chunk (cx, cy) (game.ChunkSize tiles).
	ChunkLayer(cx, cy int) game.ChunkView
	// PlantsIn appends the indexes into plants of the plants on tiles
	// x0..x1 × y0..y1 from the simulation's chunk index; ok is false when
	// plants is not the simulation's current list (or there is no
	// simulation), and the pipeline scans plants itself.
	PlantsIn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) (out []int32, ok bool)
	// DrawOrderRoute draws the selected pawn's queued orders over the
	// objects (the orders feature's pass).
	DrawOrderRoute(screen *ebiten.Image, tile float64)
}

// Layers are the draw layers the pipeline composes, all built and owned
// by the renderer: the light, the environment, the creatures, the
// overlays, the sound cues, the shadow pass and the creature motion.
type Layers struct {
	Lit       *light.Lit
	Env       *env.Layer
	Creatures *creature.Set
	Overlays  *overlay.Set
	Cues      *cue.Bus
	Shadows   *light.Shadows[creature.FigurePose]
	Motion    *motion.State[creature.Body]
}

// Hosts are the layers' own host interfaces the renderer implements and
// the pipeline passes on (render/env deps.go, light.Casters): stored once,
// so passing them allocates nothing.
type Hosts struct {
	Index   env.Chunks
	Stamp   env.Stamp
	Movers  env.Movers
	Casters light.Casters[creature.FigurePose]
}

// Shared is the renderer state the pipeline works on that root code not
// yet moved still reads (ARCH-004: it moves here at the sweep): the
// camera a bake moves, the shadow pass's cast and a figure's blood (the
// sprite pass), the zoom tier with its bakes and chunk lists, the link
// grid and the stone cache.
type Shared struct {
	Cam   *camera.Camera
	Cast  **light.Cast
	Blood *float32
	LOD   *LOD
	Links *overlay.Links
	Stone *StoneArt
}

// base is what the pipeline's parts share: the layers, hosts and shared
// state, the painter, the frame's view and the source, the reused object
// queue and each vent's last cued burst.
type base struct {
	Layers
	Hosts
	Shared
	Paint sprite.Painter
	v     *render.View
	src   Source

	draws      []ArtDraw         // the object queue, reused every frame
	steamHeard map[[2]int]uint64 // each vent's last burst already cued

	// Bk, Ob and Lb are the pipeline's parts: the zoom tiers and bakes,
	// the object pass and the labels and marks.
	Bk Bakes
	Ob Objects
	Lb Labels
}

// Pipeline draws the world of one frame in its fixed order (Draw), over
// the parts: Bakes (the zoom tiers, the chunk and whole-map bakes, the
// frame's chunk lists), Objects (the object switch and object draws) and
// Labels (stack counts, bubbles, markers, damage, the vents' puffs).
type Pipeline struct{ *base }

// Bakes is the zoom tiers: the frame's chunk lists, the chunk and
// whole-map bakes and the baked ground drawn lit.
type Bakes struct{ *base }

// Objects is the object pass: the queue's switch and every object draw.
type Objects struct{ *base }

// Labels is what floats over the world: stack counts, chat bubbles,
// status markers, damage bars and the steam vents' puffs.
type Labels struct{ *base }

// New builds the pipeline over the renderer's layers, hosts and shared
// state, reading v each frame and src for what no layer owns.
func New(l Layers, h Hosts, s Shared, v *render.View, src Source) *Pipeline {
	b := &base{Layers: l, Hosts: h, Shared: s, v: v, src: src}
	b.Paint = sprite.Painter{Lit: l.Lit, Stone: &s.Stone.StoneCache}
	b.Bk, b.Ob, b.Lb = Bakes{b}, Objects{b}, Labels{b}
	return &Pipeline{b}
}

// SetArt points the painter at the renderer's current catalog (the App
// sets it after the renderer is built; tests swap it).
func (p *Pipeline) SetArt(c *art.Catalog) { p.Paint.Art = c }
