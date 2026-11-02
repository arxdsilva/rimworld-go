package overlay

import (
	"image/color"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"rimworld-go/internal/application/game"
)

// The temperature overlay (TEMP-5, UI/UX 16.3): every visible non-wall tile
// is tinted by its air temperature (rooms from RoomView.Temp, outdoors from
// WorldView.OutdoorTemp), each visible room is labelled with its degrees,
// and tiles in the danger bands (frost, scorching) get a frame. The
// per-room tints and labels are cached on WorldView.TempVer, so a quiet
// frame allocates nothing; the overlay draws nothing while it is off.
// Presentation only (original palette, ART §2).

// Danger bands, tenths of a degree C (original; the wiki's frostbite and
// scorching bands).
const (
	TempDangerCold = 0
	TempDangerHot  = 400
)

// tempStop is one stop of the colour ramp.
type tempStop struct {
	t int // whole degrees C
	c color.NRGBA
}

var tempRamp = [...]tempStop{
	{-30, color.NRGBA{70, 40, 200, 120}},
	{-10, color.NRGBA{50, 110, 230, 112}},
	{0, color.NRGBA{60, 180, 240, 100}},
	{10, color.NRGBA{80, 200, 200, 70}},
	{20, color.NRGBA{90, 200, 110, 46}}, // comfortable: faint
	{28, color.NRGBA{200, 210, 70, 80}},
	{36, color.NRGBA{240, 170, 50, 108}},
	{45, color.NRGBA{235, 70, 50, 124}},
	{60, color.NRGBA{200, 30, 120, 132}},
}

// TempTint is the overlay fill for an air temperature in tenths of a degree.
func TempTint(tenths int) color.NRGBA {
	t := tenths
	r := &tempRamp
	if t <= r[0].t*10 {
		return r[0].c
	}
	for i := 1; i < len(r); i++ {
		if t <= r[i].t*10 {
			a, b := r[i-1], r[i]
			span := (b.t - a.t) * 10
			f := t - a.t*10
			mix := func(x, y uint8) uint8 { return uint8(int(x) + (int(y)-int(x))*f/span) }
			return color.NRGBA{mix(a.c.R, b.c.R), mix(a.c.G, b.c.G), mix(a.c.B, b.c.B), mix(a.c.A, b.c.A)}
		}
	}
	return r[len(r)-1].c
}

// TempDangerous reports tenths is in a danger band.
func TempDangerous(tenths int) bool { return tenths <= TempDangerCold || tenths >= TempDangerHot }

// TempLabel is "21 °C" for tenths.
func TempLabel(tenths int) string { return TempLabelIn(units.Celsius, tenths) }

// TempLabelIn is TempLabel in the player's temperature unit (WG-16): Celsius
// floors to the degree below, Fahrenheit rounds.
func TempLabelIn(u units.Temp, tenths int) string {
	if u.F {
		return u.Tenths(tenths)
	}
	d := tenths / 10
	if tenths < 0 && tenths%10 != 0 {
		d-- // floor
	}
	return strconv.Itoa(d) + " °C"
}

// TempDrawState caches the overlay's per-room tints and labels on TempVer
// and holds the per-frame label anchors; all slices are reused.
type TempDrawState struct {
	// Units is how the labels read (Options, WG-16); the App sets it and
	// a change rebuilds the labels (unit is the one they were built in).
	Units, unit units.Temp
	valid       bool
	ver         uint64
	outdoor     int
	rooms       int

	tint   []color.NRGBA // per room id
	danger []bool
	label  []string
	outTn  color.NRGBA
	outLbl string
	outDng bool

	// per frame: visible cell count and coordinate sums per room
	n      []int32
	sx, sy []int64
	Rects  int // tiles filled by the last draw, for tests
	Builds int // cache rebuilds, for tests
}

