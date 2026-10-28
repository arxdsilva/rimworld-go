package world

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/application/game"
)

// ChunkCache holds the chunk tier's baked ground images (lod.go): one
// image per chunk at chunkBakePx per tile with a bakePad margin, keyed by
// chunk index and stamped with the chunk version it shows; the least
// recently drawn is reused once chunkCacheMax are held (~24 MB).
type ChunkCache struct {
	ByChunk map[int]*chunkBake
	Frame   uint64
	held    int
	// Bakes counts bakes made (tests).
	Bakes int
}

type chunkBake struct {
	img   *ebiten.Image
	chunk int
	ver   uint64
	used  uint64 // frame last drawn
}

// chunkCacheMax is how many chunk images are kept.
const chunkCacheMax = 96

// chunkBakeSide is a chunk image's side in pixels.
const chunkBakeSide = game.ChunkSize*chunkBakePx + 2*bakePad

// get returns chunk c's bake and whether it must be (re)baked to show
// version ver: a missing chunk gets a cleared image when canBake, else
// nil; a stale one is returned as it is (shown a frame or two late)
// unless canBake. The caller bakes into b.img and calls done.
func (cc *ChunkCache) get(c int, ver uint64, canBake bool) (b *chunkBake, bake bool) {
	if cc.ByChunk == nil {
		cc.ByChunk = map[int]*chunkBake{}
	}
	b = cc.ByChunk[c]
	if b != nil && (b.ver == ver || !canBake) {
		b.used = cc.Frame
		return b, false
	}
	if !canBake {
		return nil, false
	}
	if b == nil {
		b = cc.take()
		b.chunk = c
		cc.ByChunk[c] = b
	}
	b.img.Clear()
	return b, true
}

// done stamps a fresh bake of version ver.
func (cc *ChunkCache) done(b *chunkBake, ver uint64) {
	b.ver, b.used = ver, cc.Frame
	cc.Bakes++
}

// take is a free image: a new one while under chunkCacheMax, else the
// least recently drawn (never one drawn this frame while others exist).
func (cc *ChunkCache) take() *chunkBake {
	if cc.held < chunkCacheMax {
		cc.held++
		return &chunkBake{img: ebiten.NewImage(chunkBakeSide, chunkBakeSide)}
	}
	var old *chunkBake
	for _, b := range cc.ByChunk {
		if old == nil || b.used < old.used || (b.used == old.used && b.chunk < old.chunk) {
			old = b
		}
	}
	delete(cc.ByChunk, old.chunk)
	return old
}

// drawChunkBake draws tiles tx0..tx1 × ty0..ty1 of chunk (cx, cy) from
// its bake, baking it first when missing or stale (within the frame's
// budget). False: nothing drawn.
func (k *Bakes) drawChunkBake(screen *ebiten.Image, w *game.WorldView, cx, cy int, cv game.ChunkView, tx0, ty0, tx1, ty1 int) bool {
	cs := game.ChunkSize
	mw, mh := k.v.MapW, k.v.MapH
	c := cy*((mw+cs-1)/cs) + cx
	x0, y0 := cx*cs, cy*cs
	ver, tufts := cv.Ver, k.LOD.Tier == Chunk && k.Cam.Tile() < env.GrassLiveMin
	if tufts { // 16–24 px: the image holds the chunk's tufts too (render/env)
		ver = env.GrassBakeKey(ver, k.Env.GrassChunkSig(k.v, w, cx, cy, k.Index))
	}
	b, bake := k.LOD.Chunks.get(c, ver, k.LOD.budget > 0)
	if b == nil {
		return false
	}
	if bake {
		k.LOD.budget--
		x1, y1 := min(x0+cs, mw), min(y0+cs, mh)
		k.bakeGround(b.img, w, x0, y0, x1, y1, chunkBakePx, bakePad, bakePad, false)
		if tufts {
			k.bakeTufts(b.img, w, x0, y0, x1, y1, chunkBakePx, bakePad, bakePad)
		}
		k.LOD.Chunks.done(b, ver)
	}
	k.DrawBaked(screen, b.img, x0, y0, bakePad, bakePad, chunkBakePx, tx0, ty0, tx1, ty1)
	return true
}

// bakeTufts bakes the grass tufts of tiles x0..x1-1 × y0..y1-1 into dst
// at px per tile, tile (x0, y0) at (ox, oy) (the 16–24 px chunk images).
func (k *Bakes) bakeTufts(dst *ebiten.Image, w *game.WorldView, x0, y0, x1, y1 int, px, ox, oy float64) {
	k.WithBake(dst, px, x0, y0, ox, oy, func(sw, sh int, tile float64) {
		k.Env.DrawGrassBatch(k.v, dst, w, x0, y0, x1-1, y1-1, tile, sw, sh, env.GrassMidCap, k.LOD.Baking, k.Index)
	})
}
