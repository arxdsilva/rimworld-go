package env

import (
	"math"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

func TestFrostOnFreezingMornings(t *testing.T) {
	t.Parallel()
	at := func(h int) clock.Date { return clock.Date{Hour: h} }
	if FrostStrength(-40, 0, at(6)) <= FrostStrength(-10, 0, at(6)) || FrostStrength(-40, 0, at(6)) <= 0 {
		t.Fatal("colder mornings should frost more")
	}
	if FrostStrength(-40, 0, at(9)) >= FrostStrength(-40, 0, at(6)) {
		t.Fatal("frost should fade through the morning")
	}
	for _, c := range []struct {
		temp int
		snow float64
		hour int
	}{{10, 0, 6}, {-40, 0, 14}, {-40, 0, 1}, {-40, 0.5, 6}} {
		if f := FrostStrength(c.temp, c.snow, at(c.hour)); f != 0 {
			t.Fatalf("frost %.2f at %+v", f, c)
		}
	}
}

func TestPuddlesFollowWetness(t *testing.T) {
	t.Parallel()
	// PLAY-022: puddles are one field, not per-tile stickers: water lies
	// where the ground's relief (PuddleField) is under the level the
	// wetness raises. The six sprite variants are retired; patch shape and
	// size variety is TestPuddlesAreCoherentPatches.
	salt := PuddleSalt(30, 30)
	count := func(wet float64) int {
		n := 0
		for i := 0; i < 900; i++ {
			if PuddleField(float64(i%30)+0.5, float64(i/30)+0.5, salt) < PuddleLevel(wet) {
				n++
			}
		}
		return n
	}
	dry, damp, soaked := count(0), count(0.3), count(1)
	if dry != 0 || !(damp > 0 && damp < soaked) || soaked > 450 {
		t.Fatalf("puddles by wetness: 0->%d 0.3->%d 1->%d of 900", dry, damp, soaked)
	}
	// a puddle never shrinks as it gets wetter: the level only rises
	for wet := 0.0; wet < 1; wet += 0.05 {
		if PuddleLevel(wet+0.05) < PuddleLevel(wet) {
			t.Fatalf("the water level falls from wetness %.2f", wet)
		}
	}
	// the field is the map's own: another map size lays them elsewhere
	other, same := PuddleSalt(31, 30), 0
	for i := 0; i < 900; i++ {
		fx, fy := float64(i%30)+0.5, float64(i/30)+0.5
		if (PuddleField(fx, fy, salt) < PuddleLevel(1)) == (PuddleField(fx, fy, other) < PuddleLevel(1)) {
			same++
		}
	}
	if same == 900 {
		t.Fatal("two maps share one puddle layout")
	}
}

func TestStormDarkensSky(t *testing.T) {
	t.Parallel()
	veil := func(kind string, i float64) float64 {
		c, _ := WeatherVeil(game.WorldView{Weather: kind, WeatherIntensity: i})
		lum := (float64(c.R) + float64(c.G) + float64(c.B)) / (3 * 255)
		return float64(c.A) / 255 * (1 - lum) // how much it darkens
	}
	clear, cloudy, rain, storm := veil("clear", 0), veil("cloudy", 0.15), veil("rain", 0.6), veil("storm", 1)
	if clear != 0 || !(cloudy > 0 && rain > cloudy && storm > rain) {
		t.Fatalf("darkening: clear %.3f cloudy %.3f rain %.3f storm %.3f", clear, cloudy, rain, storm)
	}
	if _, fog := WeatherVeil(game.WorldView{Weather: "blizzard", WeatherIntensity: 1}); fog.A < 40 {
		t.Fatalf("a blizzard barely cuts visibility: fog alpha %d", fog.A)
	}
	if _, fog := WeatherVeil(game.WorldView{Weather: "rain", WeatherIntensity: 1}); fog.A != 0 {
		t.Fatal("rain fogs the screen like a blizzard")
	}
	// lightning: rare, deterministic, storms only
	flashes := 0
	for b := uint64(0); b < 4000; b++ {
		f := LightningAt("storm", 1, b)
		if f != LightningAt("storm", 1, b) {
			t.Fatal("lightning is not deterministic")
		}
		if f {
			flashes++
		}
		if LightningAt("rain", 1, b) || LightningAt("clear", 0, b) {
			t.Fatal("lightning outside a storm")
		}
	}
	if flashes == 0 || flashes > 200 {
		t.Fatalf("%d flashes in 4000 buckets", flashes)
	}
}

func TestSnowDepthPerTile(t *testing.T) {
	t.Parallel()
	e := &Sky{TerrW: 4, SnowCells: []uint8{0, 255, 0, 0, 51, 0, 0, 0}, Snow: 0.7}
	if d := e.SnowDepthAt(1, 0); d != 1 {
		t.Fatalf("tile depth %v", d)
	}
	if d := e.SnowDepthAt(2, 1); d != 0 || e.SnowDepthAt(-1, 0) != 0 || e.SnowDepthAt(0, 5) != 0 {
		t.Fatal("bare or off-map tiles read 0")
	}
	// the corner between (0,0), (1,0), (0,1), (1,1) is their mean
	if c := e.SnowCorner(1, 1); math.Abs(c-(255+51)/4.0/255) > 1e-9 {
		t.Fatalf("corner %v", c)
	}
	// a map corner averages the one tile it touches
	if c := e.SnowCorner(0, 0); c != 0 {
		t.Fatalf("map corner %v", c)
	}
	if c := e.SnowCorner(-5, -5); c != 0 {
		t.Fatalf("far corner %v", c)
	}
	// without the layer every tile reads the map figure
	u := &Sky{TerrW: 4, Snow: 0.7}
	if u.SnowDepthAt(3, 1) != 0.7 || u.SnowCorner(2, 2) != 0.7 {
		t.Fatal("no layer: the map figure")
	}
}

func TestPuddleSkyFollowsDaylight(t *testing.T) {
	t.Parallel()
	noon, night := PuddleSky(light.LightAt(12, 40, "rain", 1)), PuddleSky(light.LightAt(1, 40, "rain", 1))
	if noon[3] != 1 || night[3] != 1 {
		t.Fatalf("alpha scale %v %v: the darkening is baked", noon[3], night[3])
	}
	if !(noon[0]+noon[1]+noon[2] > 1.5*(night[0]+night[1]+night[2])) || noon[2] < noon[0] {
		t.Fatalf("reflection noon %v night %v: want a cool sky, dimmer at night", noon, night)
	}
}

// TestPuddleBakeReliefIsTheField: the bake's cached lattices give
// PuddleField's relief.
func TestPuddleBakeReliefIsTheField(t *testing.T) {
	t.Parallel()
	salt := PuddleSalt(96, 64)
	var wb WetBake
	wb.FillLattices(0, 0, 39, 39, salt)
	for i := 0; i < 400; i++ {
		fx, fy := motion.Hash01(uint64(i), 1)*39, motion.Hash01(uint64(i), 2)*39
		if f, want := wb.Relief(fx, fy), PuddleField(fx, fy, salt); math.Abs(f-want) > 1e-12 {
			t.Fatalf("relief at (%.2f, %.2f) = %v, field %v", fx, fy, f, want)
		}
	}
}
