// Command normalgen writes a normal map <name>_n.png next to every world
// sprite under the given directories (ART §6.1), with pkg/spritenormals,
// a port of spritefusion-sprite-normals (MIT, (c) 2026 Hugo Duprez); see
// THIRD_PARTY_NOTICES.md. Tint masks, normal maps and UI art are skipped,
// and a sprite whose normal map is newer is left alone unless -force.
// Autotiled sprites (metadata `autotile` + `mask`) are clipped flush where
// they join a neighbour; their silhouette is extended past those edges
// before the bevel is computed, so joined tiles light as one surface.
//
//	normalgen [-dirs assets/generated/world] [-force]
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"rimworld-go/pkg/spritenormals"
)

func main() {
	dirs := flag.String("dirs", "assets/generated/world", "comma-separated sprite directories")
	force := flag.Bool("force", false, "regenerate every normal map")
	flag.Parse()
	start := time.Now()
	n, skipped, err := run(strings.Split(*dirs, ","), *force)
	if err != nil {
		fmt.Fprintln(os.Stderr, "normalgen:", err)
		os.Exit(1)
	}
	fmt.Printf("normalgen: %d normal maps written, %d up to date, in %s\n", n, skipped, time.Since(start).Round(time.Millisecond))
}

// settings is spritefusion's default scaled for our @2x art: radius,
// softness and detail scale doubled (ART §6.1).
func settings() spritenormals.Settings {
	s := spritenormals.DefaultSettings()
	s.ShapeRadius, s.ShapeSoftness, s.DetailScale = 8, 2.3, 1.2
	return s
}

// wanted reports a sprite PNG that needs a normal map. Terrain, floors and
// the flat ground decals (snow cover, frost) lie flat on the ground and
// are lit flat (ART §6.1), so they get none; a bevel dome made them read
// as gel blobs (PLAY-022).
func wanted(path string) bool {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".png") || strings.HasSuffix(base, "_mask.png") || strings.HasSuffix(base, "_n.png") {
		return false
	}
	p := filepath.ToSlash(path)
	if strings.Contains(p, "/structure/") && strings.HasPrefix(base, "floor") {
		return false
	}
	if strings.Contains(p, "/weather/") && (strings.HasPrefix(base, "snow_cover") || strings.HasPrefix(base, "frost")) {
		return false
	}
	return !strings.Contains(p, "/ui/") && !strings.Contains(p, "/terrain/")
}

// run collects the sprites that need a map, then writes them on a
// bounded worker pool; each map depends only on its own sprite, so the
// output is identical whatever the order.
func run(dirs []string, force bool) (written, skipped int, err error) {
	type job struct{ in, out string }
	var jobs []job
	for _, dir := range dirs {
		err = filepath.WalkDir(strings.TrimSpace(dir), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !wanted(path) {
				return err
			}
			out := strings.TrimSuffix(path, ".png") + "_n.png"
			if !force && newer(out, path) {
				skipped++
				return nil
			}
			jobs = append(jobs, job{path, out})
			return nil
		})
		if err != nil {
			return 0, skipped, err
		}
	}
	s := settings()
	queue := make(chan job)
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range queue {
				err := one(j.in, j.out, s)
				mu.Lock()
				if err != nil {
					if firstErr == nil {
						firstErr = fmt.Errorf("%s: %w", j.in, err)
					}
				} else {
					written++
				}
				mu.Unlock()
			}
		}()
	}
	for _, j := range jobs {
		queue <- j
	}
	close(queue)
	wg.Wait()
	return written, skipped, firstErr
}

// newer reports a exists and is at least as new as b.
func newer(a, b string) bool {
	sa, err := os.Stat(a)
	if err != nil {
		return false
	}
	sb, err := os.Stat(b)
	return err == nil && !sa.ModTime().Before(sb.ModTime())
}

func one(in, out string, s spritenormals.Settings) error {
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	src, err := png.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	nrgba, ok := src.(*image.NRGBA)
	if !ok {
		nrgba = image.NewNRGBA(src.Bounds())
		draw.Draw(nrgba, nrgba.Bounds(), src, src.Bounds().Min, draw.Src)
	}
	meta := metaOf(in)
	var n *image.NRGBA
	if flatNormal(meta) {
		n = flatNormals(nrgba)
	} else if n, err = normalsFor(nrgba, linkedMask(meta), s); err != nil {
		return err
	}
	w, err := os.Create(out)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(w, n); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

// Autotile mask bits (ART spec / assets/AGENTS.md).
const (
	linkN = 1
	linkE = 2
	linkS = 4
	linkW = 8
)

// metaOf reads a sprite's metadata sidecar (generated/world/<fam>/<name>.png
// -> metadata/<fam>/<name>.yaml); empty when there is none.
func metaOf(png string) string {
	p := filepath.ToSlash(png)
	i := strings.LastIndex(p, "/generated/world/")
	if i < 0 {
		return ""
	}
	meta := p[:i] + "/metadata/" + strings.TrimSuffix(p[i+len("/generated/world/"):], ".png") + ".yaml"
	src, err := os.ReadFile(filepath.FromSlash(meta))
	if err != nil {
		return ""
	}
	return string(src)
}

// flatNormal reports a top-level `flat_normal: true` in sprite metadata:
// low cover (grass, moss) is lit flat so it can be mirrored freely.
func flatNormal(meta string) bool {
	for _, line := range strings.Split(meta, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok || k != "flat_normal" {
			continue
		}
		v, _, _ = strings.Cut(v, "#")
		return strings.TrimSpace(v) == "true"
	}
	return false
}

// flatNormals is a normal map pointing straight at the viewer (0,0,1)
// wherever src has alpha, keeping src's alpha.
func flatNormals(src *image.NRGBA) *image.NRGBA {
	b := src.Bounds()
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if a := src.NRGBAAt(x, y).A; a > 0 {
				out.SetNRGBA(x, y, color.NRGBA{128, 128, 255, a})
			}
		}
	}
	return out
}

