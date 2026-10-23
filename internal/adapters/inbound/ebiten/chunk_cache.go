package ebiten

import (
	"image"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// chunkCache holds the chunk tier's baked ground images (lod.go): one
// image per chunk at chunkBakePx per tile with a bakePad margin, keyed by
// chunk index and stamped with the chunk version it shows; the least
// recently drawn is reused once chunkCacheMax are held (~24 MB).
type chunkCache struct {
	byChunk map[int]*chunkBake
	frame   uint64
	held    int
	// bakes counts bakes made (tests).
	bakes int
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

func rectOf(x0, y0, x1, y1 int) image.Rectangle { return image.Rect(x0, y0, x1, y1) }

// get returns chunk c's bake and whether it must be (re)baked to show
// version ver: a missing chunk gets a cleared image when canBake, else
// nil; a stale one is returned as it is (shown a frame or two late)
// unless canBake. The caller bakes into b.img and calls done.
func (cc *chunkCache) get(c int, ver uint64, canBake bool) (b *chunkBake, bake bool) {
	if cc.byChunk == nil {
		cc.byChunk = map[int]*chunkBake{}
	}
	b = cc.byChunk[c]
	if b != nil && (b.ver == ver || !canBake) {
		b.used = cc.frame
		return b, false
	}
	if !canBake {
		return nil, false
	}
	if b == nil {
		b = cc.take()
		b.chunk = c
		cc.byChunk[c] = b
	}
	b.img.Clear()
	return b, true
}

// done stamps a fresh bake of version ver.
func (cc *chunkCache) done(b *chunkBake, ver uint64) {
	b.ver, b.used = ver, cc.frame
	cc.bakes++
}

// take is a free image: a new one while under chunkCacheMax, else the
// least recently drawn (never one drawn this frame while others exist).
func (cc *chunkCache) take() *chunkBake {
	if cc.held < chunkCacheMax {
		cc.held++
		return &chunkBake{img: ebiten.NewImage(chunkBakeSide, chunkBakeSide)}
	}
	var old *chunkBake
	for _, b := range cc.byChunk {
		if old == nil || b.used < old.used || (b.used == old.used && b.chunk < old.chunk) {
			old = b
		}
	}
	delete(cc.byChunk, old.chunk)
	return old
}

// drawChunkBake draws tiles tx0..tx1 × ty0..ty1 of chunk (cx, cy) from
// its bake, baking it first when missing or stale (within the frame's
// budget). False: nothing drawn.
func (r *Renderer) drawChunkBake(screen *ebiten.Image, w *game.WorldView, cx, cy int, cv game.ChunkView, tx0, ty0, tx1, ty1 int) bool {
	cs := game.ChunkSize
	mw, mh := r.game.Width(), r.game.Height()
	c := cy*((mw+cs-1)/cs) + cx
	x0, y0 := cx*cs, cy*cs
	b, bake := r.lod.chunks.get(c, cv.Ver, r.lod.budget > 0)
	if b == nil {
		return false
	}
	if bake {
		r.lod.budget--
		r.bakeGround(b.img, w, x0, y0, min(x0+cs, mw), min(y0+cs, mh), chunkBakePx, bakePad, bakePad, false)
		r.lod.chunks.done(b, cv.Ver)
	}
	r.drawBaked(screen, b.img, x0, y0, bakePad, bakePad, chunkBakePx, tx0, ty0, tx1, ty1)
	return true
}
