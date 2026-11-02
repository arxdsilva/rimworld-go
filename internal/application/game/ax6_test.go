package game

import (
	"testing"

	"rimworld-go/internal/application/game/recreate"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/mapgen"
)

// ax6Game is an open researched map with one colonist at (4, 10) low on
// recreation.
func ax6Game(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 6, Width: 30, Height: 20})
	g.CompleteResearch()
	pid, err := g.SpawnPawn(4, 10)
	if err != nil {
		t.Fatal(err)
	}
	g.joy[pid] = recreation.Bored
	return g, pid
}

// ax6Session steps g until pid is doing a recreation session (at most n
// ticks): its job then.
func ax6Session(t *testing.T, g *Game, pid entity.EntityID, n int) job.Record {
	t.Helper()
	for i := 0; i < n; i++ {
		g.Step(1)
		if j, _ := g.jobs.Get(pid); recreate.Uses(j.Kind) && j.State == job.Do {
			return j
		}
	}
	j, _ := g.jobs.Get(pid)
	t.Fatalf("no recreation session in %d ticks (job %v)", n, j.Kind)
	return j
}

// TestAX6ColonistPlaysChess (AX-6): a bored colonist with free time goes
// to the chess table by itself, plays from beside it, gains recreation
// and Intellectual experience, and the views read it: the table's joy
// kind and user, the colonist's joy kind, its "play chess" act toward
// the table and, afterwards, its cerebral tolerance.
func TestAX6ColonistPlaysChess(t *testing.T) {
	t.Parallel()
	g, pid := ax6Game(t)
	table := buildNow(t, g, construction.ChessTable, item.Wood, construction.South, 10, 10)
	j := ax6Session(t, g, pid, 20000)
	if j.Kind != job.Recreate || j.Dest != table || (j.TX != 9 && j.TX != 11) || j.TY != 10 {
		t.Fatalf("session %+v, want a game at the chess table from beside it", j)
	}
	joy := g.joy[pid]
	g.Step(200)
	if g.joy[pid] <= joy {
		t.Fatalf("recreation %d -> %d while playing", joy, g.joy[pid])
	}
	var bv BuildingView
	for _, b := range g.World().Buildings {
		if b.ID == table {
			bv = b
		}
	}
	if bv.AX.Joy != "cerebral" || bv.AX.Users != 1 {
		t.Fatalf("table pane %q, %d users; want cerebral, 1", bv.AX.Joy, bv.AX.Users)
	}
	for _, p := range g.Snapshot() {
		if p.ID != pid {
			continue
		}
		if p.AX.Joy != "cerebral" || p.Acting != "play chess" || p.ActX != 10 || p.ActY != 10 {
			t.Fatalf("pawn view joy %q acting %q at %d,%d", p.AX.Joy, p.Acting, p.ActX, p.ActY)
		}
		if len(p.AX.Tolerance) == 0 || p.AX.Tolerance[0].Kind != "cerebral" {
			t.Fatalf("tolerance %+v, want cerebral", p.AX.Tolerance)
		}
	}
}

// TestAX6WatchesFromTheArmchair (AX-6): a powered tube television is
// watched from the armchair in its watch area, facing the screen; an
// unpowered one is not watched at all.
func TestAX6WatchesFromTheArmchair(t *testing.T) {
	t.Parallel()
	for _, powered := range []bool{true, false} {
		g, pid := ax6Game(t)
		tv := buildNow(t, g, construction.TubeTV, item.Steel, construction.South, 15, 5)
		buildNow(t, g, construction.Armchair, item.Cloth, construction.North, 16, 8)
		if powered {
			onPower(t, g, 15, 5, 16, 4)
			if !g.powered(tv) {
				t.Fatal("the television has no power")
			}
		}
		if got := recOf(g).Seek(pid, 600); got != powered {
			t.Fatalf("powered %v: seek %v", powered, got)
		}
		if !powered {
			continue
		}
		j, _ := g.jobs.Get(pid)
		if j.Kind != job.WatchTelevision || j.TX != 16 || j.TY != 8 || j.DestX != 15 || j.DestY != 5 {
			t.Fatalf("job %+v, want watching from the armchair at 16,8 facing 15,5", j)
		}
	}
}

// TestAX6SaveKeepsTolerance (AX-6): tolerances survive a save and load,
// and the loaded game hashes the same.
func TestAX6SaveKeepsTolerance(t *testing.T) {
	t.Parallel()
	g, pid := ax6Game(t)
	buildNow(t, g, construction.HorseshoePin, item.Wood, construction.South, 10, 10)
	ax6Session(t, g, pid, 20000)
	g.Step(400)
	tol := recOf(g).Tolerance(pid)
	if tol.Pct(recreation.JoyDexterity) == 0 && tol.Milli[recreation.JoyDexterity] == 0 {
		t.Fatal("no dexterity tolerance after horseshoes")
	}
	l := saveLoad(t, g)
	if got := recOf(l).Tolerance(pid); got != tol {
		t.Fatalf("tolerance %+v after load, want %+v", got, tol)
	}
	if l.StateHash() != g.StateHash() {
		t.Fatal("the loaded game hashes differently")
	}
}

// TestAX6FoundPiecesComeFromRuins (AX-6): a generated shed's big screen
// television and telescope stand as ruin pieces nobody uses until the
// colony claims them (A1's Claim clears Ruin); neither can be built.
func TestAX6FoundPiecesComeFromRuins(t *testing.T) {
	t.Parallel()
	g, pid := ax6Game(t)
	g.placeRuin(mapgen.Ruin{Kind: mapgen.Shed, Screens: []mapgen.RuinFloor{{X: 12, Y: 4, Stuff: item.Steel}},
		Scopes: []mapgen.RuinFloor{{X: 20, Y: 12, Stuff: item.Steel}}})
	screen := g.layerBldgAt(13, 4, construction.LayerObject)
	scope := g.layerBldgAt(20, 12, construction.LayerObject)
	sb, _ := g.bldgs.Get(screen)
	tb, _ := g.bldgs.Get(scope)
	if sb.Kind != construction.BigScreenTV || !sb.Ruin || tb.Kind != construction.Telescope || !tb.Ruin {
		t.Fatalf("found %s (ruin %v) and %s (ruin %v)", sb.Kind, sb.Ruin, tb.Kind, tb.Ruin)
	}
	if recOf(g).Seek(pid, 600) {
		t.Fatal("a ruin's telescope is used before it is claimed")
	}
	for _, k := range []construction.Kind{construction.BigScreenTV, construction.Telescope} {
		if g.placeFrameStuff(k, item.Steel, construction.South, 5, 15); g.layerFrameAt(5, 15, construction.LayerObject) != 0 {
			t.Fatalf("%s takes a blueprint", k)
		}
	}
}
