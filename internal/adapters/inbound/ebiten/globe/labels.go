package globe

import (
	"image/color"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/domain/world/site"
)

// Rivers and roads by kind, feature labels and the legend swatches (WG
// plan §1.2, part 2).

// riverWidth is each river size's line width in zoom-scale units; original
// tuning.
var riverWidth = [...]float32{site.NoRiver: 0, site.Creek: 0.6, site.River: 1.1, site.LargeRiver: 1.7, site.HugeRiver: 2.4}

const (
	riverSourceTaper = 0.3  // a source tile's river starts at this share of its width; original tuning
	creekZoom        = 1.0  // creeks show from this zoom scale on
	riverZoom        = 0.75 // larger rivers from this one
	roadZoom         = 0.75 // roads from this one
	pathZoom         = 1.0  // paths only from this one
	highwayWide      = 1.6  // an ancient highway is this much wider than an asphalt road
)

// roadStyle is how one road class is drawn: its palette colour, body and
// casing widths (zoom-scale units; no casing when 0), dashes (pieces
// along the segment; solid when nil) and an optional dashed centre line.
type roadStyle struct {
	key    string
	body   float32
	casing float32
	dashes [][2]float32
	centre bool
}

var (
	roadDashes = [][2]float32{{0.08, 0.46}, {0.54, 0.92}}
	roadDots   = [][2]float32{{0.06, 0.16}, {0.31, 0.41}, {0.56, 0.66}, {0.81, 0.91}}
	roadSolid  = [][2]float32{{0, 1}}
	// roadStyles: original tuning (plan §1.2 table).
	roadStyles = [planet.NumRoadClasses]roadStyle{
		planet.Path:               {key: "road_path", body: 1.0, dashes: roadDots},
		planet.DirtRoad:           {key: "road_dirt", body: 1.5, casing: 2.9, dashes: roadDashes},
		planet.StoneRoad:          {key: "road_stone", body: 1.5, casing: 2.9, dashes: roadSolid},
		planet.AncientAsphaltRoad: {key: "road_asphalt", body: 1.7, casing: 3.0, dashes: roadSolid, centre: true},
		planet.AncientHighway:     {key: "road_asphalt", body: 1.7 * highwayWide, casing: 3.0 * highwayWide, dashes: roadSolid, centre: true},
	}
	roadCasing = ovColour{0.06, 0.05, 0.04, 0.55}
)

func roadStyleOf(class uint8) *roadStyle {
	if int(class) >= len(roadStyles) {
		return &roadStyles[planet.DirtRoad]
	}
	return &roadStyles[class]
}

// strokeRivers strokes the rivers, tapered from their sources; creeks
// only once zoomed in.
func (geo *planetGeom) strokeRivers(x *xform, m *ovMesh, zs, scale float32) {
	river := ovFrom(WorldPalette["river"], 0.92)
	lim := x.Zh + 0.01
	for k := range geo.Rivers {
		s := &geo.Rivers[k]
		if s.w <= uint8(site.Creek) && zs < creekZoom {
			continue
		}
		ax, ay, az := x.WorldPt(&s.a)
		bx, by, bz := x.WorldPt(&s.b)
		if az < lim || bz < lim {
			continue
		}
		m.taper(ax, ay, bx, by, max(s.wa*scale, 0.6), max(s.wb*scale, 0.6), river)
	}
}

// strokeRoads strokes the roads by class: casings first, then the bodies,
// then the centre lines.
func (geo *planetGeom) strokeRoads(x *xform, m *ovMesh, zs, scale float32) {
	centre := ovFrom(WorldPalette["road_centre"], 0.85)
	for pass := 0; pass < 3; pass++ {
		for k := range geo.Roads {
			s := &geo.Roads[k]
			if s.w == uint8(planet.Path) && zs < pathZoom {
				continue
			}
			st := roadStyleOf(s.w)
			switch {
			case pass == 0 && st.casing > 0:
				m.dashes(x, s, st.dashes, st.casing*scale, roadCasing)
			case pass == 1:
				m.dashes(x, s, st.dashes, st.body*scale, ovFrom(WorldPalette[st.key], 1))
			case pass == 2 && st.centre:
				m.dashes(x, s, roadDashes, st.body*0.28*scale, centre)
			}
		}
	}
}

// LegendEntry is one legend swatch: its label, colour and line width (in
// zoom-scale units; 0 for an area swatch).
type LegendEntry struct {
	Label  string
	Colour color.RGBA
	Width  float32
}

// RoadLegend lists the road classes with their colour and line width.
func RoadLegend() [planet.NumRoadClasses]LegendEntry {
	var out [planet.NumRoadClasses]LegendEntry
	for c := range out {
		st := &roadStyles[c]
		out[c] = LegendEntry{Label: planet.RoadClass(c).Label(), Colour: WorldPalette[st.key], Width: st.body}
	}
	return out
}

// RiverLegend lists the river sizes (creek to huge) with their colour and
// line width.
func RiverLegend() [4]LegendEntry {
	var out [4]LegendEntry
	for k := range out {
		sz := site.Creek + site.RiverSize(k)
		out[k] = LegendEntry{Label: sz.Label(), Colour: WorldPalette["river"], Width: riverWidth[sz]}
	}
	return out
}

// HillLegend lists the Hilliness view mode's swatches, flat to mountainous.
func HillLegend() [4]LegendEntry {
	var out [4]LegendEntry
	for k := range out {
		h := site.Flat + site.Hills(k)
		out[k] = LegendEntry{Label: h.Label(), Colour: WorldPalette[HillKey[h]]}
	}
	return out
}

