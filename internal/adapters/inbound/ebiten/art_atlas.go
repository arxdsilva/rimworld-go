package ebiten

import (
	"bufio"
	"bytes"
	"compress/flate"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Atlas-packed sprite families (NG-1 plan 7.1). cmd/atlasgen packs a
// family's sprites into a few large pages with
// metadata/atlas/<family>.yaml; when that file exists the catalog serves
// the family from the pages instead of reading its individual files.
// Sprite metadata, origins, attachments and tint masks are exactly those
// of the individual files; only where the pixels come from changes.
//
// Pages are decoded on first use (a family can run to a few GB of pixels
// unpacked, so decoding them all at start would be the cost this avoids).
// A sprite is cut out of its page into a canvas-sized image once, the
// first time the catalog hands it out, so tinting, measuring and normal
// maps work on it as on a file-backed sprite.

// atlasMetaDir is the metadata directory of atlas indexes, not a sprite family.
const atlasMetaDir = "atlas"

// atlasPageCache is how many decoded pages may be held at once: the one
// being cut (see take).
const atlasPageCache = 1

// atlasEntry is one parsed atlas sprite.
type atlasEntry struct {
	Key    string
	Page   int
	Rect   image.Rectangle // trimmed pixels in the page
	Trim   image.Point     // where rect sits in the sprite canvas
	Normal bool
	Meta   string // the sprite's own metadata, as in its individual yaml
}

// parseSpriteAtlas reads metadata/atlas/<family>.yaml: the family name,
// the page count and every sprite.
func parseSpriteAtlas(src string) (family string, pages int, out []atlasEntry, err error) {
	var cur *atlasEntry
	var meta strings.Builder
	flush := func() {
		if cur != nil {
			cur.Meta = meta.String()
			out = append(out, *cur)
		}
		meta.Reset()
	}
	inSprites := false
	sc := bufio.NewScanner(strings.NewReader(src))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for ln := 1; sc.Scan(); ln++ {
		line := strings.TrimRight(sc.Text(), " \t\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		k, v, _ := strings.Cut(strings.TrimSpace(line), ":")
		v = strings.Trim(strings.TrimSpace(v), `"`)
		switch {
		case indent == 0:
			switch k {
			case "family":
				family = v
			case "pages":
				if pages, err = strconv.Atoi(v); err != nil {
					return "", 0, nil, fmt.Errorf("atlas line %d: pages: %w", ln, err)
				}
			case "sprites":
				inSprites = true
			}
		case inSprites && indent == 2:
			flush()
			cur = &atlasEntry{Key: k}
		case inSprites && cur != nil && indent >= 4:
			if indent == 4 {
				switch k {
				case "page":
					if cur.Page, err = strconv.Atoi(v); err != nil {
						return "", 0, nil, fmt.Errorf("atlas %s: page: %w", cur.Key, err)
					}
					continue
				case "rect":
					var r [4]int
					if r, err = atlasInts(v, 4); err != nil {
						return "", 0, nil, fmt.Errorf("atlas %s: rect: %w", cur.Key, err)
					}
					cur.Rect = image.Rect(r[0], r[1], r[0]+r[2], r[1]+r[3])
					continue
				case "trim":
					var r [4]int
					if r, err = atlasInts(v, 2); err != nil {
						return "", 0, nil, fmt.Errorf("atlas %s: trim: %w", cur.Key, err)
					}
					cur.Trim = image.Pt(r[0], r[1])
					continue
				case "normal":
					cur.Normal = v == "true"
					continue
				}
			}
			meta.WriteString(line[4:] + "\n")
		}
	}
	flush()
	return family, pages, out, sc.Err()
}

func atlasInts(v string, n int) ([4]int, error) {
	var out [4]int
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "[") || !strings.HasSuffix(v, "]") {
		return out, fmt.Errorf("want a list, got %q", v)
	}
	parts := strings.Split(v[1:len(v)-1], ",")
	if len(parts) != n {
		return out, fmt.Errorf("want %d numbers, got %q", n, v)
	}
	for i, p := range parts {
		x, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return out, err
		}
		out[i] = x
	}
	return out, nil
}

