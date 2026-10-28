package cue

import (
	"math"
	"sort"
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
)

// The electric machines' hum and the water bed (PLAY-021): positioned loops
// the director adds to its ambience.

// MaxMachineLoops is how many machines of one kind are heard at once (the
// nearest ones; a row of generators is one hum, not five).
const MaxMachineLoops = 2

var (
	// WoodGeneratorKind is the wood generator's building kind: it has its own
	// chug, so the fire bed skips it.
	WoodGeneratorKind = construction.WoodGenerator.String()
	geothermalKind    = construction.GeothermalGenerator.String()
	windTurbineKind   = construction.WindTurbine.String()
	heaterKind        = construction.Heater.String()
	coolerKind        = construction.Cooler.String()
)

// machineLoop is the loop a kind of machine makes while it runs.
type machineLoop struct {
	id   string
	runs func(b game.BuildingView) bool
}

// thermostatWorking is a heater or cooler of kind k heating or cooling:
// powered and drawing more than it does idling at its target.
func thermostatWorking(k construction.Kind) func(b game.BuildingView) bool {
	idle := power.SpecOf(k).Idle
	return func(b game.BuildingView) bool { return b.Powered && b.PowerUse > idle }
}

// makingPower is a generator making power now.
func makingPower(b game.BuildingView) bool { return b.PowerOut > 0 }

// machineLoops are the machines that hum, by building kind. The wood
// generator also has to be burning (a switched-off one keeps its fire).
var machineLoops = map[string]machineLoop{
	WoodGeneratorKind: {"amb.generator_wood", func(b game.BuildingView) bool { return b.Burning && makingPower(b) }},
	geothermalKind:    {"amb.generator_geo", makingPower},
	windTurbineKind:   {"amb.wind_turbine", makingPower},
	heaterKind:        {"amb.heater", thermostatWorking(construction.Heater)},
	coolerKind:        {"amb.cooler", thermostatWorking(construction.Cooler)},
}

const (
	// waterSamples is the most tiles sampled across the view each way
	// (strided on big views, so the cost never grows with the map).
	waterSamples = 40
	// WaterRiverGain and WaterShoreGain turn an on-screen fraction into a
	// volume: a sixth of the view in water, or a tenth of it shoreline,
	// is the full bed.
	WaterRiverGain = 6.0
	WaterShoreGain = 10.0
)

// Machines adds a positioned loop for each running machine (a
// generator making power, a heater or cooler working), the nearest
// MaxMachineLoops of each sound. A machine stopping drops out of the list
// and fades away.
func (b *Bus) Machines(rv *render.View, want map[string]Loop, w game.WorldView, v camera.Viewport) {
	type heardMachine struct {
		key, id  string
		vol, pan float64
	}
	var heard []heardMachine
	for _, bd := range w.Buildings {
		if bd.PowerOut <= 0 && !bd.Powered { // cheap filter: nothing electric runs without one
			continue
		}
		m, ok := machineLoops[bd.Kind]
		if !ok || !m.runs(bd) {
			continue
		}
		x, y := float64(bd.X)+float64(max(bd.W, 1))/2, float64(bd.Y)+float64(max(bd.H, 1))/2
		if vol, pan, ok := Spatial(rv, x, y, v); ok {
			heard = append(heard, heardMachine{"power:" + strconv.FormatUint(uint64(bd.ID), 10), m.id, vol, pan})
		}
	}
	sort.SliceStable(heard, func(i, j int) bool { return heard[i].vol > heard[j].vol })
	count := map[string]int{}
	for _, h := range heard {
		if count[h.id] == MaxMachineLoops {
			continue
		}
		count[h.id]++
		want[h.key] = Loop{ID: h.id, Vol: h.vol, Pan: h.pan}
	}
}

// WaterOnScreen is the share of the visible tiles that are water (shallow
// or deep) and the share that are shore: water with land beside it. It
// samples a strided grid of at most waterSamples x waterSamples tiles.
func (b *Bus) WaterOnScreen(rv *render.View, w game.WorldView, v camera.Viewport) (water, shore float64) {
	if w.TerrainW <= 0 || len(w.Terrain) == 0 {
		return 0, 0
	}
	h := len(w.Terrain) / w.TerrainW
	x0f, y0f := screenToWorld(rv, int(v.X0), int(v.Y0))
	x1f, y1f := screenToWorld(rv, int(v.X1), int(v.Y1))
	x0, y0 := max(0, int(math.Floor(x0f))), max(0, int(math.Floor(y0f)))
	x1, y1 := min(w.TerrainW-1, int(math.Ceil(x1f))), min(h-1, int(math.Ceil(y1f)))
	if x1 < x0 || y1 < y0 {
		return 0, 0
	}
	stride := max(1, (max(x1-x0, y1-y0)+waterSamples)/waterSamples)
	isWater := func(x, y int) (wet, ok bool) {
		if x < 0 || y < 0 || x >= w.TerrainW || y >= h {
			return false, false
		}
		return b.Host.IsWater(w.Terrain[y*w.TerrainW+x]), true
	}
	n, wet, edge := 0, 0, 0
	for y := y0; y <= y1; y += stride {
		for x := x0; x <= x1; x += stride {
			n++
			if here, _ := isWater(x, y); !here {
				continue
			}
			wet++
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if nw, in := isWater(x+d[0], y+d[1]); in && !nw {
					edge++
					break
				}
			}
		}
	}
	return float64(wet) / float64(n), float64(edge) / float64(n)
}

// screenToWorld maps a physical screen pixel to world tile coordinates.
func screenToWorld(rv *render.View, sx, sy int) (float64, float64) {
	tile := rv.Cam.Tile()
	return (float64(sx) + rv.Cam.OffsetX) / tile, (float64(sy) + rv.Cam.OffsetY) / tile
}
