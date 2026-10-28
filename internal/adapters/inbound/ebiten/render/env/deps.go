package env

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// What the layer needs from the renderer, each consumer-owned and small
// (SOLID_GO.md §6, §10). The renderer's adapter is a struct holding one
// pointer, so passing it as one of these allocates nothing.

// Chunks is the renderer's chunk index over the simulation's read model:
// the covers the grass mask hides, the floors and crops puddles avoid, the
// trees a creature brushes.
type Chunks interface {
	// ChunkView is the read model's chunk (cx, cy) (game.ChunkSize tiles).
	ChunkView(cx, cy int) game.ChunkView
	// PlantsOn appends the indexes into plants of the plants on tiles
	// x0..x1 × y0..y1.
	PlantsOn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) []int32
	// FrameLists reports whether l is the world this frame's chunk lists
	// index (the same shared lists).
	FrameLists(l Lists) bool
	// Current reports whether buildings is the simulation's current list
	// (the chunk read model indexes it).
	Current(buildings []game.BuildingView) bool
}

// Lists identifies a world by its object lists: the host tells the frame's
// world from any other by their backing arrays. The methods above take
// these slices by value instead of the *game.WorldView because a pointer
// passed through an interface call escapes to the heap, and the renderer's
// world is a local of its draw frame (SOLID_GO.md §7: no interface values
// in the hot paths).
type Lists struct {
	Buildings []game.BuildingView
	Plants    []game.PlantView
	Items     []game.ItemView
	Frames    []game.FrameView
}

// ListsOf is w's identity for Chunks.FrameLists.
func ListsOf(w *game.WorldView) Lists {
	return Lists{Buildings: w.Buildings, Plants: w.Plants, Items: w.Items, Frames: w.Frames}
}

// Stamp draws what the renderer's ground pipeline owns: a baked mask lit
// per tile corner or as one flat quad (lod.go), a terrain tile, a posed
// sprite.
type Stamp interface {
	DrawBakedColor(screen, src *ebiten.Image, bx, by int, sx0, sy0, bpx float64, tx0, ty0, tx1, ty1 int, col [4]float32)
	DrawChunkFlat(screen, src *ebiten.Image, srcX, srcY, dstX, dstY, span, srcSpan float64, col [4]float32)
	DrawTile(screen *ebiten.Image, key string, wx, wy, tile float64, sw, sh int, cs sprite.ColorScale)
	DrawSpritePose(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, ps motion.Pose)
}

// Movers is the creature motion the contact springs read: whether a
// creature is near the last frame's view, and its interpolated motion.
type Movers interface {
	InView(x, y, pad, padS int) bool
	PawnMotion(id entity.EntityID) motion.MoveState
	AnimalMotion(id entity.EntityID) motion.MoveState
}