// Pages are decoded at most once per layer (sprite pixels, tint masks,
// normal maps): the first sprite asked for decodes its page, every sprite
// on that page is cut to its trimmed rectangle and stored deflated, and
// the page is dropped at once. A sprite's compressed copy is released once
// the catalog has taken it (fill, normalImage), so memory holds only the
// sprites in use plus the compressed rest of the pages touched, never a
// decoded page between calls (a decoded page is ~67 MB per layer, and the
// old 4-page cache thrashed when pawns used sprites spread over 6 pages).

// Atlas page layers.
const (
	layerPix = iota
	layerMask
	layerNorm
	atlasLayers
)

// pageData is a decoded page's layers; it lives only while its sprites are
// being cut (the cache map stays empty between calls).
type pageData struct {
	pix, norm *image.NRGBA
	mask      *image.Gray
}

// spriteAtlas serves the sprites of one packed family.
type spriteAtlas struct {
	root string
	mu   sync.Mutex
	// Cache holds decoded pages while they are cut; empty between calls.
	Cache map[int]*pageData
	// ByPage lists every sprite of each page; cut marks the page layers
	// already cut into the sprites' compressed copies.
	ByPage [][]*atlasSrc
	Cut    [][atlasLayers]bool
	// Decodes counts page-layer Decodes (tests, the memory benchmark).
	Decodes int
	def     deflater
	zr      io.ReadCloser
	zsrc    bytes.Reader
}

// atlasSrc ties a sprite to its place in an atlas.
type atlasSrc struct {
	Atlas *spriteAtlas
	Entry atlasEntry
	// page file paths (from the sprite metadata)
	pagePath, maskPath string
	// Z holds the trimmed pixels of each layer, deflated, from when the
	// page layer is cut until the catalog takes them; nil after (or when
	// the page was unreadable).
	Z [atlasLayers][]byte
}

// layerPath is the page file of a layer of s's page.
func (s *atlasSrc) layerPath(layer int) string {
	switch layer {
	case layerMask:
		return s.maskPath
	case layerNorm:
		return strings.TrimSuffix(s.pagePath, ".png") + "_n.png"
	}
	return s.pagePath
}

// trimBytes is the bytes per pixel of a layer's trimmed copy: NRGBA
// pixels, grey masks and RGB normals.
func trimBytes(layer int) int {
	switch layer {
	case layerMask:
		return 1
	case layerNorm:
		return 3
	}
	return 4
}

// take returns s's trimmed pixels of layer (trimBytes per pixel, row-
// major over entry.rect), nil when the page is unreadable, and releases
// s's compressed copy (the catalog keeps what it cut). The first take on
// a page layer cuts the whole page (cutPage); a second take of the same
// sprite decodes the page again for that sprite alone.
func (a *spriteAtlas) take(s *atlasSrc, layer int) []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	pg := s.Entry.Page
	a.cutPage(pg, layer)
	if z := s.Z[layer]; z != nil {
		s.Z[layer] = nil
		return a.inflateAll(z, s.Entry.Rect.Dx()*s.Entry.Rect.Dy()*trimBytes(layer))
	}
	pd := a.decodePage(pg, layer, s.layerPath(layer))
	defer delete(a.Cache, pg)
	return trimmedOf(pd, layer, s.Entry.Rect)
}

// cutPage decodes one layer of page pg, once, keeps every sprite's
// trimmed rect of it compressed and drops the page. a.mu must be held.
func (a *spriteAtlas) cutPage(pg, layer int) {
	if a.Cut[pg][layer] {
		return
	}
	a.Cut[pg][layer] = true
	path := ""
	for _, o := range a.ByPage[pg] { // sprites without a tint mask name none
		if path = o.layerPath(layer); path != "" {
			break
		}
	}
	if path == "" {
		return
	}
	pd := a.decodePage(pg, layer, path)
	defer delete(a.Cache, pg)
	for _, o := range a.ByPage[pg] {
		if (layer == layerMask && o.maskPath == "") || (layer == layerNorm && !o.Entry.Normal) {
			continue // never asked for
		}
		if raw := trimmedOf(pd, layer, o.Entry.Rect); raw != nil {
			o.Z[layer] = a.deflateAll(raw)
		}
	}
}

// prewarm cuts every page layer of the atlas now (a loading screen's
// work, and benchmarks that time the steady state), leaving no decoded
// page behind; it returns the compressed bytes held.
func (a *spriteAtlas) prewarm() (held int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for pg := range a.ByPage {
		for layer := 0; layer < atlasLayers; layer++ {
			a.cutPage(pg, layer)
		}
		for _, o := range a.ByPage[pg] {
			for _, z := range o.Z {
				held += len(z)
			}
		}
	}
	return held
}

