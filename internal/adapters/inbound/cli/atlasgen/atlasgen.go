package atlasgen

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"rimworld-go/pkg/flatyaml"
)

const gutter = 1 // pixels between packed sprites

// Main runs the tool with the command-line args and returns the exit code.
func Main(args []string, out, errw io.Writer) int {
	fs := flag.NewFlagSet("atlasgen", flag.ContinueOnError)
	fs.SetOutput(errw)
	family := fs.String("family", "pawn", "sprite family to pack (assets/metadata/<family>)")
	maxPx := fs.Int("max", 4096, "maximum page width and height in pixels")
	root := fs.String("root", "assets", "assets directory")
	check := fs.Bool("check", false, "validate the existing atlas instead of writing one")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	start := time.Now()
	if *check {
		errs := checkAtlas(*root, *family, *maxPx)
		for _, e := range errs {
			fmt.Fprintln(errw, "atlasgen:", e)
		}
		if len(errs) > 0 {
			return 1
		}
		fmt.Fprintf(out, "atlasgen: %s atlas ok\n", *family)
		return 0
	}
	res, err := pack(*root, *family, *maxPx)
	if err != nil {
		fmt.Fprintln(errw, "atlasgen:", err)
		return 1
	}
	fmt.Fprintf(out, "atlasgen: %s: %d sprites in %d pages (%d bytes of PNG) in %s\n", *family, res.sprites, res.pages,
		res.bytes, time.Since(start).Round(time.Millisecond))
	return 0
}

// entry is one sprite to pack.
type entry struct {
	key                string
	meta               []string // metadata lines to embed, unindented
	imagePath          string   // absolute paths
	maskPath, normPath string
	canvas             image.Point
	trim               image.Rectangle // opaque bounding box in canvas coordinates
	page               int
	x, y               int // trimmed rect origin in the page
}

type result struct {
	sprites, pages int
	bytes          int64
}

// pack writes the atlas for family under root.
func pack(root, family string, maxPx int) (result, error) {
	entries, err := collect(root, family)
	if err != nil {
		return result{}, err
	}
	if len(entries) == 0 {
		return result{}, fmt.Errorf("no sprites under %s", filepath.Join(root, "metadata", family))
	}
	if err := measure(entries); err != nil {
		return result{}, err
	}
	pageSizes, err := layout(entries, maxPx)
	if err != nil {
		return result{}, err
	}
	outDir := filepath.Join(root, "generated", "atlas")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return result{}, err
	}
	if err := removeStale(outDir, family); err != nil {
		return result{}, err
	}
	byPage := make([][]*entry, len(pageSizes))
	for _, e := range entries {
		byPage[e.page] = append(byPage[e.page], e)
	}
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		total int64
		first error
	)
	sem := make(chan struct{}, max(1, runtime.NumCPU()/2)) // pages are big: bound the memory
	for p := range byPage {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			n, err := writePage(outDir, family, p, pageSizes[p], byPage[p])
			mu.Lock()
			defer mu.Unlock()
			total += n
			if err != nil && first == nil {
				first = err
			}
		}()
	}
	wg.Wait()
	if first != nil {
		return result{}, first
	}
	yaml := atlasYAML(family, maxPx, len(pageSizes), entries)
	mp := filepath.Join(root, "metadata", "atlas", family+".yaml")
	if err := os.MkdirAll(filepath.Dir(mp), 0o755); err != nil {
		return result{}, err
	}
	if err := os.WriteFile(mp, []byte(yaml), 0o644); err != nil {
		return result{}, err
	}
	return result{sprites: len(entries), pages: len(pageSizes), bytes: total}, nil
}

// removeStale deletes the pages of an earlier, larger run.
func removeStale(dir, family string) error {
	for _, suffix := range []string{".png", "_mask.png", "_n.png"} {
		old, err := filepath.Glob(filepath.Join(dir, family+"_[0-9]*"+suffix))
		if err != nil {
			return err
		}
		for _, f := range old {
			if pageFileName(family, f, suffix) {
				if err := os.Remove(f); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// pageFileName reports whether path is exactly <family>_<digits><suffix>
// (the glob also matches <family>_0_mask.png for the ".png" suffix).
func pageFileName(family, path, suffix string) bool {
	n := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), family+"_"), suffix)
	_, err := strconv.Atoi(n)
	return err == nil
}

// collect lists the family's sprites, sorted by key, with their metadata.
func collect(root, family string) ([]*entry, error) {
	dir := filepath.Join(root, "metadata", family)
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []*entry
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			return nil, err
		}
		e, err := parseSpriteMeta(root, strings.TrimSuffix(f.Name(), ".yaml"), string(raw))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name(), err)
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out, nil
}

