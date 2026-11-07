package world

import (
	"math"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// The Neat Storage furniture's art (NST, NEAT_STORAGE_GAP_ANALYSIS.md): its
// sprite keys, how each piece shows what it holds, and the index that finds
// the piece under a stored stack.

// neatHeapBodies is how many stacks a heap draws: twelve sprites on one tile
// overlap into noise, so the rest show only in the count label.
const neatHeapBodies = 4

// neatManySlots is the slot count from which a piece's stack counts show
// only at the closest zoom tier.
const neatManySlots = 6

// Stacked stored stacks sort after their piece and before the next row: a
// lower cell of the piece over an upper one, a later slot over an earlier
// one. Twelve slots stay below one cell's step.
const (
	neatRowCellStep = 0.0002
	neatRowSlotStep = 0.00001
)

// neatShow is how a piece shows what it holds.
type neatShow uint8

const (
	showFlat  neatShow = iota // every stack level at its slot
	showNone                  // closed: no stack, no count
	showHeap                  // the first few stacks, and one count for the tile
	showOnEnd                 // every stack stood on end at its slot
)

// neatOnEndRot turns a level sprite on end: its right side up (radians,
// clockwise on screen).
const neatOnEndRot = -math.Pi / 2

// neatPiece is one piece's look.
type neatPiece struct {
	name  string
	slots int // stacks per cell
	show  neatShow
	keys  [4]string // by neatFacing; the same key four times on a fixed piece
}

// hides reports a stack in this slot draws nothing: a closed piece shows no
// stack, a heap only its first few.
func (p *neatPiece) hides(slot uint8) bool {
	return p.show == showNone || (p.show == showHeap && slot >= neatHeapBodies)
}

// neatLooks are the 27 pieces in construction's order (NeatProfile), with
// the slots per cell of the doc's table.
var neatLooks = [...]struct {
	kind  construction.Kind
	slots int
	show  neatShow
}{
	{construction.DisplayCase, 1, showFlat},
	{construction.RoundPedestal, 1, showFlat},
	{construction.SmallDisplayShelf, 3, showFlat},
	{construction.DisplayShelf, 3, showFlat},
	{construction.WeaponRack, 3, showFlat},
	{construction.NarrowWardrobe, 4, showFlat},
	{construction.Wardrobe, 4, showFlat},
	{construction.NarrowDisplayCabinet, 4, showFlat},
	{construction.DisplayCabinet, 4, showFlat},
	{construction.OpenShelf, 6, showFlat},
	{construction.TwoTierDisplayShelf, 6, showFlat},
	{construction.NarrowTallShelf, 6, showFlat},
	{construction.TallShelf, 6, showFlat},
	{construction.Safe, 6, showNone},
	{construction.Drum, 6, showNone},
	{construction.HorizontalWeaponRack, 6, showFlat},
	{construction.VerticalWeaponRack, 6, showOnEnd},
	{construction.TextileRack, 6, showFlat},
	{construction.MeatHook, 6, showFlat},
	{construction.MiniPallet, 6, showFlat},
	{construction.HalfPallet, 6, showFlat},
	{construction.Pallet, 6, showFlat},
	{construction.SmallCrate, 6, showNone},
	{construction.Crate, 6, showNone},
	{construction.LargeCrate, 6, showNone},
	{construction.TallCrate, 6, showNone},
	{construction.CorpseDumpSpot, 12, showHeap},
}

// neatPieces are neatLooks with their names and keys, built once: the draw
// reads precomputed keys, so a stored stack allocates nothing.
var neatPieces = buildNeatPieces()

// neatByName finds a piece by its build name (a BuildingView's Kind).
var neatByName = buildNeatNames()

func buildNeatPieces() [len(neatLooks)]neatPiece {
	var out [len(neatLooks)]neatPiece
	for i, l := range neatLooks {
		d := construction.DefOf(l.kind)
		base := "furniture/" + strings.ReplaceAll(d.Name, " ", "_")
		p := neatPiece{name: d.Name, slots: l.slots, show: l.show}
		for f, name := range [...]string{"south", "east", "north", "west"} {
			p.keys[f] = base
			if d.Rotatable {
				p.keys[f] = base + "_" + name
			}
		}
		out[i] = p
	}
	return out
}

func buildNeatNames() map[string]*neatPiece {
	out := make(map[string]*neatPiece, len(neatPieces))
	for i := range neatPieces {
		out[neatPieces[i].name] = &neatPieces[i]
	}
	return out
}

// neatPieceOf is the piece an item's Store names; nil for loose stacks and
// for storage with no slots (a plain shelf, a hopper).
func neatPieceOf(store uint8) *neatPiece {
	k := construction.Kind(store)
	if !k.IsNeatStorage() {
		return nil
	}
	return &neatPieces[k-construction.DisplayCase]
}

// neatFacing is a facing's index in neatPiece.keys; south when unset.
func neatFacing(f string) uint8 {
	switch f {
	case "east":
		return 1
	case "north":
		return 2
	case "west":
		return 3
	}
	return 0
}

// NeatKey is a storage piece's world sprite: furniture/<slug>_<facing> when
// it rotates, furniture/<slug> when not; "" for any other kind.
func NeatKey(kind, facing string) string {
	p := neatByName[kind]
	if p == nil {
		return ""
	}
	return p.keys[neatFacing(facing)]
}

// neatCell is the piece standing on one tile.
type neatCell struct {
	piece *neatPiece
	x, y  int // the piece's top-left tile
	h     int // its height in tiles
	face  uint8
}

// neatIndex finds the piece on a tile, in O(1). It is built only when a
// stored stack first asks, and rebuilt only when the buildings change:
// StructVer, or another map's list (the viewed map moves no StructVer).
// heaps is the label pass's per-cell tally, reused between frames.
type neatIndex struct {
	built       bool
	ver         uint64
	n           int
	first, last entity.EntityID
	cells       map[[2]int]neatCell
	heaps       map[[2]int]int
}

// sync rebuilds the index if the building list changed.
func (ix *neatIndex) sync(ver uint64, bs []game.BuildingView) {
	var first, last entity.EntityID
	if len(bs) > 0 {
		first, last = bs[0].ID, bs[len(bs)-1].ID
	}
	if ix.built && ix.ver == ver && ix.n == len(bs) && ix.first == first && ix.last == last {
		return
	}
	ix.built, ix.ver, ix.n, ix.first, ix.last = true, ver, len(bs), first, last
	if ix.cells == nil {
		ix.cells = map[[2]int]neatCell{}
	}
	clear(ix.cells)
	for i := range bs {
		b := &bs[i]
		if b.Floor {
			continue
		}
		p := neatByName[b.Kind]
		if p == nil {
			continue
		}
		w, h := max(b.W, 1), max(b.H, 1)
		c := neatCell{piece: p, x: b.X, y: b.Y, h: h, face: neatFacing(b.Facing)}
		for dy := 0; dy < h; dy++ {
			for dx := 0; dx < w; dx++ {
				ix.cells[[2]int{b.X + dx, b.Y + dy}] = c
			}
		}
	}
}

// at is the piece p under a stored stack. ok is false when the buildings
// don't show that piece on the tile: the cell is then the stack's own tile.
func (ix *neatIndex) at(ver uint64, bs []game.BuildingView, p *neatPiece, it *game.ItemView) (neatCell, bool) {
	ix.sync(ver, bs)
	if c, ok := ix.cells[[2]int{it.X, it.Y}]; ok && c.piece == p {
		return c, true
	}
	return neatCell{piece: p, x: it.X, y: it.Y, h: 1}, false
}

// stackRow is a stored stack's draw row in cell c: after the piece, which
// sorts by its bottom row, and before the next row's objects.
func stackRow(c neatCell, it *game.ItemView) float64 {
	slot := min(int(it.Slot), c.piece.slots-1) // an overfilled cell can pass its slots
	return float64(c.y+c.h-1) + neatRowCellStep*float64(1+it.Y-c.y) + neatRowSlotStep*float64(slot)
}

// slotPoint is where a stack's centre sits, in world tiles, and its scale:
// the slot attachment of the piece's sprite m (nil: no sprite or no piece),
// given for the piece's first cell, moved to the stack's own cell. A stack past
// the last slot (a spill can overfill a cell) shares that slot. ok is false
// without an attachment, when the stack sits on the centre of its tile.
func slotPoint(m *art.Meta, it *game.ItemView) (ax, ay, k float64, ok bool) {
	k = 1
	if m != nil {
		if m.SlotScale > 0 {
			k = m.SlotScale
		}
		if a, has := m.Slot(min(int(it.Slot), len(m.Slots)-1)); has {
			ppt := m.PxPerTile()
			return float64(it.X) + (a[0]-m.Origin[0])/ppt, float64(it.Y) + (a[1]-m.Origin[1])/ppt, k, true
		}
	}
	return float64(it.X) + 0.5, float64(it.Y) + 0.5, k, false
}

// stackOrigin is the world tile to draw a sprite of footprint fw x fh at so
// that, scaled by k about its footprint's bottom-centre (sprite.DrawScaled),
// the footprint's centre lands on (ax, ay).
func stackOrigin(ax, ay, k float64, fw, fh int) (wx, wy float64) {
	return ax - float64(fw)/2, ay - float64(fh)*(1-k/2)
}

// heapBody scales a lying figure's pose by k and gives the tile to draw it at so
// that the body's middle lands on (ax, ay). LyingPose's offsets centre the
// figure at full size, so they shrink with it; left whole, they push a small
// body off its heap.
func heapBody(ps motion.Pose, ax, ay, k float64) (out motion.Pose, wx, wy float64) {
	ps.Sx, ps.Sy = k, k
	ps.Dx *= k
	ps.Dy *= k
	return ps, ax - ps.Px, ay - ps.Py - ps.Dy
}

// stackSlot is slotPoint for stack it of piece p on this frame's world.
func (b *base) stackSlot(p *neatPiece, it *game.ItemView) (ax, ay, k float64, ok bool) {
	w := &b.v.W
	c, found := b.neat.at(w.StructVer, w.Buildings, p, it)
	if !found {
		return slotPoint(nil, it)
	}
	return slotPoint(b.art().MetaOf(p.keys[c.face]), it)
}

// showsCount reports a stack of qty in this piece prints its count at tile:
// never in a closed piece, only from DetailMin with many slots.
func (p *neatPiece) showsCount(qty int, tile float64) bool {
	return p.show != showNone && qty > 1 && (p.slots < neatManySlots || tile >= DetailMin)
}

// stackOnEnd is the pose that stands a sprite of footprint fw x fh on end
// about its footprint's centre, scaled by k: draw it at the tile that puts
// that centre on the slot.
func stackOnEnd(fw, fh, k float64, cs sprite.ColorScale) motion.Pose {
	return motion.Pose{Px: fw / 2, Py: fh / 2, Rot: neatOnEndRot, Sx: k, Sy: k, Cs: cs}
}