// PrewarmAtlases cuts every packed family's pages (see spriteAtlas.
// prewarm) and returns the compressed bytes held.
func (c *artCatalog) PrewarmAtlases() (held int) {
	if c == nil {
		return 0
	}
	seen := map[*spriteAtlas]bool{}
	for _, sp := range c.Sprites {
		if sp.Src != nil && !seen[sp.Src.Atlas] {
			seen[sp.Src.Atlas] = true
			held += sp.Src.Atlas.prewarm()
		}
	}
	return held
}

// PrewarmAsync cuts every packed family's page layers on a background
// goroutine, so play seldom meets an uncut page: cutting one on first use
// decodes a ~67 MB page layer mid-frame (~0.4 s, a visible hitch; NG-1
// §6.1 draw budget). Decoding and compressing run outside the atlas lock
// with the goroutine's own compressor; the lock is held only to publish
// a page layer's copies, and one a draw cut meanwhile is left as it is.
// The result is what cutPage leaves (the compressed copies; see prewarm).
// A second goroutine decodes the file families every colony shows ahead
// (prewarmFiles, art_memory.go). The returned channel is closed when both
// are done (tests); both end on their own.
func (c *artCatalog) PrewarmAsync() <-chan struct{} {
	done := make(chan struct{})
	if c == nil {
		close(done)
		return done
	}
	var atlases []*spriteAtlas
	seen := map[*spriteAtlas]bool{}
	for _, sp := range c.Sprites {
		if sp.Src != nil && !seen[sp.Src.Atlas] {
			seen[sp.Src.Atlas] = true
			atlases = append(atlases, sp.Src.Atlas)
		}
	}
	files := c.prewarmFileKeys()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		var z deflater
		for _, a := range atlases {
			for pg := range a.ByPage {
				for layer := 0; layer < atlasLayers; layer++ {
					a.CutPageAside(pg, layer, &z)
				}
			}
		}
	}()
	go func() {
		defer wg.Done()
		c.prewarmFiles(files)
	}()
	go func() {
		wg.Wait()
		close(done)
	}()
	return done
}

