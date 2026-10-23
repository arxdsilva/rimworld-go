package ebiten

import (
	"bufio"
	"fmt"
	"image"
	"image/draw"
	"image/png"
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
// unpacked, so decoding them all at start would be the cost this avoids)
// and a few stay cached. A sprite is cut out of its page into a canvas-
// sized image once, the first time the catalog hands it out, so tinting,
// measuring and normal maps work on it as on a file-backed sprite.

// atlasMetaDir is the metadata directory of atlas indexes, not a sprite family.
const atlasMetaDir = "atlas"

// atlasPageCache is how many decoded pages stay in memory.
const atlasPageCache = 4

// atlasEntry is one parsed atlas sprite.
type atlasEntry struct {
	key    string
	page   int
	rect   image.Rectangle // trimmed pixels in the page
	trim   image.Point     // where rect sits in the sprite canvas
	normal bool
	meta   string // the sprite's own metadata, as in its individual yaml
}

// parseSpriteAtlas reads metadata/atlas/<family>.yaml: the family name,
// the page count and every sprite.
func parseSpriteAtlas(src string) (family string, pages int, out []atlasEntry, err error) {
	var cur *atlasEntry
	var meta strings.Builder
	flush := func() {
		if cur != nil {
			cur.meta = meta.String()
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
			cur = &atlasEntry{key: k}
		case inSprites && cur != nil && indent >= 4:
			if indent == 4 {
				switch k {
				case "page":
					if cur.page, err = strconv.Atoi(v); err != nil {
						return "", 0, nil, fmt.Errorf("atlas %s: page: %w", cur.key, err)
					}
					continue
				case "rect":
					var r [4]int
					if r, err = atlasInts(v, 4); err != nil {
						return "", 0, nil, fmt.Errorf("atlas %s: rect: %w", cur.key, err)
					}
					cur.rect = image.Rect(r[0], r[1], r[0]+r[2], r[1]+r[3])
					continue
				case "trim":
					var r [4]int
					if r, err = atlasInts(v, 2); err != nil {
						return "", 0, nil, fmt.Errorf("atlas %s: trim: %w", cur.key, err)
					}
					cur.trim = image.Pt(r[0], r[1])
					continue
				case "normal":
					cur.normal = v == "true"
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

// pageData is one decoded page: sprite pixels, tint masks (grey) and
// normal maps, each decoded when first asked for.
type pageData struct {
	pix, norm *image.NRGBA
	mask      *image.Gray
	hasMask   bool
	hasNorm   bool
}

// spriteAtlas serves the sprites of one packed family.
type spriteAtlas struct {
	root string
	mu   sync.Mutex
	// cache holds decoded pages, oldest first.
	cache map[int]*pageData
	order []int
}

// atlasSrc ties a sprite to its place in an atlas.
type atlasSrc struct {
	atlas *spriteAtlas
	entry atlasEntry
	// page file paths (from the sprite metadata)
	pagePath, maskPath string
}

func (a *spriteAtlas) page(n int) *pageData {
	if pd, ok := a.cache[n]; ok {
		for i, p := range a.order { // most recently used last
			if p == n {
				a.order = append(append(a.order[:i:i], a.order[i+1:]...), n)
				break
			}
		}
		return pd
	}
	pd := &pageData{}
	a.cache[n] = pd
	a.order = append(a.order, n)
	if len(a.order) > atlasPageCache {
		delete(a.cache, a.order[0])
		a.order = a.order[1:]
	}
	return pd
}

// pixels returns the page's sprite pixels (nil when unreadable).
func (a *spriteAtlas) pixels(n int, path string) *image.NRGBA {
	a.mu.Lock()
	defer a.mu.Unlock()
	pd := a.page(n)
	if pd.pix == nil {
		if img, err := decodeNRGBA(filepath.Join(a.root, filepath.FromSlash(path))); err == nil {
			pd.pix = img
		}
	}
	return pd.pix
}

func (a *spriteAtlas) masks(n int, path string) *image.Gray {
	a.mu.Lock()
	defer a.mu.Unlock()
	pd := a.page(n)
	if !pd.hasMask {
		pd.hasMask = true
		pd.mask = decodeGray(filepath.Join(a.root, filepath.FromSlash(path)))
	}
	return pd.mask
}

func (a *spriteAtlas) normals(n int, path string) *image.NRGBA {
	a.mu.Lock()
	defer a.mu.Unlock()
	pd := a.page(n)
	if !pd.hasNorm {
		pd.hasNorm = true
		if img, err := decodeNRGBA(filepath.Join(a.root, filepath.FromSlash(path))); err == nil {
			pd.norm = img
		}
	}
	return pd.norm
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

// cutNRGBA copies the trimmed rect of page into a canvas-sized image at
// trim, the rest transparent black, or transparent flat-normal blue when
// flat is set.
func cutNRGBA(page *image.NRGBA, e atlasEntry, canvas image.Point, flat bool) *image.NRGBA {
	out := image.NewNRGBA(image.Rectangle{Max: canvas})
	if flat {
		for i := 0; i < len(out.Pix); i += 4 {
			out.Pix[i], out.Pix[i+1], out.Pix[i+2] = 128, 128, 255
		}
	}
	if page == nil {
		return out
	}
	w, h := e.rect.Dx(), e.rect.Dy()
	for y := 0; y < h; y++ {
		src := page.Pix[page.PixOffset(e.rect.Min.X, e.rect.Min.Y+y):][:4*w]
		copy(out.Pix[out.PixOffset(e.trim.X, e.trim.Y+y):], src)
	}
	return out
}

// cutMask expands a grey mask page rect into the opaque NRGBA mask the
// individual file holds: R=G=B=grey, A=255 over the whole canvas.
func cutMask(page *image.Gray, e atlasEntry, canvas image.Point) *image.NRGBA {
	out := image.NewNRGBA(image.Rectangle{Max: canvas})
	for i := 3; i < len(out.Pix); i += 4 {
		out.Pix[i] = 255
	}
	if page == nil {
		return out
	}
	w, h := e.rect.Dx(), e.rect.Dy()
	for y := 0; y < h; y++ {
		src := page.Pix[page.PixOffset(e.rect.Min.X, e.rect.Min.Y+y):][:w]
		dst := out.Pix[out.PixOffset(e.trim.X, e.trim.Y+y):]
		for x, g := range src {
			dst[4*x], dst[4*x+1], dst[4*x+2] = g, g, g
		}
	}
	return out
}

// fill cuts sp's pixels and tint mask out of the atlas. The atlas files
// were checked when the catalog loaded; a page that has since become
// unreadable leaves the sprite blank rather than crashing a draw.
func (s *atlasSrc) fill(sp *artSprite) {
	canvas := image.Pt(sp.meta.Canvas[0], sp.meta.Canvas[1])
	sp.pix = cutNRGBA(s.atlas.pixels(s.entry.page, s.pagePath), s.entry, canvas, false)
	if sp.meta.TintMask != "" {
		sp.mask = cutMask(s.atlas.masks(s.entry.page, s.maskPath), s.entry, canvas)
	}
}

// normalImage is the sprite's normal map cut out of the normal page, nil
// when it has none. The page holds RGB only; a normal map has its sprite's
// alpha.
func (s *atlasSrc) normalImage(sp *artSprite) *image.NRGBA {
	if !s.entry.normal {
		return nil
	}
	page := s.atlas.normals(s.entry.page, strings.TrimSuffix(s.pagePath, ".png")+"_n.png")
	if page == nil {
		return nil
	}
	if sp.pix == nil {
		s.fill(sp)
	}
	out := cutNRGBA(page, s.entry, image.Pt(sp.meta.Canvas[0], sp.meta.Canvas[1]), true)
	for i := 3; i < len(out.Pix); i += 4 {
		out.Pix[i] = sp.pix.Pix[i]
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
	a := &spriteAtlas{root: root, cache: map[int]*pageData{}}
	for _, e := range entries {
		m, err := parseArtMeta(e.meta)
		if err != nil {
			return fmt.Errorf("atlas %s/%s: %w", family, e.key, err)
		}
		if e.page < 0 || e.page >= pages || e.rect.Empty() || !e.rect.In(image.Rectangle{Max: sizes[e.page]}) {
			return fmt.Errorf("atlas %s/%s: rect %v outside page %d", family, e.key, e.rect, e.page)
		}
		if e.trim.X+e.rect.Dx() > m.Canvas[0] || e.trim.Y+e.rect.Dy() > m.Canvas[1] {
			return fmt.Errorf("atlas %s/%s: rect %v at %v exceeds canvas %v", family, e.key, e.rect, e.trim, m.Canvas)
		}
		sp := &artSprite{meta: m, src: &atlasSrc{atlas: a, entry: e, pagePath: m.Image, maskPath: m.TintMask}}
		c.sprites[family+"/"+e.key] = sp
	}
	return nil
}
