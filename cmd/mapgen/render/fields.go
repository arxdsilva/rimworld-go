package render

import (
	"errors"
	"image"
	"image/color"

	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/terrain"
)

// ErrNoFields is returned for a Result generated without Config.Debug.
var ErrNoFields = errors.New("render: result has no debug fields (Config.Debug)")

// Named is one output image (1 pixel per cell).
type Named struct {
	Name string
	Img  *image.RGBA
}

// Names lists the per-field images in output order.
var Names = []string{
	"elevation", "mountain_bias", "fertility", "wetness", "stone", "caves",
	"water_distance", "terrain", "veg_density", "plants", "resources",
	"start_score", "regions", "roofs", "fog_initial", "composite",
}

// view bundles what the painters read.
type view struct {
	res mapgen.Result
	f   *mapgen.Fields
	w   int
	h   int
}

func newView(res mapgen.Result) (*view, error) {
	if res.Fields == nil {
		return nil, ErrNoFields
	}
	return &view{res: res, f: res.Fields, w: res.W, h: res.H}, nil
}

func (v *view) paint(at func(i int) color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, v.w, v.h))
	for i := 0; i < v.w*v.h; i++ {
		c := at(i)
		img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = c.R, c.G, c.B, 255
	}
	return img
}

func (v *view) ter(i int) terrain.Type { return v.res.Terrain.Cells[i] }

func (v *view) set(img *image.RGBA, x, y int, c color.RGBA) {
	if x >= 0 && y >= 0 && x < v.w && y < v.h {
		img.SetRGBA(x, y, c)
	}
}

// Composite is terrain with plants and features drawn on it.
func Composite(res mapgen.Result) (*image.RGBA, error) {
	v, err := newView(res)
	if err != nil {
		return nil, err
	}
	return v.composite(), nil
}

func (v *view) composite() *image.RGBA {
	img := v.paint(func(i int) color.RGBA { return TerrainColor(v.ter(i)) })
	v.drawPlants(img)
	v.drawFeatures(img, false)
	return img
}

func (v *view) drawPlants(img *image.RGBA) {
	for _, p := range v.res.Plants {
		v.set(img, p.X, p.Y, PlantColor(p.Species))
	}
}

// drawFeatures marks chunks, vents (2x2), ruins, bridges, vaults and, when roads, the road mask.
func (v *view) drawFeatures(img *image.RGBA, roads bool) {
	for _, c := range v.res.Chunks {
		v.set(img, c.X, c.Y, ChunkColor)
	}
	for _, p := range v.res.Vents {
		for dy := 0; dy < 2; dy++ {
			for dx := 0; dx < 2; dx++ {
				v.set(img, p.X+dx, p.Y+dy, VentColor)
			}
		}
	}
	for _, r := range v.res.Ruins {
		for _, w := range r.Walls {
			v.set(img, w.X, w.Y, RuinColor)
		}
		for _, fl := range r.Floors {
			v.set(img, fl.X, fl.Y, Dim(RuinColor, 0.7))
		}
	}
	v.drawBridgesAndVaults(img)
	if roads {
		for i, r := range v.f.Road {
			if r {
				img.SetRGBA(i%v.w, i/v.w, RoadColor)
			}
		}
	}
}

func (v *view) walkable() []bool {
	w := make([]bool, v.w*v.h)
	for i := range w {
		w[i] = v.ter(i).Walkable()
	}
	return w
}