// CutPageAside is cutPage for the background prewarm: a.mu is taken only
// to check and to publish.
func (a *spriteAtlas) CutPageAside(pg, layer int, z *deflater) {
	a.mu.Lock()
	done := a.Cut[pg][layer]
	a.mu.Unlock()
	if done {
		return
	}
	path := ""
	for _, o := range a.ByPage[pg] { // byPage and the sprites' paths never change after load
		if path = o.layerPath(layer); path != "" {
			break
		}
	}
	var copies [][]byte
	if path != "" {
		pd := decodeLayer(a.root, layer, path)
		copies = make([][]byte, len(a.ByPage[pg]))
		for i, o := range a.ByPage[pg] {
			if (layer == layerMask && o.maskPath == "") || (layer == layerNorm && !o.Entry.Normal) {
				continue
			}
			if raw := trimmedOf(pd, layer, o.Entry.Rect); raw != nil {
				copies[i] = z.deflate(raw)
			}
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.Cut[pg][layer] {
		return // a draw cut it first
	}
	a.Cut[pg][layer] = true
	if path == "" {
		return
	}
	a.Decodes++
	for i, o := range a.ByPage[pg] {
		if copies[i] != nil {
			o.Z[layer] = copies[i]
		}
	}
}

// deflater is a reusable flate compressor (a flate writer is ~1 MB of
// state).
type deflater struct {
	zw  *flate.Writer
	buf bytes.Buffer
}

// deflate compresses b at flate's fastest level.
func (d *deflater) deflate(b []byte) []byte {
	d.buf.Reset()
	if d.zw == nil {
		d.zw, _ = flate.NewWriter(&d.buf, flate.BestSpeed)
	} else {
		d.zw.Reset(&d.buf)
	}
	d.zw.Write(b)
	d.zw.Close()
	return bytes.Clone(d.buf.Bytes())
}

// decodePage decodes one layer of page n into the transient cache.
func (a *spriteAtlas) decodePage(n, layer int, path string) *pageData {
	pd := decodeLayer(a.root, layer, path)
	a.Cache[n] = pd
	a.Decodes++
	return pd
}

// decodeLayer decodes one page layer file under root.
func decodeLayer(root string, layer int, path string) *pageData {
	pd := &pageData{}
	full := filepath.Join(root, filepath.FromSlash(path))
	switch layer {
	case layerMask:
		pd.mask = decodeGray(full)
	case layerNorm:
		pd.norm, _ = decodeNRGBA(full)
	default:
		pd.pix, _ = decodeNRGBA(full)
	}
	return pd
}

// trimmedOf copies rect of a decoded page layer, nil when unreadable.
func trimmedOf(pd *pageData, layer int, r image.Rectangle) []byte {
	w, h := r.Dx(), r.Dy()
	switch layer {
	case layerMask:
		if pd.mask == nil {
			return nil
		}
		out := make([]byte, w*h)
		for y := 0; y < h; y++ {
			copy(out[y*w:], pd.mask.Pix[pd.mask.PixOffset(r.Min.X, r.Min.Y+y):][:w])
		}
		return out
	case layerNorm:
		if pd.norm == nil {
			return nil
		}
		out := make([]byte, 3*w*h)
		for y := 0; y < h; y++ {
			src := pd.norm.Pix[pd.norm.PixOffset(r.Min.X, r.Min.Y+y):][:4*w]
			dst := out[3*w*y:]
			for x := 0; x < w; x++ {
				dst[3*x], dst[3*x+1], dst[3*x+2] = src[4*x], src[4*x+1], src[4*x+2]
			}
		}
		return out
	}
	if pd.pix == nil {
		return nil
	}
	out := make([]byte, 4*w*h)
	for y := 0; y < h; y++ {
		copy(out[4*w*y:], pd.pix.Pix[pd.pix.PixOffset(r.Min.X, r.Min.Y+y):][:4*w])
	}
	return out
}

// deflateAll compresses b at flate's fastest level (a's writer and buffer
// are reused: a flate writer is ~1 MB of state).
func (a *spriteAtlas) deflateAll(b []byte) []byte { return a.def.deflate(b) }

// inflateAll expands a deflateAll copy of n bytes, nil when corrupt.
func (a *spriteAtlas) inflateAll(z []byte, n int) []byte {
	out := make([]byte, n)
	a.zsrc.Reset(z)
	if a.zr == nil {
		a.zr = flate.NewReader(&a.zsrc)
	} else {
		a.zr.(flate.Resetter).Reset(&a.zsrc, nil)
	}
	if _, err := io.ReadFull(a.zr, out); err != nil {
		return nil
	}
	return out
}

// decodeGray reads an 8-bit grey PNG (a tint-mask page), nil on failure.
func decodeGray(path string) *image.Gray {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return nil
	}
	if g, ok := src.(*image.Gray); ok {
		return g
	}
	g := image.NewGray(src.Bounds())
	draw.Draw(g, g.Bounds(), src, src.Bounds().Min, draw.Src)
	return g
}

// cutNRGBA places a trimmed NRGBA copy (nil: none) into a canvas-sized
// image at e.trim, the rest transparent black.
func cutNRGBA(trim []byte, e atlasEntry, canvas image.Point) *image.NRGBA {
	out := image.NewNRGBA(image.Rectangle{Max: canvas})
	if trim == nil {
		return out
	}
	w, h := e.Rect.Dx(), e.Rect.Dy()
	for y := 0; y < h; y++ {
		copy(out.Pix[out.PixOffset(e.Trim.X, e.Trim.Y+y):], trim[4*w*y:][:4*w])
	}
	return out
}

// cutMask expands a trimmed grey mask copy into the opaque NRGBA mask the
// individual file holds: R=G=B=grey, A=255 over the whole canvas.
func cutMask(trim []byte, e atlasEntry, canvas image.Point) *image.NRGBA {
	out := image.NewNRGBA(image.Rectangle{Max: canvas})
	for i := 3; i < len(out.Pix); i += 4 {
		out.Pix[i] = 255
	}
	if trim == nil {
		return out
	}
	w, h := e.Rect.Dx(), e.Rect.Dy()
	for y := 0; y < h; y++ {
		dst := out.Pix[out.PixOffset(e.Trim.X, e.Trim.Y+y):]
		for x, g := range trim[w*y:][:w] {
			dst[4*x], dst[4*x+1], dst[4*x+2] = g, g, g
		}
	}
	return out
}

// fill cuts sp's pixels and tint mask out of the atlas. The atlas files
// were checked when the catalog loaded; a page that has since become
// unreadable leaves the sprite blank rather than crashing a draw.
func (s *atlasSrc) fill(sp *artSprite) {
	canvas := image.Pt(sp.Meta.Canvas[0], sp.Meta.Canvas[1])
	sp.Pix = cutNRGBA(s.Atlas.take(s, layerPix), s.Entry, canvas)
	if sp.Meta.TintMask != "" {
		sp.Mask = cutMask(s.Atlas.take(s, layerMask), s.Entry, canvas)
	}
}

// NormalImage is the sprite's normal map cut out of the normal page, nil
// when it has none. The page holds RGB only; a normal map has its sprite's
// alpha, and is transparent flat-normal blue outside the trimmed rect.
func (s *atlasSrc) NormalImage(sp *artSprite) *image.NRGBA {
	if !s.Entry.Normal {
		return nil
	}
	if sp.Pix == nil {
		s.fill(sp)
	}
	trim := s.Atlas.take(s, layerNorm)
	if trim == nil {
		return nil
	}
	canvas := image.Pt(sp.Meta.Canvas[0], sp.Meta.Canvas[1])
	out := image.NewNRGBA(image.Rectangle{Max: canvas})
	for i := 0; i < len(out.Pix); i += 4 {
		out.Pix[i], out.Pix[i+1], out.Pix[i+2] = 128, 128, 255
	}
	w, h := s.Entry.Rect.Dx(), s.Entry.Rect.Dy()
	for y := 0; y < h; y++ {
		dst := out.Pix[out.PixOffset(s.Entry.Trim.X, s.Entry.Trim.Y+y):]
		src := trim[3*w*y:][:3*w]
		for x := 0; x < w; x++ {
			dst[4*x], dst[4*x+1], dst[4*x+2] = src[3*x], src[3*x+1], src[3*x+2]
		}
	}
	for i := 3; i < len(out.Pix); i += 4 {
		out.Pix[i] = sp.Pix.Pix[i]
	}
	return out
}

// loadAtlas registers every sprite of family from its atlas index
// metadata/atlas/<family>.yaml under root, after checking the pages exist
// and hold their sprites' rectangles.
func (c *artCatalog) loadAtlas(root, family string) error {
	path := filepath.Join(root, artMetadataRel, atlasMetaDir, family+".yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fam, pages, entries, err := parseSpriteAtlas(string(raw))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if fam != family {
		return fmt.Errorf("%s: atlas is for family %q", path, fam)
	}
	sizes := make([]image.Point, pages)
	for p := range sizes {
		f, err := os.Open(filepath.Join(root, "generated", "atlas", fmt.Sprintf("%s_%d.png", family, p)))
		if err != nil {
			return fmt.Errorf("atlas %s: %w; run make art", family, err)
		}
		cfg, err := png.DecodeConfig(f)
		f.Close()
		if err != nil {
			return fmt.Errorf("atlas %s page %d: %w", family, p, err)
		}
		sizes[p] = image.Pt(cfg.Width, cfg.Height)
	}
	a := &spriteAtlas{root: root, Cache: map[int]*pageData{}, ByPage: make([][]*atlasSrc, pages), Cut: make([][atlasLayers]bool, pages)}
	for _, e := range entries {
		m, err := parseArtMeta(e.Meta)
		if err != nil {
			return fmt.Errorf("atlas %s/%s: %w", family, e.Key, err)
		}
		if e.Page < 0 || e.Page >= pages || e.Rect.Empty() || !e.Rect.In(image.Rectangle{Max: sizes[e.Page]}) {
			return fmt.Errorf("atlas %s/%s: rect %v outside page %d", family, e.Key, e.Rect, e.Page)
		}
		if e.Trim.X+e.Rect.Dx() > m.Canvas[0] || e.Trim.Y+e.Rect.Dy() > m.Canvas[1] {
			return fmt.Errorf("atlas %s/%s: rect %v at %v exceeds canvas %v", family, e.Key, e.Rect, e.Trim, m.Canvas)
		}
		src := &atlasSrc{Atlas: a, Entry: e, pagePath: m.Image, maskPath: m.TintMask}
		a.ByPage[e.Page] = append(a.ByPage[e.Page], src)
		c.Sprites[family+"/"+e.Key] = &artSprite{Meta: m, Src: src}
	}
	return nil
}