// linkedMask is the top-level `mask:` of an `autotile:` sprite, else -1.
func linkedMask(meta string) int {
	auto, mask := false, -1
	for _, line := range strings.Split(meta, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok || strings.HasPrefix(line, " ") {
			continue
		}
		switch k {
		case "autotile":
			auto = true
		case "mask":
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				mask = n
			}
		}
	}
	if !auto {
		return -1
	}
	return mask
}

// linkPad is how far a linked edge is extended: well past the bevel
// radius, so the dome is flat where the tile meets its neighbour.
const linkPad = 32

// normalsFor makes the normal map of src. For an autotiled sprite (mask
// >= 0) every linked edge's outermost opaque column/row is replicated
// outward first, and the result cropped back to src's bounds.
func normalsFor(src *image.NRGBA, mask int, s spritenormals.Settings) (*image.NRGBA, error) {
	if mask <= 0 {
		return spritenormals.Generate(src, s)
	}
	b := src.Bounds()
	pad := image.NewNRGBA(image.Rect(0, 0, b.Dx()+2*linkPad, b.Dy()+2*linkPad))
	draw.Draw(pad, image.Rect(linkPad, linkPad, linkPad+b.Dx(), linkPad+b.Dy()), src, b.Min, draw.Src)
	extendLinked(pad, mask)
	n, err := spritenormals.Generate(pad, s)
	if err != nil {
		return nil, err
	}
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), n, image.Pt(linkPad, linkPad), draw.Src)
	// the extension only shaped the bevel: keep the sprite's own alpha
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			a := src.NRGBAAt(b.Min.X+x, b.Min.Y+y).A
			c := out.NRGBAAt(x, y)
			if a == 0 {
				c = color.NRGBA{}
			}
			c.A = a
			out.SetNRGBA(x, y, c)
		}
	}
	return out, nil
}

// extendLinked copies the outermost opaque column (west/east) or row
// (north/south) of img outward to the image edge for each linked side;
// columns first, so rows then carry the widened shape to the corners.
func extendLinked(img *image.NRGBA, mask int) {
	b := img.Bounds()
	opaque := func(x, y int) bool { return img.NRGBAAt(x, y).A > 8 }
	colHas := func(x int) bool {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			if opaque(x, y) {
				return true
			}
		}
		return false
	}
	rowHas := func(y int) bool {
		for x := b.Min.X; x < b.Max.X; x++ {
			if opaque(x, y) {
				return true
			}
		}
		return false
	}
	copyCol := func(from, to int) {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			img.SetNRGBA(to, y, img.NRGBAAt(from, y))
		}
	}
	copyRow := func(from, to int) {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.SetNRGBA(x, to, img.NRGBAAt(x, from))
		}
	}
	if mask&linkW != 0 {
		x0 := b.Min.X
		for x0 < b.Max.X && !colHas(x0) {
			x0++
		}
		for x := b.Min.X; x < x0 && x0 < b.Max.X; x++ {
			copyCol(x0, x)
		}
	}
	if mask&linkE != 0 {
		x1 := b.Max.X - 1
		for x1 >= b.Min.X && !colHas(x1) {
			x1--
		}
		for x := x1 + 1; x < b.Max.X && x1 >= b.Min.X; x++ {
			copyCol(x1, x)
		}
	}
	if mask&linkN != 0 {
		y0 := b.Min.Y
		for y0 < b.Max.Y && !rowHas(y0) {
			y0++
		}
		for y := b.Min.Y; y < y0 && y0 < b.Max.Y; y++ {
			copyRow(y0, y)
		}
	}
	if mask&linkS != 0 {
		y1 := b.Max.Y - 1
		for y1 >= b.Min.Y && !rowHas(y1) {
			y1--
		}
		for y := y1 + 1; y < b.Max.Y && y1 >= b.Min.Y; y++ {
			copyRow(y1, y)
		}
	}
}