// Fields paints every debug image for a generated map, in Names order.
func Fields(res mapgen.Result) ([]Named, error) {
	v, err := newView(res)
	if err != nil {
		return nil, err
	}
	f := v.f
	grey := func(s []float32) *image.RGBA { return v.paint(func(i int) color.RGBA { return Grey(s[i]) }) }
	comp := v.composite()
	byName := map[string]*image.RGBA{
		"elevation":     grey(f.Elevation),
		"mountain_bias": grey(f.MountainBias),
		"fertility":     grey(f.Fertility),
		"wetness":       v.paint(func(i int) color.RGBA { return WetRamp(f.Wetness[i]) }),
		"stone":         v.paint(func(i int) color.RGBA { return StoneColor(f.Stone[i]) }),
		"caves": v.paint(func(i int) color.RGBA {
			if f.Cave[i] {
				return CaveColor
			}
			if f.Rock[i] {
				return Grey(0.4)
			}
			return Grey(0.12)
		}),
		"water_distance": v.waterDistance(),
		"terrain":        v.paint(func(i int) color.RGBA { return TerrainColor(v.ter(i)) }),
		"veg_density":    v.paint(func(i int) color.RGBA { return Heat(f.VegDensity[i]) }),
		"plants":         v.plants(),
		"resources":      v.resources(),
		"start_score":    v.startScore(),
		"regions":        v.regions(),
		"roofs":          v.roofs(),
		"fog_initial":    v.fog(comp),
		"composite":      comp,
	}
	out := make([]Named, 0, len(Names))
	for _, n := range Names {
		out = append(out, Named{n, byName[n]})
	}
	return out, nil
}

func (v *view) waterDistance() *image.RGBA {
	f := v.f
	img := v.paint(func(i int) color.RGBA {
		d := min(f.RiverDist[i], f.CoastDist[i])
		return DistRamp(d)
	})
	for _, p := range f.RiverLine {
		v.set(img, int(p.X), int(p.Y), RiverLine)
	}
	return img
}

func (v *view) plants() *image.RGBA {
	img := v.paint(func(i int) color.RGBA { return Dim(TerrainColor(v.ter(i)), 0.55) })
	v.drawPlants(img)
	return img
}

func (v *view) resources() *image.RGBA {
	img := v.paint(func(i int) color.RGBA {
		t := v.ter(i)
		if terrain.IsRockWall(t) {
			if o := v.res.Geology.OreAt(i%v.w, i/v.w); o != geology.NoOre {
				return OreColor(o)
			}
			return Grey(0.3)
		}
		return Dim(TerrainColor(t), 0.6)
	})
	v.drawFeatures(img, true)
	return img
}

func (v *view) startScore() *image.RGBA {
	img := v.paint(func(i int) color.RGBA { return Heat(v.f.StartScore[i]) })
	s := v.res.Start
	for d := -3; d <= 3; d++ {
		v.set(img, s.X+d, s.Y, color.RGBA{255, 255, 255, 255})
		v.set(img, s.X, s.Y+d, color.RGBA{255, 255, 255, 255})
	}
	return img
}

func (v *view) regions() *image.RGBA {
	label, _ := mapgen.Components(v.w, v.h, v.walkable(), false)
	return v.paint(func(i int) color.RGBA { return RegionColor(int(label[i])) })
}

func (v *view) roofs() *image.RGBA {
	return v.paint(func(i int) color.RGBA {
		c := v.res.Roof[i]
		switch {
		case c.Thick():
			return color.RGBA{60, 40, 110, 255}
		case c.Has(roof.Natural):
			return color.RGBA{150, 130, 200, 255}
		}
		return Grey(0.15)
	})
}

// fog is black where the start cannot reach through walkable cells.
func (v *view) fog(comp *image.RGBA) *image.RGBA {
	src := make([]bool, v.w*v.h)
	s := v.res.Start
	if s.X >= 0 && s.Y >= 0 && s.X < v.w && s.Y < v.h {
		src[s.Y*v.w+s.X] = true
	}
	d := mapgen.BFSDist(v.w, v.h, src, v.walkable(), false)
	img := image.NewRGBA(comp.Rect)
	copy(img.Pix, comp.Pix)
	for i, dd := range d {
		if dd < 0 {
			img.SetRGBA(i%v.w, i/v.w, color.RGBA{0, 0, 0, 255})
		}
	}
	return img
}
