package ebiten

import (
	"bytes"
	"compress/flate"
	"container/list"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Memory diet (PLAY-019 §2). The game's catalog (loadArtCatalog) decodes
// nothing at load: a sprite is decoded the first time it is used. So the
// first use seldom pays a PNG decode mid-frame, a background prewarm
// (prewarmAsync) decodes the families every colony shows ahead and keeps
// them deflated (a fill then inflates: ~3x faster than the PNG, and ~1/12
// of the pixels' size), like the atlas pages' copies. A sprite the prewarm
// has not reached yet is decoded on the spot (counted as a cold fill)
// rather than skipped: a sprite missing for a frame would pop in, and a
// chunk bake would keep the hole until its next rebake.
//
// Once a sprite's GPU image exists its CPU pixels go (release), except for
// the sprites a CPU bake reads back. Tint bakes run on the GPU (tint.kage)
// from the sprite's image and its mask's coverage box, so no tinted sprite
// keeps CPU pixels either, and the bakes are bounded by an LRU.

// prewarmSkipFamilies are file families the background prewarm leaves to
// first use: animal is 4608 sprites (765 MiB decoded, 122 MiB deflated)
// and a map shows a few species.
var prewarmSkipFamilies = map[string]bool{"animal": true}

// prewarmFirst are the families the prewarm takes first, most seen first;
// the others follow by key.
var prewarmFirst = []string{"terrain", "plant", "rock", "structure", "overlay", "item", "furniture"}

// defaultTintBudget bounds the tint bakes' pixels (canvas w*h*4 each): a
// pawn layer bake is 186 KiB, so this holds ~700 bakes, the walk and work
// frames of a large colony's colonists on screen.
const defaultTintBudget = 128 << 20

// artMem is a catalog's memory bookkeeping, shared with its file sources.
type artMem struct {
	// mu guards every fileSrc's warm and taken, and warmHeld.
	mu       sync.Mutex
	warmHeld int64 // deflated bytes the prewarm holds

	warmFills, coldFills  atomic.Int64 // file sprites filled from a prewarmed copy / from the PNG
	freed, freedBytes     atomic.Int64 // sprites whose CPU pixels were released after upload
	kept, keptBytes       atomic.Int64 // uploaded sprites whose pixels stay (CPU bakes read them)
	warmPublished         atomic.Int64 // layers the prewarm published
	tintEvicted, tintBake atomic.Int64 // tint bakes evicted / made
}

// artMemStats is a snapshot of a catalog's memory bookkeeping.
type artMemStats struct {
	WarmHeld             int64 // deflated prewarm copies still held, bytes
	AtlasHeld            int64 // atlas sprites' deflated copies still held, bytes
	WarmFills, ColdFills int64
	Freed, FreedBytes    int64
	Kept, KeptBytes      int64
	Tints                int // tint bakes cached
	TintBytes            int64
	TintBakes            int64 // made since load
	TintEvicted          int64
}

// MemStats reads the catalog's memory bookkeeping (frame goroutine).
func (c *artCatalog) MemStats() artMemStats {
	var s artMemStats
	if c == nil {
		return s
	}
	if m := c.mem; m != nil {
		m.mu.Lock()
		s.WarmHeld = m.warmHeld
		m.mu.Unlock()
		s.WarmFills, s.ColdFills = m.warmFills.Load(), m.coldFills.Load()
		s.Freed, s.FreedBytes = m.freed.Load(), m.freedBytes.Load()
		s.Kept, s.KeptBytes = m.kept.Load(), m.keptBytes.Load()
		s.TintBakes, s.TintEvicted = m.tintBake.Load(), m.tintEvicted.Load()
	}
	s.Tints, s.TintBytes = c.tintLRU.order.Len(), c.tintLRU.bytes
	seen := map[*spriteAtlas]bool{}
	for _, sp := range c.Sprites {
		if sp.Src == nil || seen[sp.Src.Atlas] {
			continue
		}
		a := sp.Src.Atlas
		seen[a] = true
		a.mu.Lock()
		for _, pg := range a.ByPage {
			for _, o := range pg {
				for _, z := range o.Z {
					s.AtlasHeld += int64(len(z))
				}
			}
		}
		a.mu.Unlock()
	}
	return s
}

// String is the probe's art line (fps_probe.go).
func (s artMemStats) String() string {
	mib := func(b int64) float64 { return float64(b) / (1 << 20) }
	return fmt.Sprintf("art: cpu freed %d (%.0f MiB), kept %d (%.1f MiB); prewarm held %.1f MiB, atlas held %.0f MiB, fills %d warm / %d cold; tints %d (%.0f MiB), %d baked, %d evicted",
		s.Freed, mib(s.FreedBytes), s.Kept, mib(s.KeptBytes), mib(s.WarmHeld), mib(s.AtlasHeld), s.WarmFills, s.ColdFills,
		s.Tints, mib(s.TintBytes), s.TintBakes, s.TintEvicted)
}

// --- prewarmed file copies ----------------------------------------------

// warmCopy is one layer of a file sprite decoded ahead by the prewarm,
// deflated; r is the decoded image's rectangle.
type warmCopy struct {
	Z []byte
	r image.Rectangle
}

// takeWarm marks layer of f taken (the prewarm publishes no copy of it
// after this) and returns its prewarmed pixels, nil when there were none.
func (f *fileSrc) takeWarm(layer int) *image.NRGBA {
	if f.mem == nil {
		return nil
	}
	f.mem.mu.Lock()
	f.taken[layer] = true
	w := f.Warm[layer]
	f.Warm[layer] = warmCopy{}
	f.mem.warmHeld -= int64(len(w.Z))
	f.mem.mu.Unlock()
	if w.Z == nil {
		return nil
	}
	return inflateNRGBA(w.Z, w.r)
}

// isTaken reports whether a fill already took layer of f.
func (f *fileSrc) isTaken(layer int) bool {
	f.mem.mu.Lock()
	defer f.mem.mu.Unlock()
	return f.taken[layer]
}

// publish stores the prewarm's copy of layer unless a fill took the layer
// meanwhile.
func (f *fileSrc) publish(layer int, z []byte, r image.Rectangle) {
	f.mem.mu.Lock()
	defer f.mem.mu.Unlock()
	if f.taken[layer] {
		return
	}
	f.Warm[layer] = warmCopy{Z: z, r: r}
	f.mem.warmHeld += int64(len(z))
	f.mem.warmPublished.Add(1)
}

// NormalNRGBA is sp's normal map from its prewarmed copy or its file, nil
// when it has none.
func (f *fileSrc) NormalNRGBA(sp *artSprite) *image.NRGBA {
	if sp.NormalPath == "" {
		return nil
	}
	if n := f.takeWarm(layerNorm); n != nil {
		return n
	}
	n, _ := decodeNRGBA(sp.NormalPath)
	return n
}

// prewarmFileKeys lists the file sprites the prewarm decodes, in its order.
func (c *artCatalog) prewarmFileKeys() []string {
	rank := map[string]int{}
	for i, f := range prewarmFirst {
		rank[f] = i + 1
	}
	var keys []string
	for k, sp := range c.Sprites {
		fam, _, _ := strings.Cut(k, "/")
		if sp.File != nil && sp.File.mem != nil && !prewarmSkipFamilies[fam] {
			keys = append(keys, k)
		}
	}
	famRank := func(k string) int {
		fam, _, _ := strings.Cut(k, "/")
		if r := rank[fam]; r > 0 {
			return r
		}
		return len(prewarmFirst) + 1
	}
	sort.Slice(keys, func(i, j int) bool {
		if a, b := famRank(keys[i]), famRank(keys[j]); a != b {
			return a < b
		}
		return keys[i] < keys[j]
	})
	return keys
}

// prewarmWorkers is how many goroutines decode files ahead: ~1700
// sprites at ~0.5 ms a layer take ~0.5 s on 4 (a decode holds one sprite,
// so memory does not grow with them).
const prewarmWorkers = 4

// prewarmFiles decodes each layer of keys' sprites not yet taken and
// publishes it deflated (the background prewarm's file half), keys taken
// in order by prewarmWorkers goroutines. Sprites, paths and keys never
// change after load; only warm and taken are shared (under mem.mu).
func (c *artCatalog) prewarmFiles(keys []string) {
	var next atomic.Int64
	var wg sync.WaitGroup
	for range prewarmWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var z deflater
			for i := next.Add(1) - 1; i < int64(len(keys)); i = next.Add(1) - 1 {
				sp := c.Sprites[keys[i]]
				f := sp.File
				for layer, path := range [atlasLayers]string{layerPix: f.Image, layerMask: f.mask, layerNorm: sp.NormalPath} {
					if path == "" || f.isTaken(layer) {
						continue
					}
					img, err := decodeNRGBA(path)
					if err != nil {
						continue // the fill decodes it again and keeps the error
					}
					f.publish(layer, z.deflate(img.Pix), img.Rect)
				}
			}
		}()
	}
	wg.Wait()
}