// Label is one feature or settlement name placed this frame: its screen
// rectangle (physical pixels, top-left) and text.
type Label struct {
	X, Y, W, H float32
	Text       string
	Ocean      bool // an ocean, sea or ice cap name (label_ocean, spaced)
	Settlement bool
}

// markSize is a settlement emblem's size in physical pixels: two tiles,
// clamped.
func markSize(tilePx, pk float64) float64 { return Clamp(tilePx*2, 12*pk, 46*pk) }

// Label limits (plan §1.2): at most maxLabels a frame, later overlaps dropped.
const (
	maxLabels    = 24
	labelFacing  = 0.55 // a feature's centre must face the eye this much
	settleLabelD = 1.9  // settlement names show at this distance and closer
)

// featLabel is a named world feature ready to label.
type featLabel struct {
	text       string
	tile       int32
	ocean      bool
	minD, maxD float32
	w, h       float32 // logical size at the measured UI scale
}

// labelBand is the camera distance band a feature kind is named in.
func labelBand(k planet.FeatureKind) (minD, maxD float32, ocean, ok bool) {
	switch k {
	case planet.FeatOcean, planet.FeatSea, planet.FeatIceCap:
		return 2.4, 99, true, true
	case planet.FeatMountains, planet.FeatLake, planet.FeatDesert, planet.FeatForest, planet.FeatBay:
		return 1.6, 3.2, false, true
	}
	return 0, 0, false, false
}

// spaced writes an ocean name in spaced capitals ("T H E   D E E P").
func spaced(s string) string {
	words := strings.Fields(strings.ToUpper(s))
	for k, w := range words {
		words[k] = strings.Join(strings.Split(w, ""), " ")
	}
	return strings.Join(words, "   ")
}

// buildLabels lists the planet's features, largest first (they win a
// collision), with their zoom band and text.
func (geo *planetGeom) buildLabels(p *planet.Planet) {
	fs := p.Features()
	order := make([]int, 0, len(fs))
	for k := range fs {
		if _, _, _, ok := labelBand(fs[k].Kind); ok && fs[k].Name != "" && fs[k].Centre >= 0 && fs[k].Centre < p.TileCount() {
			order = append(order, k)
		}
	}
	// insertion sort by size (a few dozen features), stable on index
	for i := 1; i < len(order); i++ {
		for j := i; j > 0 && len(fs[order[j]].Tiles) > len(fs[order[j-1]].Tiles); j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	geo.feats = make([]featLabel, 0, len(order))
	for _, k := range order {
		f := &fs[k]
		lo, hi, ocean, _ := labelBand(f.Kind)
		t := f.Name
		if ocean {
			t = spaced(t)
		}
		geo.feats = append(geo.feats, featLabel{text: t, tile: int32(f.Centre), ocean: ocean, minD: lo, maxD: hi})
	}
	geo.labels = make([]Label, 0, maxLabels)
}

// measureLabels sizes every label once per UI scale.
func (geo *planetGeom) measureLabels(marks []settleMark) {
	sc := gfx.UIScale()
	if geo.measuredAt == sc && len(geo.markWH) == len(marks) {
		return
	}
	geo.measuredAt = sc
	for k := range geo.feats {
		w, h := gfx.Measure(geo.feats[k].text, gfx.Small)
		geo.feats[k].w, geo.feats[k].h = float32(w), float32(h)
	}
	geo.markWH = make([][2]float32, len(marks))
	for k := range marks {
		w, h := gfx.Measure(marks[k].Label, gfx.Small)
		geo.markWH[k] = [2]float32{float32(w), float32(h)}
	}
}

// placeLabel adds a label unless the frame is full or it overlaps one
// already placed.
func (geo *planetGeom) placeLabel(l Label) {
	if len(geo.labels) >= maxLabels {
		return
	}
	for k := range geo.labels {
		o := &geo.labels[k]
		if l.X < o.X+o.W && o.X < l.X+l.W && l.Y < o.Y+o.H && o.Y < l.Y+l.H {
			return
		}
	}
	geo.labels = append(geo.labels, l)
}

// layoutLabels places this frame's names: settlements first (the player's
// destinations), then features largest first.
func (geo *planetGeom) layoutLabels(g *View) {
	geo.labels = geo.labels[:0]
	if len(geo.feats) == 0 && len(g.Marks) == 0 {
		return
	}
	geo.measureLabels(g.Marks)
	ts := float32(gfx.UIScale())
	dist := float32(g.Cam.Dist)
	if dist <= settleLabelD {
		size := float32(markSize(g.TilePx(), g.k()))
		for k := range g.Marks {
			m := &g.Marks[k]
			if m.Label == "" {
				continue
			}
			c := g.Surface(int(m.Tile))
			sx, sy, vz := g.WorldPt(&c)
			if vz < g.Zh+0.04 {
				continue
			}
			w, h := geo.markWH[k][0]*ts, geo.markWH[k][1]*ts
			geo.placeLabel(Label{X: sx - w/2, Y: sy + size*0.58, W: w, H: h, Text: m.Label, Settlement: true})
		}
	}
	cd := &g.CamDir
	for k := range geo.feats {
		f := &geo.feats[k]
		if dist < f.minD || dist > f.maxD {
			continue
		}
		u := &g.P.Center[f.tile]
		if u[0]*cd[0]+u[1]*cd[1]+u[2]*cd[2] <= labelFacing {
			continue
		}
		c := g.Surface(int(f.tile))
		sx, sy, _ := g.WorldPt(&c)
		w, h := f.w*ts, f.h*ts
		geo.placeLabel(Label{X: sx - w/2, Y: sy - h/2, W: w, H: h, Text: f.text, Ocean: f.ocean})
	}
}
