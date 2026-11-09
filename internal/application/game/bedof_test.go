package game

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// bedOfScan is the reference: the scan bedOf replaced.
func bedOfScan(g *Game, id entity.EntityID) entity.EntityID {
	for _, bid := range g.bldgIDs {
		if b, ok := g.bldgs.Get(bid); ok && b.Kind.Sleeps() && b.SlotOf(uint64(id)) >= 0 {
			return bid
		}
	}
	return 0
}

// TestBedOfMatchesScan: through random builds, claims, releases,
// deconstructions and save/loads the cached bed list gives bedOf the scan's
// answer for every pawn, and rebuilds only when the structure moves.
func TestBedOfMatchesScan(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 40, Height: 30})
	var pawns []entity.EntityID
	for i := range 6 {
		id, _ := g.SpawnNamedPawn(1+i, 1, "P")
		pawns = append(pawns, id)
	}
	rng := rand.New(rand.NewPCG(7, 9))
	kinds := []construction.Kind{construction.Bed, construction.DoubleBed, construction.SleepingSpot, construction.HospitalBed, construction.Campfire}
	var beds []entity.EntityID
	owned := 0
	check := func(step int, what string) {
		t.Helper()
		for _, p := range pawns {
			if got, want := g.bedOf(p), bedOfScan(g, p); got != want {
				t.Fatalf("step %d (%s): bedOf(%d) = %d, scan %d", step, what, p, got, want)
			} else if got != 0 {
				owned++
			}
		}
	}
	for step := range 400 {
		what := ""
		switch op := rng.IntN(5); {
		case op == 0 && len(beds) < 14:
			k := kinds[rng.IntN(len(kinds))]
			x, y := 2+rng.IntN(36), 4+rng.IntN(24)
			if ok, _ := g.canPlace(k, x, y); ok {
				what = "build"
				if id := g.layerBldgAt(x, y, construction.DefOf(k).Layer); id == 0 {
					g.placeFrameStuff(k, construction.DefaultStuff(k), construction.South, x, y)
					if fid := g.layerFrameAt(x, y, construction.DefOf(k).Layer); fid != 0 {
						fr, _ := g.frames.Get(fid)
						g.completeFrame(fid, fr)
					}
				}
			}
			beds = beds[:0]
			for _, bid := range g.bldgIDs {
				beds = append(beds, bid)
			}
		case op == 1 && len(beds) > 0:
			what = "claim"
			bid := beds[rng.IntN(len(beds))]
			g.rest.Beds.AssignOwner(bid, pawns[rng.IntN(len(pawns))], rng.IntN(2))
		case op == 2 && len(beds) > 0:
			what = "release"
			g.rest.Beds.AssignOwner(beds[rng.IntN(len(beds))], 0, rng.IntN(2))
		case op == 3 && len(beds) > 0:
			what = "deconstruct"
			i := rng.IntN(len(beds))
			g.removeBuilding(beds[i])
			beds = append(beds[:i], beds[i+1:]...)
		default:
			what = "save/load"
			var buf bytes.Buffer
			if err := g.Save(&buf, "t"); err != nil {
				t.Fatal(err)
			}
			g2, _, err := Load(&buf)
			if err != nil {
				t.Fatal(err)
			}
			g = g2
			pawns = pawns[:0]
			for _, id := range g.pawnIDs {
				pawns = append(pawns, id)
			}
		}
		check(step, what)
	}
	if owned < 20 {
		t.Fatalf("only %d claimed beds seen: the walk proves little", owned)
	}
	if n := g.vents.beds.Builds; n == 0 || n > 400 {
		t.Fatalf("bed list rebuilt %d times", n)
	}
}
