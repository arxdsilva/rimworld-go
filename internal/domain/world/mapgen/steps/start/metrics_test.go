package start

import (
	"math"
	"testing"

	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/plant"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestMetricsHandMade(t *testing.T) {
	c := fixture(1, 30, 30)
	for y := 2; y <= 4; y++ {
		for x := 2; x <= 4; x++ {
			setRock(c, x, y)
		}
	}
	setRock(c, 20, 2)
	c.R.Geology.Stone[c.Idx(20, 2)] = geology.Marble
	setWater(c, 10, 20, mapgen.Shallow)
	setWater(c, 11, 20, mapgen.Shallow)
	for x := 2; x <= 12; x += 2 { // 6 trees two apart: one grove
		c.R.Plants = append(c.R.Plants, mapgen.PlantSpawn{X: x, Y: 25, Species: plant.Tree})
	}
	for x := 20; x <= 28; x += 2 { // 5 pines: too few for a grove
		c.R.Plants = append(c.R.Plants, mapgen.PlantSpawn{X: x, Y: 25, Species: plant.Pine})
	}
	c.R.Plants = append(c.R.Plants, mapgen.PlantSpawn{X: 25, Y: 5, Species: plant.BerryBush})
	c.F.RiverLine = []mapgen.PointF{{X: 0, Y: 0}, {X: 3, Y: 4}, {X: 6, Y: 0}}
	c.R.Start = mapgen.Point{X: 15, Y: 15}

	m := metrics(c)
	checks := []struct {
		name      string
		got, want float64
	}{
		{"RockShare", m.RockShare, 10.0 / 900},
		{"LargestRockShare", m.LargestRockShare, 0.9},
		{"TinyRockBodies", float64(m.TinyRockBodies), 1},
		{"MeanPatchSize", m.MeanPatchSize, 900.0 / 4},
		{"OpenShare", m.OpenShare, 888.0 / 900},
		{"VegNW", m.VegQuadrants[0], 0},
		{"VegNE", m.VegQuadrants[1], 1.0 / 225},
		{"VegSW", m.VegQuadrants[2], 6.0 / 225},
		{"VegSE", m.VegQuadrants[3], 5.0 / 225},
		{"TreeClusters", float64(m.TreeClusters), 1},
		{"RiverSinuosity", m.RiverSinuosity, 10.0 / 6},
		{"Granite", m.StoneShare[geology.Granite], 0.9},
		{"Marble", m.StoneShare[geology.Marble], 0.1},
		{"StartReach", m.StartReach, 1},
	}
	for _, ch := range checks {
		if !near(ch.got, ch.want) {
			t.Errorf("%s = %v, want %v", ch.name, ch.got, ch.want)
		}
	}
}

func TestMetricsEmptyAndPartial(t *testing.T) {
	// no rock, no river, no plants: zero shares, not NaN
	c := fixture(1, 1, 1)
	m := metrics(c)
	if m.LargestRockShare != 0 || m.RiverSinuosity != 0 || m.StartReach != 1 || m.MeanPatchSize != 1 {
		t.Fatalf("%+v", m)
	}
	// start walled off from half the land
	c = fixture(1, 20, 10)
	for y := range 10 {
		setRock(c, 9, y)
	}
	c.R.Start = mapgen.Point{X: 2, Y: 5}
	if r := metrics(c).StartReach; !near(r, 90.0/190) {
		t.Fatalf("reach %v", r)
	}
	// unwalkable start: nothing reached
	setRock(c, 2, 5)
	if r := metrics(c).StartReach; r != 0 {
		t.Fatalf("reach from rock %v", r)
	}
	// a closed river loop has no end-to-end distance
	c.F.RiverLine = []mapgen.PointF{{X: 1, Y: 1}, {X: 4, Y: 1}, {X: 1, Y: 1}}
	if s := metrics(c).RiverSinuosity; s != 0 {
		t.Fatalf("closed loop sinuosity %v", s)
	}
}