// parseSpriteMeta reads the fields atlasgen needs (image, canvas, tint
// mask) and keeps the rest verbatim.
func parseSpriteMeta(root, key, src string) (*entry, error) {
	e := &entry{key: key}
	inTint := false
	for l := range flatyaml.Lines(src) {
		if strings.HasPrefix(l.Text, "version:") {
			continue
		}
		nested, k, v := l.Indent > 0, l.Key, l.Value
		if !nested {
			inTint = k == "tint"
		}
		switch {
		case !nested && k == "image":
			e.imagePath = filepath.Join(root, filepath.FromSlash(v))
		case !nested && k == "canvas":
			c, err := flatyaml.Ints(v, 2)
			if err != nil {
				return nil, fmt.Errorf("canvas: %w", err)
			}
			e.canvas = image.Pt(c[0], c[1])
		case nested && inTint && k == "mask":
			e.maskPath = filepath.Join(root, filepath.FromSlash(v))
		}
		e.meta = append(e.meta, l.Text)
	}
	if e.imagePath == "" || e.canvas.X < 1 || e.canvas.Y < 1 {
		return nil, fmt.Errorf("metadata missing image/canvas")
	}
	if _, err := os.Stat(strings.TrimSuffix(e.imagePath, ".png") + "_n.png"); err == nil {
		e.normPath = strings.TrimSuffix(e.imagePath, ".png") + "_n.png"
	}
	return e, nil
}

func decode(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if n, ok := src.(*image.NRGBA); ok {
		return n, nil
	}
	dst := image.NewNRGBA(src.Bounds())
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst, nil
}

// measure decodes every sprite (in parallel) to find the bounding box of
// its non-zero pixels across the sprite and its mask. Everything outside
// that box is transparent (and a zero mask), so trimming loses nothing; the
// normal map shares the sprite's alpha, so it needs no box of its own.
func measure(entries []*entry) error {
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		first error
		next  int
	)
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				i := next
				next++
				failed := first != nil
				mu.Unlock()
				if i >= len(entries) || failed {
					return
				}
				if err := entries[i].measure(); err != nil {
					mu.Lock()
					if first == nil {
						first = err
					}
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	return first
}

func (e *entry) measure() error {
	img, mask, _, err := e.load()
	if err != nil {
		return err
	}
	box := image.Rectangle{}
	grow := func(r image.Rectangle) { box = box.Union(r) }
	grow(nonZero(img, 0xf))
	if mask != nil {
		grow(nonZero(mask, 0x1)) // red channel only
	}
	if box.Empty() {
		box = image.Rect(0, 0, 1, 1) // a blank sprite still needs a rect
	}
	e.trim = box
	return nil
}

// load decodes the sprite and its companions and checks their sizes. The
// mask must be grey (R=G=B) and opaque, which is how the Blender pipeline
// writes it, so the atlas can store its red channel alone.
func (e *entry) load() (img, mask, norm *image.NRGBA, err error) {
	if img, err = decode(e.imagePath); err != nil {
		return nil, nil, nil, err
	}
	if img.Bounds().Size() != e.canvas {
		return nil, nil, nil, fmt.Errorf("%s: png %v != canvas %v", e.key, img.Bounds().Size(), e.canvas)
	}
	if e.maskPath != "" {
		if mask, err = decode(e.maskPath); err != nil {
			return nil, nil, nil, err
		}
		if mask.Bounds().Size() != e.canvas {
			return nil, nil, nil, fmt.Errorf("%s: mask %v != canvas %v", e.key, mask.Bounds().Size(), e.canvas)
		}
		for i := 0; i < len(mask.Pix); i += 4 {
			if mask.Pix[i] != mask.Pix[i+1] || mask.Pix[i] != mask.Pix[i+2] || mask.Pix[i+3] != 255 {
				return nil, nil, nil, fmt.Errorf("%s: tint mask is not an opaque grey image", e.key)
			}
		}
	}
	if e.normPath != "" {
		if norm, err = decode(e.normPath); err != nil {
			return nil, nil, nil, err
		}
		if norm.Bounds().Size() != e.canvas {
			return nil, nil, nil, fmt.Errorf("%s: normal map %v != canvas %v", e.key, norm.Bounds().Size(), e.canvas)
		}
		for i := 3; i < len(norm.Pix); i += 4 {
			if norm.Pix[i] != img.Pix[i] {
				return nil, nil, nil, fmt.Errorf("%s: normal map alpha differs from the sprite (make normals)", e.key)
			}
		}
	}
	return img, mask, norm, nil
}

