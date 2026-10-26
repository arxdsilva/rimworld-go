package game

import (
	"fmt"
	"testing"
	"time"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// Golden state hashes (NG-1 §6.2): the complete state after 20 000 ticks
// of a generated colony, pinned before the engine-scale work (E1) so every
// behaviour-preserving optimisation is proven bit-identical. An intended
// behaviour change re-pins these, with the reason in the handoff.
//
// Re-pin: go test -run TestScaleGolden -v ./internal/application/game/
// and copy the "got" values.
//
// Re-pinned (ENG-003 M3 social core): the founders get families (off-map
// relatives take entity ids, the social stream rolls), raiders and
// colonists are registered people, colonists interact every ~1250 ticks
// (opinion memories, insult and kind-words moods, a social log), idle
// chats stand both pawns together, social relaxing at tables and fires,
// and mood is smoothed (mental breaks follow the smoothed value). Three
// identical runs; taken on a tree that also carries the in-flight M1/M2
// host-system changes (4 of 6 pins had moved with social disabled).
//
// Re-pinned (ENG-003): StateHash encodes saved values sparsely (struct
// fields by name, zero ones skipped; arrays without trailing zeros), so
// the social contract's appended record fields and work slot leave pins
// alone. Encoding only: computed on f957fc8a with no other change, and
// unchanged by the contract's enum and field additions.
//
// Re-pinned (PLAY-010): animals react to every attack (fight or flee,
// penned ones inside the pen), predators charge colonists within 3 tiles,
// unarmed colonists flee from 12 tiles, colonists and tame animals walk
// through each other, and grazed grass regrows from shoots. Checked field
// by field against 4b12749d after 20 000 ticks of all six colonies: only
// Animals, Positions, Motion, Jobs, Carried, Claims, Postponed, Hits,
// Strikes, Wounds, Stains, Logs, Skills, Joy, Need, TendedUntil, Vital,
// RNG and the new GrassGrowth moved; no colonist dies (two are hurt).
//
// Re-pinned (NG-1 E4, plant_growth.go): plants grow per chunk once per
// climate interval (plant.GrowSpan, one rate per window) instead of every
// tick, and frost kills in the same pass. Checked field by field against
// f815d545 after 1, 250, 1 000, 5 000 and 20 000 ticks of all six
// colonies: every saved field is identical except Plants, and within the
// plants only Growth differs (ids, species, positions and marks equal).
//
// Re-pinned (NG-1 S3): colonists now save their bios and skills (new
// state); every other saved field is identical to the previous pins, so
// behaviour is unchanged (checked field by field against 4d68b9aa).
//
// Re-pinned (ENG-003 polish): conversation memories now land at the
// speaker's Social impact (spec §13.2), so opinions move; with the scale
// forced to 1000 the old pins hold. Stable over -count=3, with and
// without -race.
//
// Re-pinned (ENG-003 M9b): stale since 8a84be08 (M5-M10); M9b leaves
// them as they were at that commit (checked on both, -count=3).
//
// Re-pinned earlier: save format (research slice), behaviour unchanged. StateHash
// became format-independent (normalised fields) at the same time; these
// values are identical when computed on the contract commit (fbc76fd6)
// and on the new save format.
// Re-pinned (PLAY-015): new colonists start with Firefight on (priority
// 3); checked field by field on all six runs, only Prios and Labor differ.
// Re-pinned (PLAY-017): a charging or hunting animal now warns the player
// (a danger message, and the warned colonist on the animal). Checked field
// by field against c7d739ac6 on all six runs: only Animals (ChaseWarnedAt,
// ChaseWarnedTarget) and Messages (e.g. "A timber wolf charges Cora!")
// differ; seeds 7 (48x40) and 1 (120x120) see no charge and keep their pin.
// Re-pinned (PLAY-020 bed rest): a hurt colonist rests until well. The
// start colonies have no beds, so one with an untended wound lies down
// where it is until another colonist tends it (before: only one in pain
// or sick), and a hurt one finding no bed records its retry (BedRest).
// Checked field by field on 0a403eb88 plus only the PLAY-020 sim files:
// the three runs with wounded colonists move in what tending and healing
// touch (Wounds, TendedUntil, Vital, Bodies, Jobs, Positions, Motion,
// SleptAt, Logs, Need, Joy, Skills, Social, then the RNG); 48x40/seed42's
// colonist that ended downed at 69 injury is tended and healed. The three
// runs where no one is hurt keep their pins. Stable over -count=2.
// Re-pinned (MED-01 bleeding): open cuts, bites and scratches bleed until
// tended, blood loss lowers consciousness and drains back after, pain is
// 12.5 per HP whether tended or not, downing follows consciousness,
// moving and pain shock, doctors tend the fastest bleeder first, and the
// hourly medical phase runs blood loss before the infections. The three
// runs with a hurt colonist move (48x40/seed1 and seed42, 120x120/seed7:
// each ends with a colonist alive at 25, 51 and 105 permille blood loss);
// the three with no one hurt keep their pins. Same-seed runs identical
// (TestStepIsDeterministicForSameSeed, TestBodiesAreDeterministic, and
// TestScaleGolden -count=2 printed the same values).
// Re-pinned (MED-02 tend quality and immunity): tends roll a quality on the
// medical stream (no instant heal), wounds heal per wound at the wiki's
// 8 HP a day (faster tended and lying) in the hourly medical pass instead
// of a point every 300 ticks, scars are rolled on the wound's depth, and
// infection rolls come 6–18 h after the wound on the medical stream (no
// longer g.rng). The same three runs with a hurt colonist move; the three
// with no one hurt keep their pins. Same-seed runs identical
// (TestStepIsDeterministicForSameSeed, TestBodiesAreDeterministic, and
// TestScaleGolden -count=2 printed the same values).
var scaleGolden = map[string]uint64{
	"48x40/seed1":    0xe0741d9f0ddd90e8,
	"48x40/seed7":    0xee46a1f4ce740cae,
	"48x40/seed42":   0xb1380f278ba0cb52,
	"120x120/seed1":  0x24d9b4a5e97f473a,
	"120x120/seed7":  0x4118850af9407833,
	"120x120/seed42": 0x4b3e98040be7af43,
}

const scaleGoldenTicks = 20000

func TestScaleGolden(t *testing.T) {
	if testing.Short() || raceOn {
		t.Skip("long golden run (runs in the determinism gate without -race)")
	}
	for _, size := range [][2]int{{48, 40}, {120, 120}} {
		for _, seed := range []uint64{1, 7, 42} {
			name := fmt.Sprintf("%dx%d/seed%d", size[0], size[1], seed)
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				g := NewColony(Config{Seed: seed, Width: size[0], Height: size[1]})
				g.Step(scaleGoldenTicks)
				got := g.StateHash()
				t.Logf("%q: %#x,", name, got)
				if want := scaleGolden[name]; got != want {
					t.Fatalf("state hash = %#x, want %#x", got, want)
				}
			})
		}
	}
}

