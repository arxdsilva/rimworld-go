package game

import (
	"testing"

	"rimworld-go/internal/application/game/rest"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/entity"
)

// Comfort, beauty and outdoors needs and sitting (BED-3): rest/needs.go,
// rest/seats.go, rest/beauty.go.

func TestNeedsReadModelAndSave(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	id, _ := g.SpawnNamedPawn(4, 4, "Bo")
	g.Step(3000)
	c, b, o := g.rest.Needs.Percents(id)
	v := pawnView(g, id)
	if v.Comfort != c || v.Beauty != b || v.Outdoors != o || o != 100 || c >= 50 {
		t.Fatalf("view %d %d %d, needs %d %d %d", v.Comfort, v.Beauty, v.Outdoors, c, b, o)
	}
	h := saveLoad(t, g)
	if h.rest.Needs.Of(id) != g.rest.Needs.Of(id) {
		t.Fatalf("round trip %+v vs %+v", h.rest.Needs.Of(id), g.rest.Needs.Of(id))
	}
}

func TestHasBedInReadModel(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	id, _ := g.SpawnNamedPawn(4, 4, "Bo")
	other, _ := g.SpawnNamedPawn(5, 4, "Al")
	if pawnView(g, id).HasBed || pawnView(g, other).HasBed {
		t.Fatal("a colonist with no bed has one")
	}
	// one free bed: one of the two has a bed, without waiting to sleep
	bed := buildNow(t, g, construction.Bed, item.Wood, construction.South, 8, 8)
	if a, b := pawnView(g, id).HasBed, pawnView(g, other).HasBed; a == b {
		t.Fatalf("one free bed for two: %v %v", a, b)
	}
	// the second colonist claims it: now the first is the one without
	b, _ := g.bldgs.Get(bed)
	g.bldgs.Set(bed, b.Claim(uint64(other)))
	g.Step(1)
	if pawnView(g, id).HasBed || !pawnView(g, other).HasBed {
		t.Fatal("the owner should hold the only bed")
	}
	buildNow(t, g, construction.Bedroll, item.Cloth, construction.South, 2, 8)
	g.Step(1)
	if !pawnView(g, id).HasBed {
		t.Fatal("a second free bed")
	}
}

// TestSittingRaisesComfort: in free time a colonist with low comfort sits
// in the armchair, its comfort climbs, and once up it does not sit again
// while its comfort stays above rest.SeekComfort (no flap).
func TestSittingRaisesComfort(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 12})
	scheduleAll(g, schedule.Recreation)
	id, _ := g.SpawnNamedPawn(4, 4, "Bo")
	buildNow(t, g, construction.Armchair, item.Cloth, construction.South, 8, 4)
	r := g.rest.Needs.Of(id)
	r.Comfort = 5 * needs.LevelPct
	g.rest.Needs.Import(map[entity.EntityID]rest.NeedRecord{id: r})
	sat, stoodAt, sits, uncomfortable := false, 0, 0, false
	for tick := range 6000 {
		g.Step(1)
		j, _ := g.jobs.Get(id)
		if j.Kind == job.SitDown && j.State == job.Do && !sat {
			sat, sits = true, sits+1
			if j.TX != 8 || j.TY != 4 {
				t.Fatalf("sits at %d,%d", j.TX, j.TY)
			}
		}
		if sat && j.Kind != job.SitDown {
			sat, stoodAt = false, tick
		}
		if tick%50 == 0 {
			for _, th := range g.thoughts(id) {
				uncomfortable = uncomfortable || th.Kind == mood.ComfortMood && th.Value == -3
			}
		}
	}
	if sits != 1 || stoodAt == 0 {
		t.Fatalf("sat %d times, stood at %d", sits, stoodAt)
	}
	if c := g.rest.Needs.Of(id).Comfort; c < rest.SeekComfort {
		t.Fatalf("comfort %d after sitting", c)
	}
	if !uncomfortable {
		t.Fatal("never felt uncomfortable at 5 %")
	}
}

// TestDiningPrefersTheArmchair: a hungry colonist carries its meal to the
// armchair at the table rather than the nearer stool (rest/seats.go).
func TestDiningPrefersTheArmchair(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(10, 8, "Bo")
	buildNow(t, g, construction.Table, item.Wood, construction.South, 10, 4)
	buildNow(t, g, construction.Stool, item.Wood, construction.South, 10, 5)
	buildNow(t, g, construction.Armchair, item.Cloth, construction.South, 11, 4)
	x, y, ok := g.rest.Seats.AtTable(id, 10, 8)
	if !ok || x != 11 || y != 4 {
		t.Fatalf("seat %d,%d %v", x, y, ok)
	}
}