// inflaters reuses flate readers across fills (any goroutine).
var inflaters sync.Pool

// inflateNRGBA expands a deflated copy of an image over r, nil when corrupt.
func inflateNRGBA(z []byte, r image.Rectangle) *image.NRGBA {
	out := image.NewNRGBA(r)
	src := bytes.NewReader(z)
	zr, _ := inflaters.Get().(io.ReadCloser)
	if zr == nil {
		zr = flate.NewReader(src)
	} else if err := zr.(flate.Resetter).Reset(src, nil); err != nil {
		return nil
	}
	defer inflaters.Put(zr)
	if _, err := io.ReadFull(zr, out.Pix); err != nil {
		return nil
	}
	return out
}

// --- releasing CPU pixels ------------------------------------------------

// keepsCPUPixels reports a sprite whose pixels CPU bakes read back after
// it may have been drawn: crop units and their fallbacks and the furrow
// decals (crop_batch.go), and grass tufts (grass_batch.go).
func keepsCPUPixels(key string) bool {
	return strings.HasPrefix(key, "overlay/tilled_") || cpuReadKeys()[key]
}

var cpuReadKeys = sync.OnceValue(func() map[string]bool {
	m := map[string]bool{}
	for _, ks := range plantKeys.crops {
		for st := range ks {
			for _, k := range ks[st] {
				m[k] = true
				for _, fb := range plantKeys.fallback[k] {
					m[fb] = true
				}
			}
		}
	}
	for _, set := range [][]string{grassShortKeys, grassTallKeys, grassDormantKeys} {
		for _, k := range set {
			m[k] = true
		}
	}
	return m
})

