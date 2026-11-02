package worldinfo

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/domain/world/site"
)

// Filter panel layout (px).
const (
	FilterW      = 380
	filterPad    = 12
	filterRowH   = 26
	filterTempLo = -40 // °C bounds of the temperature range
	filterTempHi = 45
	filterStep   = 5
)

// FilterPanel is the random-site criteria editor (WG-11): which biomes,
// the yearly-mean temperature range, hilliness, river, coast, road, caves
// and stone. The zero value filters nothing. Filter() is what
// planet.RandomLandingFiltered takes.
type FilterPanel struct {
	Biomes  biome.Set
	TempOn  bool
	TempMin int // °C, whole degrees (used while TempOn)
	TempMax int
	Hills   uint8 // bit per site.Hills value
	River   planet.Tri
	Coast   planet.Tri
	Road    planet.Tri
	Caves   planet.Tri
	Stone   geology.Stone
}

// Reset clears every criterion.
func (f *FilterPanel) Reset() { *f = FilterPanel{} }

// Active reports whether any criterion is set.
func (f *FilterPanel) Active() bool { return *f != FilterPanel{} }

// Filter is the planet filter the criteria make.
func (f *FilterPanel) Filter() planet.Filter {
	out := planet.Filter{Biomes: f.Biomes, Hills: f.Hills, River: f.River, Coast: f.Coast, Road: f.Road, Caves: f.Caves, Stone: f.Stone}
	if f.TempOn {
		out.TempMin, out.TempMax = float64(f.TempMin), float64(f.TempMax)
		if out.TempMax <= out.TempMin {
			out.TempMax = out.TempMin + 1 // 0..0 would mean "any"
		}
	}
	return out
}

var hillChoices = []site.Hills{site.Flat, site.SmallHills, site.LargeHills, site.Mountainous}

var triLabels = [...]string{planet.Any: "Any", planet.Yes: "Yes", planet.No: "No"}

func nextTri(t planet.Tri) planet.Tri { return (t + 1) % 3 }

// nextStone cycles none (any), then each stone, then none again.
func nextStone(s geology.Stone) geology.Stone {
	all := geology.Stones()
	if s == geology.None {
		return all[0]
	}
	for i, x := range all {
		if x == s {
			if i+1 < len(all) {
				return all[i+1]
			}
			break
		}
	}
	return geology.None
}

// Height is the panel's height.
func (f *FilterPanel) Height() float32 {
	biomeRows := (len(biome.All()) + 1) / 2
	return 2*filterPad + 22 + float32(biomeRows)*filterRowH + 6 + filterRowH + 6 + 22 + filterRowH + 6 + 2*filterRowH + 6 + filterRowH + 8 + 36
}