func (s *TempDrawState) sync(w *game.WorldView) {
	if s.valid && s.ver == w.TempVer && s.outdoor == w.OutdoorTemp && s.rooms == len(w.Rooms) && s.unit == s.Units {
		return
	}
	s.valid, s.ver, s.outdoor, s.rooms, s.unit = true, w.TempVer, w.OutdoorTemp, len(w.Rooms), s.Units
	s.Builds++
	n := len(w.Rooms)
	if cap(s.tint) < n {
		s.tint, s.danger, s.label = make([]color.NRGBA, n), make([]bool, n), make([]string, n)
		s.n, s.sx, s.sy = make([]int32, n), make([]int64, n), make([]int64, n)
	}
	s.tint, s.danger, s.label = s.tint[:n], s.danger[:n], s.label[:n]
	s.n, s.sx, s.sy = s.n[:n], s.sx[:n], s.sy[:n]
	for i := range w.Rooms {
		t := w.Rooms[i].Temp
		s.tint[i], s.danger[i], s.label[i] = TempTint(t), TempDangerous(t), TempLabelIn(s.Units, t)
	}
	s.outTn, s.outDng, s.outLbl = TempTint(w.OutdoorTemp), TempDangerous(w.OutdoorTemp), TempLabelIn(s.Units, w.OutdoorTemp)
}

var (
	tempDangerCold = color.NRGBA{140, 210, 255, 210}
	tempDangerHot  = color.NRGBA{255, 120, 80, 210}
	tempPlate      = color.NRGBA{16, 18, 24, 170}
	tempLabelStyle = gfx.TextStyle{Size: 11, Bold: true, Color: color.NRGBA{240, 244, 250, 255}, Align: gfx.AlignCenter}
)

// DrawTemperature draws the overlay when it is on.
func (o *Set) DrawTemperature(v *render.View, screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	if !o.Temp.Shown() || w.TerrainW <= 0 || len(w.RoomGrid) == 0 {
		return
	}
	st := &o.TempDraw
	st.sync(&w)
	for i := range st.n {
		st.n[i], st.sx[i], st.sy[i] = 0, 0, 0
	}
	st.Rects = 0
	x0, y0, x1, y1 := visibleTiles(v, sw, sh, tile)
	rows := len(w.RoomGrid) / w.TerrainW
	edge := float32(max(1.5, tile/10))
	for y := y0; y <= y1 && y < rows; y++ {
		for x := x0; x <= x1 && x < w.TerrainW; x++ {
			id := w.RoomGrid[y*w.TerrainW+x]
			var c color.NRGBA
			var dng bool
			switch {
			case id >= 0 && id < len(st.tint):
				c, dng = st.tint[id], st.danger[id]
				st.n[id]++
				st.sx[id] += int64(x)
				st.sy[id] += int64(y)
			case id == -1:
				c, dng = st.outTn, st.outDng
			default:
				continue // walls and unknown
			}
			sx, sy := v.WorldToScreenF(float64(x), float64(y))
			o.solidRect(screen, sx, sy, float32(tile), float32(tile), c)
			st.Rects++
			if dng {
				fc := tempDangerHot
				if id >= 0 && w.Rooms[id].Temp <= TempDangerCold || id == -1 && w.OutdoorTemp <= TempDangerCold {
					fc = tempDangerCold
				}
				t := float32(tile)
				o.solidRect(screen, sx, sy, t, edge, fc)
				o.solidRect(screen, sx, sy+t-edge, t, edge, fc)
				o.solidRect(screen, sx, sy+edge, edge, t-2*edge, fc)
				o.solidRect(screen, sx+t-edge, sy+edge, edge, t-2*edge, fc)
			}
		}
	}
	if tile < 12 {
		return // too small to read a label
	}
	s := gfx.UIScale()
	for id, n := range st.n {
		if n == 0 {
			continue
		}
		cx, cy := float64(st.sx[id])/float64(n), float64(st.sy[id])/float64(n)
		px, py := v.WorldToScreenF(cx, cy)
		lbl := st.label[id]
		tw, _ := gfx.Measure(lbl, tempLabelStyle)
		pw, ph := float32((tw+6)*s), float32(15*s)
		ax, ay := px+float32(tile)/2, py+float32(tile)/2
		o.solidRect(screen, ax-pw/2, ay-ph/2, pw, ph, tempPlate)
		gfx.WorldText(screen, lbl, float64(ax), float64(ay)-6.5*s, tempLabelStyle)
	}
}
