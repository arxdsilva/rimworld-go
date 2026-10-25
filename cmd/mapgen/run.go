package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rimworld-go/cmd/mapgen/render"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/mapgen/pipeline"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/domain/world/site"
)

// StepTime is one pipeline step's wall time.
type StepTime struct {
	Step string  `json:"step"`
	Ms   float64 `json:"ms"`
}

// MetricsFile is metrics.json.
type MetricsFile struct {
	Seed    uint64         `json:"seed"`
	Size    int            `json:"size"`
	Biome   string         `json:"biome"`
	Hills   string         `json:"hills"`
	River   string         `json:"river"`
	Lakes   int            `json:"lakes"`
	Caves   bool           `json:"caves"`
	Tile    string         `json:"tile,omitempty"`
	Metrics mapgen.Metrics `json:"metrics"`
	Timings []StepTime     `json:"timings"`
	TotalMs float64        `json:"total_ms"`
}

// generate runs the pipeline with Debug on, timing each step.
func generate(seed uint64, size int, s site.Site) (mapgen.Result, []StepTime, error) {
	b, err := pipeline.NewBuilder(mapgen.Config{Seed: seed, W: size, H: size, Site: s,
		Biome: biome.DefOf(s.Biome), Debug: true})
	if err != nil {
		return mapgen.Result{}, nil, err
	}
	names := pipeline.StepNames()
	var times []StepTime
	for i := 0; ; i++ {
		t0 := time.Now()
		_, _, done, err := b.Step()
		if err != nil {
			return mapgen.Result{}, nil, err
		}
		if i < len(names) {
			times = append(times, StepTime{names[i], float64(time.Since(t0).Microseconds()) / 1000})
		}
		if done {
			return b.Result(), times, nil
		}
	}
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// tileSite builds the planet the game builds for a seed text and returns
// tile's Site.
func tileSite(spec string) (site.Site, error) {
	text, tile, err := parseTile(spec)
	if err != nil {
		return site.Site{}, err
	}
	b := planet.NewBuilder(planet.Params{SeedText: text, Coverage: 30, Rain: 2, Temp: 2, Pop: 1,
		Factions: faction.DefaultSlots()})
	for {
		if _, _, done := b.Step(1 << 16); done {
			break
		}
	}
	p := b.Planet()
	if tile >= p.TileCount() {
		return site.Site{}, fmt.Errorf("tile %d out of range (planet has %d tiles)", tile, p.TileCount())
	}
	return p.Site(tile), nil
}

func riverLabel(r site.RiverPath) string {
	if r.Size == site.NoRiver {
		return ""
	}
	return fmt.Sprintf("%s %s>%s", r.Size.Label(), dirNames[r.In%8], dirNames[r.Out%8])
}

// Run executes the options, writing files under o.Out and a summary to w.
func Run(o Options, w io.Writer) error {
	if err := os.MkdirAll(o.Out, 0o755); err != nil {
		return err
	}
	s := o.Site()
	if o.Tile != "" {
		var err error
		if s, err = tileSite(o.Tile); err != nil {
			return err
		}
	}
	switch {
	case o.Grid > 0:
		return runSheet(o, s, w)
	case o.Sweep == "hills":
		return runSweep(o, s, w)
	}
	return runSingle(o, s, w)
}

func runSingle(o Options, s site.Site, w io.Writer) error {
	res, times, err := generate(o.Seed, o.Size, s)
	if err != nil {
		return err
	}
	imgs, err := render.Fields(res)
	if err != nil {
		return err
	}
	for _, n := range imgs {
		if err := writePNG(filepath.Join(o.Out, n.Name+".png"), render.Upscale(n.Img, o.Scale)); err != nil {
			return err
		}
	}
	var total float64
	for _, t := range times {
		total += t.Ms
	}
	mf := MetricsFile{Seed: o.Seed, Size: o.Size, Biome: biome.DefOf(s.Biome).Key, Hills: s.Hills.Label(),
		River: riverLabel(s.River), Lakes: int(s.Lakes), Caves: s.Caves, Tile: o.Tile,
		Metrics: res.Metrics, Timings: times, TotalMs: total}
	data, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(o.Out, "metrics.json"), data, 0o644); err != nil {
		return err
	}
	sum := summary(mf, res)
	if err := os.WriteFile(filepath.Join(o.Out, "summary.txt"), []byte(sum), 0o644); err != nil {
		return err
	}
	_, err = io.WriteString(w, sum)
	return err
}