func TestStateHashStable(t *testing.T) {
	g := NewColony(Config{Seed: 3, Width: 48, Height: 40})
	g.Step(300)
	a, b := g.StateHash(), g.StateHash()
	if a != b {
		t.Fatalf("StateHash not stable: %x vs %x", a, b)
	}
	g.Step(1)
	if g.StateHash() == a {
		t.Fatal("StateHash unchanged after a tick")
	}
}

// TestPhaseObserverDoesNotChangeState: an observed Step is the same Step.
func TestPhaseObserverDoesNotChangeState(t *testing.T) {
	a := NewColony(Config{Seed: 5, Width: 48, Height: 40})
	b := NewColony(Config{Seed: 5, Width: 48, Height: 40})
	seen := map[string]int{}
	b.SetPhaseObserver(func(p string, _ time.Duration) { seen[p]++ })
	a.Step(500)
	b.Step(500)
	if a.StateHash() != b.StateHash() {
		t.Fatal("observed run diverged")
	}
	for _, p := range Phases() {
		if seen[p] != 500 {
			t.Fatalf("phase %s observed %d times, want 500", p, seen[p])
		}
	}
	b.SetPhaseObserver(nil)
	a.Step(10)
	b.Step(10)
	if a.StateHash() != b.StateHash() {
		t.Fatal("diverged after removing the observer")
	}
}

// Test-only hooks for the external scale benchmarks (scale_bench_test.go,
// package game_test, which needs the scenario fixture and so cannot be
// an internal test).

// BenchColonist is the first colonist's id.
func (g *Game) BenchColonist() entity.EntityID { return g.pawnIDs[0] }

// BenchReachable is reachable(id, x, y).
func (g *Game) BenchReachable(id entity.EntityID, x, y int) bool { return g.reachable(id, x, y) }

// BenchPath is the length of pawn id's path to (x, y), or -1.
func (g *Game) BenchPath(id entity.EntityID, x, y int) int {
	p, ok := g.pathTo(id, x, y)
	if !ok {
		return -1
	}
	return len(p)
}

// BenchRebuildRooms toggles a wall beside home (built, then removed on
// the next call) and rebuilds the rooms as that change does; it returns
// the room count.
func (g *Game) BenchRebuildRooms() int {
	if id, ok := benchWalls[g]; ok {
		g.removeBuilding(id)
		delete(benchWalls, g)
	} else {
		hx, hy := g.home()
		for r := 0; r < 60 && !ok; r++ {
			for dy := -r; dy <= r && !ok; dy++ {
				for dx := -r; dx <= r && !ok; dx++ {
					x, y := hx+dx, hy+dy
					if can, _ := g.canPlaceAt(construction.Wall, construction.South, x, y); can && g.BuildNow("wall", "wood", "south", x, y) {
						benchWalls[g], ok = g.layerBldgAt(x, y, construction.LayerObject), true
					}
				}
			}
		}
	}
	g.ensureRooms()
	return len(g.rooms)
}

// benchWalls is the wall BenchRebuildRooms stands, per game.
var benchWalls = map[*Game]entity.EntityID{}

// Every test in this package runs with the per-cell indexes checked
// against the original scans (indexes.go); the external benchmarks turn
// it off.
// Under -race (about ten times slower) one lookup in 32 is checked.
func init() {
	indexSelfCheck = true
	if raceOn {
		selfCheckEvery = 32
	}
}

// SetIndexSelfCheck switches the index self-check (benchmarks: off).
func SetIndexSelfCheck(on bool) { indexSelfCheck = on }