// release frees sp's CPU pixels once its GPU image and normal map exist
// (the game's catalog only): its opaque height is measured first, and its
// tint mask shrinks to its coverage box (tintNRGBA and the GPU bake read
// nothing outside it).
func (c *artCatalog) release(key string, sp *artSprite) {
	if !c.FreeCPU || sp.Pix == nil || c.mem == nil {
		return
	}
	n := int64(len(sp.Pix.Pix))
	if keepsCPUPixels(key) {
		c.mem.kept.Add(1)
		c.mem.keptBytes.Add(n)
		return
	}
	sp.OpaqueHeightPx()
	if sp.Mask != nil {
		m := maskCoverage(sp.Mask)
		n += int64(len(sp.Mask.Pix) - len(m.Pix))
		sp.Mask = m
	}
	sp.Pix = nil
	c.mem.freed.Add(1)
	c.mem.freedBytes.Add(n)
}

// maskCoverage is mask cut to the box of its covered pixels (red > 0), as
// an opaque grey copy (R=G=B=coverage, A=255); empty when nothing is
// covered.
func maskCoverage(mask *image.NRGBA) *image.NRGBA {
	b := mask.Bounds()
	x0, y0, x1, y1 := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := mask.Pix[mask.PixOffset(b.Min.X, y):]
		for x := 0; x < b.Dx(); x++ {
			if row[4*x] != 0 {
				x0, x1 = min(x0, b.Min.X+x), max(x1, b.Min.X+x+1)
				y0, y1 = min(y0, y), max(y1, y+1)
			}
		}
	}
	box := image.Rect(x0, y0, x1, y1)
	if x1 <= x0 { // nothing covered (image.Rect would swap the bounds)
		box = image.Rectangle{}
	}
	out := image.NewNRGBA(box)
	for y := box.Min.Y; y < box.Max.Y; y++ {
		src := mask.Pix[mask.PixOffset(box.Min.X, y):]
		dst := out.Pix[out.PixOffset(box.Min.X, y):]
		for x := 0; x < box.Dx(); x++ {
			k := src[4*x]
			dst[4*x], dst[4*x+1], dst[4*x+2], dst[4*x+3] = k, k, k, 255
		}
	}
	return out
}

// maskImage is the GPU copy of mask's coverage within clip (the sprite's
// bounds) and that box; nil and an empty box when nothing is covered.
func maskImage(mask *image.NRGBA, clip image.Rectangle) (*ebiten.Image, image.Rectangle) {
	cov := maskCoverage(mask)
	box := cov.Rect.Intersect(clip)
	if box.Empty() {
		return nil, image.Rectangle{}
	}
	return ebiten.NewImageFromImage(cov.SubImage(box)), box
}