// Draw declares the panel with its top-left corner at (x, y) and returns
// its rectangle. Ids are prefix+".biome.<key>", ".temp", ".tempmin.dec/.inc",
// ".tempmax.dec/.inc", ".hills.<n>", ".river", ".coast", ".road", ".caves",
// ".stone" and ".reset". u formats the temperature range.
func (f *FilterPanel) Draw(k *uikit.Kit, prefix string, x, y float32, u units.Temp) gfx.Rect {
	r := gfx.Rect{X: x, Y: y, W: FilterW, H: f.Height()}
	k.Panel(r)
	in := r.Inset(filterPad)
	hs := gfx.Label
	hs.Color = uikit.Heading
	cy := in.Y
	k.Label(in.X, cy, "Random site criteria", hs)
	k.Tip(prefix+".tip", gfx.Rect{X: in.X, Y: cy, W: in.W, H: 20}, "Random site criteria\nSelect random site only picks tiles that match every criterion you set. Nothing set means any tile.")
	cy += 22

	// Biomes, two columns.
	all := biome.All()
	colW := in.W / 2
	for n, d := range all {
		d := d
		cell := gfx.Rect{X: in.X + float32(n%2)*colW, Y: cy + float32(n/2)*filterRowH, W: colW - 4, H: filterRowH - 2}
		on := f.Biomes.Has(d.ID)
		if k.Check(prefix+".biome."+d.Key, cell, d.Name, &on, uikit.CheckOpts{}) {
			f.Biomes ^= biome.Of(d.ID)
		}
	}
	cy += float32((len(all)+1)/2)*filterRowH + 6

	// Temperature range.
	tr := gfx.Rect{X: in.X, Y: cy, W: in.W, H: filterRowH}
	k.Check(prefix+".temp", gfx.Rect{X: tr.X, Y: tr.Y, W: 150, H: tr.H - 2}, "Yearly mean", &f.TempOn,
		uikit.CheckOpts{Tip: "Temperature range\nOnly tiles whose yearly mean temperature lies in the range."})
	if f.TempOn {
		if f.TempMin == 0 && f.TempMax == 0 {
			f.TempMin, f.TempMax = 5, 30
		}
		f.stepper(k, prefix+".tempmin", tr.X+150, tr.Y, &f.TempMin, u, filterTempLo, f.TempMax-filterStep)
		f.stepper(k, prefix+".tempmax", tr.X+150+110, tr.Y, &f.TempMax, u, f.TempMin+filterStep, filterTempHi)
	}
	cy += filterRowH + 6

	// Hills.
	k.Label(in.X, cy, "Terrain", hs)
	cy += 22
	hw := in.W / 2
	for n, h := range hillChoices {
		cell := gfx.Rect{X: in.X + float32(n%2)*hw, Y: cy + float32(n/2)*filterRowH, W: hw - 4, H: filterRowH - 2}
		on := f.Hills&(1<<h) != 0
		if k.Check(fmt.Sprintf("%s.hills.%d", prefix, n), cell, h.Label(), &on, uikit.CheckOpts{}) {
			f.Hills ^= 1 << h
		}
	}
	cy += 2*filterRowH + 6

	// Tri-state switches and stone.
	type tri struct {
		id, label string
		v         *planet.Tri
	}
	tris := []tri{{"river", "River", &f.River}, {"coast", "Coast", &f.Coast}, {"road", "Road", &f.Road}, {"caves", "Caves", &f.Caves}}
	bw := (in.W - 8) / 2
	for n, t := range tris {
		t := t
		b := gfx.Rect{X: in.X + float32(n%2)*(bw+8), Y: cy + float32(n/2)*(filterRowH+4), W: bw, H: filterRowH}
		uikit.StyledButton(k, prefix+"."+t.id, b, t.label+": "+triLabels[*t.v], uikit.BtnOpts{Selected: *t.v != planet.Any,
			Tip: t.label + "\nAny, only tiles with it, or only tiles without it."}, func() { *t.v = nextTri(*t.v) })
	}
	cy += 2*(filterRowH+4) + 2
	stone := "Any"
	if f.Stone != geology.None {
		stone = f.Stone.Label()
	}
	uikit.StyledButton(k, prefix+".stone", gfx.Rect{X: in.X, Y: cy, W: in.W, H: filterRowH}, "Stone: "+stone, uikit.BtnOpts{Selected: f.Stone != geology.None,
		Tip: "Stone\nOnly tiles whose rock includes this stone."}, func() { f.Stone = nextStone(f.Stone) })
	cy += filterRowH + 8
	uikit.StyledButton(k, prefix+".reset", gfx.Rect{X: in.X, Y: cy, W: 110, H: 30}, "Reset", uikit.BtnOpts{Disabled: !f.Active(), Reason: "Nothing is set."}, f.Reset)
	return r
}

// stepper is "- value +" over a whole-degree Celsius field, in the
// player's unit.
func (f *FilterPanel) stepper(k *uikit.Kit, id string, x, y float32, v *int, u units.Temp, lo, hi int) {
	bw := float32(24)
	h := float32(filterRowH - 2)
	uikit.StyledButton(k, id+".dec", gfx.Rect{X: x, Y: y, W: bw, H: h}, "-", uikit.BtnOpts{Disabled: *v <= lo}, func() { *v = max(lo, *v-filterStep) })
	uikit.StyledButton(k, id+".inc", gfx.Rect{X: x + bw + 54, Y: y, W: bw, H: h}, "+", uikit.BtnOpts{Disabled: *v >= hi}, func() { *v = min(hi, *v+filterStep) })
	text := u.Whole(float64(*v))
	k.Queue(func(dst *ebiten.Image) {
		st := gfx.Label
		st.Align = gfx.AlignCenter
		gfx.Text(dst, text, float64(x+bw+27), float64(y)+(float64(h)-gfx.LineH(st))/2, st)
	})
}