// nonZero is the bounding box of pixels with a non-zero channel selected
// by chans (bit 0 = R ... bit 3 = A).
func nonZero(img *image.NRGBA, chans int) image.Rectangle {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X-1, b.Min.Y-1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := img.Pix[img.PixOffset(b.Min.X, y):]
		for x := 0; x < b.Dx(); x++ {
			hit := false
			for c := 0; c < 4; c++ {
				if chans&(1<<c) != 0 && row[4*x+c] != 0 {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
			minX, maxX = min(minX, b.Min.X+x), max(maxX, b.Min.X+x)
			minY, maxY = min(minY, y), max(maxY, y)
		}
	}
	if maxX < minX {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

// layout shelf-packs the trimmed sprites in key order and returns the
// size of every page. A shelf is a row; a sprite joins the current shelf
// when it fits, taller sprites grow the last shelf while the page has
// room, else a new shelf (and, when the page is full, a new page) starts.
func layout(entries []*entry, maxPx int) ([][2]int, error) {
	var (
		sizes            [][2]int
		x, y, shelfH, pw int // current cursor; pw is the widest shelf so far
	)
	newPage := func() { sizes = append(sizes, [2]int{}); x, y, shelfH, pw = 0, 0, 0, 0 }
	newPage()
	for _, e := range entries {
		w, h := e.trim.Dx(), e.trim.Dy()
		if w > maxPx || h > maxPx {
			return nil, fmt.Errorf("%s: trimmed sprite %dx%d exceeds the %d px page", e.key, w, h, maxPx)
		}
		switch {
		case x+w <= maxPx && y+max(shelfH, h) <= maxPx:
			shelfH = max(shelfH, h) // fits the shelf (it may grow the last one)
		case y+shelfH+gutter+h <= maxPx:
			y += shelfH + gutter
			x, shelfH = 0, h
		default:
			newPage()
			shelfH = h
		}
		e.page, e.x, e.y = len(sizes)-1, x, y
		x += w + gutter
		pw = max(pw, x-gutter)
		sizes[len(sizes)-1] = [2]int{pw, y + shelfH}
	}
	return sizes, nil
}

// writePage composes and writes the three images of one page.
func writePage(dir, family string, n int, size [2]int, es []*entry) (int64, error) {
	rect := image.Rect(0, 0, size[0], size[1])
	pix := image.NewNRGBA(rect)
	var (
		mask *image.Gray
		norm *image.NRGBA
	)
	for _, e := range es {
		img, m, nm, err := e.load()
		if err != nil {
			return 0, err
		}
		dst := image.Rect(e.x, e.y, e.x+e.trim.Dx(), e.y+e.trim.Dy())
		draw.Draw(pix, dst, img, e.trim.Min, draw.Src)
		if m != nil {
			if mask == nil {
				mask = image.NewGray(rect)
			}
			for y := 0; y < dst.Dy(); y++ {
				for x := 0; x < dst.Dx(); x++ {
					mask.Pix[mask.PixOffset(dst.Min.X+x, dst.Min.Y+y)] = m.Pix[m.PixOffset(e.trim.Min.X+x, e.trim.Min.Y+y)]
				}
			}
		}
		if nm != nil {
			if norm == nil {
				norm = image.NewNRGBA(rect)
				for i := 0; i < len(norm.Pix); i += 4 { // flat normal where no sprite is
					norm.Pix[i], norm.Pix[i+1], norm.Pix[i+2] = 128, 128, 255
				}
			}
			draw.Draw(norm, dst, nm, e.trim.Min, draw.Src)
		}
	}
	var total int64
	write := func(suffix string, img image.Image) error {
		sz, err := writePNG(filepath.Join(dir, fmt.Sprintf("%s_%d%s.png", family, n, suffix)), img)
		total += sz
		return err
	}
	if err := write("", pix); err != nil {
		return total, err
	}
	if mask != nil {
		if err := write("_mask", mask); err != nil {
			return total, err
		}
	}
	if norm != nil {
		for i := 3; i < len(norm.Pix); i += 4 {
			norm.Pix[i] = 255 // RGB only: the alpha is the sprite's
		}
		if err := write("_n", norm); err != nil {
			return total, err
		}
	}
	return total, nil
}

func writePNG(path string, img image.Image) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(f, img); err != nil {
		f.Close()
		return 0, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return 0, err
	}
	return st.Size(), f.Close()
}

// atlasYAML renders the atlas metadata.
func atlasYAML(family string, maxPx, pages int, entries []*entry) string {
	var b strings.Builder
	b.WriteString("# generated by cmd/atlasgen - do not hand-edit\n")
	fmt.Fprintf(&b, "version: 1\nfamily: %s\nmax: %d\npages: %d\nsprites:\n", family, maxPx, pages)
	for _, e := range entries {
		fmt.Fprintf(&b, "  %s:\n", e.key)
		fmt.Fprintf(&b, "    page: %d\n", e.page)
		fmt.Fprintf(&b, "    rect: [%d, %d, %d, %d]\n", e.x, e.y, e.trim.Dx(), e.trim.Dy())
		fmt.Fprintf(&b, "    trim: [%d, %d]\n", e.trim.Min.X, e.trim.Min.Y)
		fmt.Fprintf(&b, "    normal: %t\n", e.normPath != "")
		inTint := false
		for _, line := range e.meta {
			trim := strings.TrimSpace(line)
			nested := strings.HasPrefix(line, " ")
			if !nested {
				inTint = strings.HasPrefix(trim, "tint:")
			}
			switch {
			case !nested && strings.HasPrefix(trim, "image:"):
				line = fmt.Sprintf("image: generated/atlas/%s_%d.png", family, e.page)
			case nested && inTint && strings.HasPrefix(trim, "mask:"):
				line = fmt.Sprintf("  mask: generated/atlas/%s_%d_mask.png", family, e.page)
			}
			b.WriteString("    " + line + "\n")
		}
	}
	return b.String()
}

// ---- reading an atlas back (-check and tests) ---------------------------

// atlasSprite is one parsed atlas entry.
type atlasSprite struct {
	key    string
	page   int
	rect   image.Rectangle
	trim   image.Point
	normal bool
	mask   bool // the sprite has a tint mask
	canvas image.Point
}

type atlasIndex struct {
	family string
	max    int
	pages  int
	list   []atlasSprite
}

func parseAtlas(src string) (atlasIndex, error) {
	var idx atlasIndex
	var cur *atlasSprite
	flush := func() {
		if cur != nil {
			idx.list = append(idx.list, *cur)
		}
	}
	inSprites, inTint := false, false
	for l := range flatyaml.Lines(src) {
		indent, k, v := l.Indent, l.Key, l.Value
		var err error
		switch {
		case indent == 0 && !inSprites:
			switch k {
			case "family":
				idx.family = v
			case "max":
				idx.max, err = strconv.Atoi(v)
			case "pages":
				idx.pages, err = strconv.Atoi(v)
			case "sprites":
				inSprites = true
			}
		case inSprites && indent == 2:
			flush()
			cur = &atlasSprite{key: k}
			inTint = false
		case inSprites && cur != nil && indent == 4:
			inTint = k == "tint"
			switch k {
			case "page":
				cur.page, err = strconv.Atoi(v)
			case "rect":
				var r [4]int
				r, err = flatyaml.Ints(v, 4)
				cur.rect = image.Rect(r[0], r[1], r[0]+r[2], r[1]+r[3])
			case "trim":
				var r [4]int
				r, err = flatyaml.Ints(v, 2)
				cur.trim = image.Pt(r[0], r[1])
			case "normal":
				cur.normal = v == "true"
			case "canvas":
				var r [4]int
				r, err = flatyaml.Ints(v, 2)
				cur.canvas = image.Pt(r[0], r[1])
			}
		case inSprites && cur != nil && indent == 6 && inTint && k == "mask":
			cur.mask = true
		}
		if err != nil {
			return idx, fmt.Errorf("line %d: %s: %w", l.Number, k, err)
		}
	}
	flush()
	return idx, nil
}

// checkAtlas validates the written atlas of family: its pages exist and
// are within maxPx, every rectangle lies inside its page and none overlap,
// every mask/normal page is present, and every sprite that still has
// metadata under metadata/<family> is in the atlas.
func checkAtlas(root, family string, maxPx int) []error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	raw, err := os.ReadFile(filepath.Join(root, "metadata", "atlas", family+".yaml"))
	if err != nil {
		return []error{err}
	}
	idx, err := parseAtlas(string(raw))
	if err != nil {
		return []error{err}
	}
	if idx.family != family {
		fail("atlas is for family %q, want %q", idx.family, family)
	}
	if idx.max > maxPx {
		fail("atlas was packed at max %d, above the allowed %d", idx.max, maxPx)
	}
	dir := filepath.Join(root, "generated", "atlas")
	sizes := make([]image.Point, idx.pages)
	need := map[string]bool{} // page files some sprite needs
	for p := 0; p < idx.pages; p++ {
		sz, err := imageSize(filepath.Join(dir, fmt.Sprintf("%s_%d.png", family, p)))
		if err != nil {
			fail("page %d: %v", p, err)
			continue
		}
		if sz.X > maxPx || sz.Y > maxPx {
			fail("page %d is %v, above %d", p, sz, maxPx)
		}
		sizes[p] = sz
	}
	byPage := map[int][]atlasSprite{}
	seen := map[string]bool{}
	for _, s := range idx.list {
		if seen[s.key] {
			fail("%s: listed twice", s.key)
		}
		seen[s.key] = true
		if s.page < 0 || s.page >= idx.pages {
			fail("%s: page %d out of range", s.key, s.page)
			continue
		}
		pr := image.Rectangle{Max: sizes[s.page]}
		if s.rect.Empty() || !s.rect.In(pr) {
			fail("%s: rect %v outside page %d (%v)", s.key, s.rect, s.page, pr)
		}
		if s.canvas.X > 0 && (s.trim.X < 0 || s.trim.Y < 0 || s.trim.X+s.rect.Dx() > s.canvas.X || s.trim.Y+s.rect.Dy() > s.canvas.Y) {
			fail("%s: trim %v + %v exceeds canvas %v", s.key, s.trim, s.rect.Size(), s.canvas)
		}
		if s.mask {
			need[fmt.Sprintf("%s_%d_mask.png", family, s.page)] = true
		}
		if s.normal {
			need[fmt.Sprintf("%s_%d_n.png", family, s.page)] = true
		}
		byPage[s.page] = append(byPage[s.page], s)
	}
	for f := range need {
		sz, err := imageSize(filepath.Join(dir, f))
		if err != nil {
			fail("%s: %v", f, err)
			continue
		}
		var p int
		_, _ = fmt.Sscanf(strings.TrimPrefix(f, family+"_"), "%d", &p)
		if sz != sizes[p] {
			fail("%s is %v, but its sprite page is %v", f, sz, sizes[p])
		}
	}
	for p, list := range byPage {
		sort.Slice(list, func(i, j int) bool { return list[i].key < list[j].key })
		for i := range list {
			for j := i + 1; j < len(list); j++ {
				if list[i].rect.Overlaps(list[j].rect) {
					fail("page %d: %s and %s overlap", p, list[i].key, list[j].key)
				}
			}
		}
	}
	if files, err := os.ReadDir(filepath.Join(root, "metadata", family)); err == nil {
		for _, f := range files {
			if k, ok := strings.CutSuffix(f.Name(), ".yaml"); ok && !f.IsDir() && !seen[k] {
				fail("%s: sprite metadata exists but the key is not in the atlas (run atlasgen)", k)
			}
		}
	}
	return errs
}

func imageSize(path string) (image.Point, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Point{}, err
	}
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	return image.Pt(cfg.Width, cfg.Height), err
}