// --- tint bakes on the GPU --------------------------------------------------

//go:embed tint.kage
var tintKage []byte

// tintShader is tint.kage, compiled once.
var tintShader = sync.OnceValues(func() (*ebiten.Shader, error) { return ebiten.NewShader(tintKage) })

// tintTexel is tint.kage's colour channel (its Go twin; tintNRGBA is the
// CPU bake it matches to two levels, flooring twice where this rounds
// once): v * mix(1, t, k), all 0..255.
func tintTexel(v, k, t uint8) uint8 {
	kf, tf := float64(k)/255, float64(t)/255
	return uint8(math.Round(float64(v) * (1 + (tf-1)*kf)))
}

// bakeTint draws sp's image tinted through its mask into a new image of
// the same size: a copy, then tint.kage over the mask's coverage box.
func (c *artCatalog) bakeTint(sp *artSprite, tint color.NRGBA) *ebiten.Image {
	b := sp.Img.Bounds()
	dst := ebiten.NewImage(b.Dx(), b.Dy())
	dst.DrawImage(sp.Img, &ebiten.DrawImageOptions{Blend: ebiten.BlendCopy})
	if sp.maskImg == nil && !sp.maskDone {
		sp.maskDone = true
		sp.maskImg, sp.maskBox = maskImage(sp.Mask, b)
	}
	sh, err := tintShader()
	if sp.maskImg == nil || err != nil {
		return dst
	}
	box := sp.maskBox
	op := &ebiten.DrawRectShaderOptions{Blend: ebiten.BlendCopy}
	op.GeoM.Translate(float64(box.Min.X-b.Min.X), float64(box.Min.Y-b.Min.Y))
	op.Images[0] = sp.Img.SubImage(box).(*ebiten.Image)
	op.Images[1] = sp.maskImg
	op.Uniforms = map[string]any{"Tint": []float32{float32(tint.R) / 255, float32(tint.G) / 255, float32(tint.B) / 255}}
	dst.DrawRectShader(box.Dx(), box.Dy(), sh, op)
	return dst
}

// tintLRU orders the tint bakes in c.tinted (which light.go scans) by
// use, most recent first, and evicts the least recently used past the
// budget. Frame goroutine only.
type tintLRU struct {
	order list.List // of *tintEntry
	at    map[tintKey]*list.Element
	bytes int64
}

type tintEntry struct {
	k     tintKey
	img   *ebiten.Image
	bytes int64
}

// cachedTint returns the bake for k, marking it used; nil when none.
func (c *artCatalog) cachedTint(k tintKey) *ebiten.Image {
	img, ok := c.Tinted[k]
	if !ok {
		return nil
	}
	if e := c.tintLRU.at[k]; e != nil {
		c.tintLRU.order.MoveToFront(e)
	}
	return img
}

// addTint caches a new bake for k and evicts the least recently used ones
// while the bakes exceed the budget (never the new one): each leaves
// c.tinted and c.normals and is deallocated.
func (c *artCatalog) addTint(k tintKey, img *ebiten.Image) {
	if c.Tinted == nil {
		c.Tinted = map[tintKey]*ebiten.Image{}
	}
	if c.tintLRU.at == nil {
		c.tintLRU.at = map[tintKey]*list.Element{}
	}
	sz := img.Bounds().Size()
	e := &tintEntry{k: k, img: img, bytes: int64(sz.X * sz.Y * 4)}
	c.Tinted[k] = img
	c.tintLRU.at[k] = c.tintLRU.order.PushFront(e)
	c.tintLRU.bytes += e.bytes
	if c.mem != nil {
		c.mem.tintBake.Add(1)
	}
	budget := int64(c.TintBudget)
	if budget <= 0 {
		budget = defaultTintBudget
	}
	for c.tintLRU.bytes > budget && c.tintLRU.order.Len() > 1 {
		old := c.tintLRU.order.Remove(c.tintLRU.order.Back()).(*tintEntry)
		delete(c.tintLRU.at, old.k)
		delete(c.Tinted, old.k)
		delete(c.Normals, old.img)
		c.tintLRU.bytes -= old.bytes
		old.img.Deallocate()
		if c.mem != nil {
			c.mem.tintEvicted.Add(1)
		}
	}
}