func summary(mf MetricsFile, res mapgen.Result) string {
	m := mf.Metrics
	var b strings.Builder
	fmt.Fprintf(&b, "mapgen seed=%d size=%d biome=%s hills=%s river=%q lakes=%d caves=%v\n",
		mf.Seed, mf.Size, mf.Biome, mf.Hills, mf.River, mf.Lakes, mf.Caves)
	fmt.Fprintf(&b, "start=(%d,%d) plants=%d chunks=%d ruins=%d vents=%d roads=%d\n",
		res.Start.X, res.Start.Y, len(res.Plants), len(res.Chunks), len(res.Ruins), len(res.Vents), len(res.Roads))
	fmt.Fprintf(&b, "rock=%.3f largestRock=%.3f tinyRock=%d open=%.3f startReach=%.3f trees=%d sinuosity=%.2f\n",
		m.RockShare, m.LargestRockShare, m.TinyRockBodies, m.OpenShare, m.StartReach, m.TreeClusters, m.RiverSinuosity)
	if len(m.Repairs) > 0 {
		fmt.Fprintf(&b, "repairs: %s\n", strings.Join(m.Repairs, "; "))
	}
	fmt.Fprintf(&b, "total %.1f ms\n", mf.TotalMs)
	return b.String()
}

// compositeTile generates one map and returns its composite at scale.
func compositeTile(seed uint64, size, scale int, s site.Site) (*image.RGBA, error) {
	res, _, err := generate(seed, size, s)
	if err != nil {
		return nil, err
	}
	c, err := render.Composite(res)
	if err != nil {
		return nil, err
	}
	return render.Upscale(c, scale), nil
}

func labelScale(scale int) int { return max(1, scale) }

func runSheet(o Options, s site.Site, w io.Writer) error {
	var tiles []*image.RGBA
	var labels []uint64
	for i := 0; i < o.Grid; i++ {
		seed := o.Seed + uint64(i)
		t, err := compositeTile(seed, o.Size, o.Scale, s)
		if err != nil {
			return err
		}
		tiles, labels = append(tiles, t), append(labels, seed)
	}
	return writeSheet(o, tiles, labels, "grid.png", w,
		fmt.Sprintf("grid of %d seeds from %d", o.Grid, o.Seed))
}

var sweepHills = []site.Hills{site.Flat, site.SmallHills, site.LargeHills, site.Mountainous}

func runSweep(o Options, s site.Site, w io.Writer) error {
	var tiles []*image.RGBA
	var labels []uint64
	for i, h := range sweepHills {
		s.Hills = h
		t, err := compositeTile(o.Seed, o.Size, o.Scale, s)
		if err != nil {
			return err
		}
		tiles, labels = append(tiles, t), append(labels, uint64(i+1))
	}
	return writeSheet(o, tiles, labels, "sweep_hills.png", w,
		fmt.Sprintf("hills sweep of seed %d: 1 flat, 2 small, 3 large, 4 mountainous", o.Seed))
}

func writeSheet(o Options, tiles []*image.RGBA, labels []uint64, name string, w io.Writer, what string) error {
	px := o.Size * o.Scale
	sheet := render.Sheet(tiles, labels, px, px, labelScale(o.Scale))
	if err := writePNG(filepath.Join(o.Out, name), sheet); err != nil {
		return err
	}
	sum := fmt.Sprintf("%s\nwrote %s (%dx%d)\n", what, name, sheet.Bounds().Dx(), sheet.Bounds().Dy())
	if err := os.WriteFile(filepath.Join(o.Out, "summary.txt"), []byte(sum), 0o644); err != nil {
		return err
	}
	_, err := io.WriteString(w, sum)
	return err
}
