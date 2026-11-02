package game

import (
	"testing"

	"rimworld-go/internal/application/game/books"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/book"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// AX-7: books and bookcases end to end in the game.

// ax7Game is a researched 24x16 colony with no work for anyone.
func ax7Game(t *testing.T) *Game {
	t.Helper()
	g := New(Config{Seed: 7, Width: 24, Height: 16})
	g.CompleteResearch()
	return g
}

// spawnBook lays a book of kind k at (x, y) and returns it.
func spawnBook(t *testing.T, g *Game, k item.Kind, x, y int) (entity.EntityID, item.Stack) {
	t.Helper()
	id, err := g.SpawnStack(k, 1, x, y)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := g.stacks.Get(id)
	return id, st
}

// TestAX7BooksAreMinted: a spawned book has a quality and a seed (its
// title and subjects), the same in another game of the same seed; other
// stacks never get a seed (the state hash is unchanged for them).
func TestAX7BooksAreMinted(t *testing.T) {
	t.Parallel()
	g, h := ax7Game(t), ax7Game(t)
	for _, k := range []item.Kind{item.Novel, item.Textbook, item.Schematic} {
		_, st := spawnBook(t, g, k, 3, 3)
		_, st2 := spawnBook(t, h, k, 3, 3)
		if st.Book == 0 || st.Quality == 0 || st != st2 {
			t.Fatalf("%s minted %+v / %+v", k, st, st2)
		}
		if book.Title(k, st.Book) == "" {
			t.Fatalf("%s has no title", k)
		}
		if k != item.Novel && book.Teaches(k, st.Book) == "" {
			t.Fatalf("%s teaches nothing", k)
		}
	}
	id, _ := g.SpawnStack(item.Wood, 10, 5, 5)
	if st, _ := g.stacks.Get(id); st.Book != 0 || st.Quality != 0 {
		t.Fatalf("wood minted %+v", st)
	}
	var iv ItemView
	for _, v := range g.World().Items {
		if v.Kind == "novel" {
			iv = v
		}
	}
	if iv.AX.Title == "" || iv.AX.Power < book.NovelPower[0] || iv.Quality == "" {
		t.Fatalf("novel view %+v", iv)
	}
}

// TestAX7BookcaseStoresBooksAndGivesTheBonus: a bookcase is books-only
// storage for 10 (5 a cell); haulers shelve a loose book; the books in it
// give its room the reading bonus on the bookcase's view and at the
// room's research bench.
func TestAX7BookcaseStoresBooksAndGivesTheBonus(t *testing.T) {
	t.Parallel()
	g := ax7Game(t)
	hut(t, g, 2, 2, 12, 10)
	if !g.BuildNow("bookcase", "wood", "south", 4, 4) || !g.BuildNow("small bookcase", "wood", "south", 8, 4) ||
		!g.BuildNow("research bench", "wood", "south", 6, 7) {
		t.Fatal("not built")
	}
	bc := g.layerBldgAt(4, 4, construction.LayerObject)
	small := g.layerBldgAt(8, 4, construction.LayerObject)
	bench := g.layerBldgAt(6, 7, construction.LayerObject)
	if _, _, c := g.storeView(bc); c != 10 {
		t.Fatalf("bookcase capacity %d, want 10", c)
	}
	if _, _, c := g.storeView(small); c != 5 {
		t.Fatalf("small bookcase capacity %d, want 5", c)
	}
	if z, ok := g.storageAt(4, 4); !ok || z.Kind != zone.KindBookcase || z.Allows(item.Wood) || !z.Allows(item.Novel) {
		t.Fatalf("bookcase zone %+v", z)
	}
	base := g.lab.Factor(bench)
	for i := 0; i < 3; i++ {
		spawnBook(t, g, item.Novel, 4, 4)
	}
	if got := buildingAX(g, bc).ReadingBonus; got != 24 {
		t.Fatalf("three shelved books: bonus %d, want 24", got)
	}
	if got := g.lab.Factor(bench); got != base*1024/1000 {
		t.Fatalf("bench factor %d, want %d", got, base*1024/1000)
	}
	ada, _ := g.SpawnNamedPawn(7, 6, "Ada")
	loose, _ := spawnBook(t, g, item.Textbook, 10, 8)
	for i := 0; i < 1500; i++ {
		if i%100 == 0 {
			g.SetNeeds(ada, needs.Full, needs.Full)
			g.joy[ada] = recreation.Full // hauling, not reading
		}
		g.Step(1)
		if st, _ := g.stacks.Get(loose); st.Carrier == 0 && g.layerBldgAt(st.X, st.Y, construction.LayerObject) != 0 {
			if b, _ := g.bldgs.Get(g.layerBldgAt(st.X, st.Y, construction.LayerObject)); books.IsBookcase(b.Kind) {
				t.Logf("shelved at %d,%d after %d ticks", st.X, st.Y, i)
				return
			}
		}
	}
	st, _ := g.stacks.Get(loose)
	t.Fatalf("the loose textbook was not shelved: %+v", st)
}

// buildingAX is building id's AX view.
func buildingAX(g *Game, id entity.EntityID) (out struct{ ReadingBonus int }) {
	for _, b := range g.World().Buildings {
		if b.ID == id {
			out.ReadingBonus = b.AX.ReadingBonus
		}
	}
	return out
}

// readUntil steps until id reads (job.Read in Count 1) or n ticks pass,
// keeping it fed and rested.
func readUntil(g *Game, id entity.EntityID, n int, done func(job.Record) bool) job.Record {
	var j job.Record
	for i := 0; i < n; i++ {
		if i%200 == 0 {
			g.SetNeeds(id, needs.Full, needs.Full)
		}
		g.Step(1)
		if j, _ = g.jobs.Get(id); done(j) {
			break
		}
	}
	return j
}

// TestAX7ReadingIsRecreationAndLearning: a bored colonist picks up a
// novel, sits on the chair beside it and reads; recreation climbs and
// Intellectual trains, and its view names the book. A textbook trains its
// skills, a schematic puts points into its project.
func TestAX7ReadingIsRecreationAndLearning(t *testing.T) {
	t.Parallel()
	g := ax7Game(t)
	g.BuildNow("chair", "wood", "south", 9, 5)
	ada, _ := g.SpawnNamedPawn(5, 5, "Ada")
	g.labor.Set(ada, 0)
	novel, st := spawnBook(t, g, item.Novel, 7, 5)
	g.joy[ada] = 1000
	started := false
	for i := 0; i < 50 && !started; i++ {
		started = booksOf(g).Seek(ada)
	}
	if !started {
		t.Fatal("a bored colonist with a novel in reach never reads")
	}
	xp0 := g.skillOf(ada, skill.Intellectual)
	j := readUntil(g, ada, 400, func(j job.Record) bool { return j.Kind == job.Read && j.Count == 1 && j.State == job.Do })
	if j.Carry != novel {
		t.Fatalf("not reading the novel: %+v", j)
	}
	if p, _ := g.positions.Get(ada); p.X != 9 || p.Y != 5 {
		t.Fatalf("reads at %d,%d, not on the chair", p.X, p.Y)
	}
	var pv PawnView
	for _, v := range g.Snapshot() {
		if v.ID == ada {
			pv = v
		}
	}
	if pv.AX.Reading != book.Title(item.Novel, st.Book) {
		t.Fatalf("reading view %q", pv.AX.Reading)
	}
	joy0 := g.joy[ada]
	readUntil(g, ada, 1000, func(job.Record) bool { return false })
	if g.joy[ada] <= joy0 {
		t.Fatalf("joy %d -> %d", joy0, g.joy[ada])
	}
	if xp1 := g.skillOf(ada, skill.Intellectual); xp1 == xp0 {
		t.Fatal("reading trained no Intellectual")
	}
	// a textbook teaches its skills; a schematic studies its project
	tb, ts := spawnBook(t, g, item.Textbook, 3, 3)
	sk := book.Skills(item.Textbook, ts.Book)[0]
	g.sk.skills[ada][sk].Level = 0
	before := g.skillOf(ada, sk)
	q, _ := quality.FromStored(ts.Quality)
	rec, why := booksOf(g).Order(ada, tb)
	if why != "" {
		t.Fatal(why)
	}
	rec.Forced = true
	g.interrupt(ada, job.Cancelled)
	g.startJob(ada, rec)
	readUntil(g, ada, 1500, func(job.Record) bool { return false })
	if after := g.skillOf(ada, sk); after == before && book.Cap(q) > 0 {
		t.Fatalf("textbook taught nothing of %s", sk.Label())
	}
	sc, ss := spawnBook(t, g, item.Schematic, 3, 4)
	p := book.Projects(item.Schematic, ss.Book)[0]
	g.lab.Prog[p] = 0
	rec, _ = booksOf(g).Order(ada, sc)
	rec.Forced = true
	g.interrupt(ada, job.Cancelled)
	g.startJob(ada, rec)
	readUntil(g, ada, 1500, func(job.Record) bool { return false })
	if g.lab.Prog[p] == 0 {
		t.Fatalf("schematic put nothing into %s", research.DefOf(p).Label)
	}
}

// TestAX7ReadingDoesNotFlap (hysteresis): reading starts only below
// Entertained and, once begun, lasts the hour (or until the need is
// full); a colonist at Entertained never picks a book.
func TestAX7ReadingDoesNotFlap(t *testing.T) {
	t.Parallel()
	g := ax7Game(t)
	ada, _ := g.SpawnNamedPawn(5, 5, "Ada")
	g.labor.Set(ada, 0)
	spawnBook(t, g, item.Textbook, 6, 5)
	g.joy[ada] = recreation.Entertained
	for i := 0; i < 50; i++ {
		if booksOf(g).Seek(ada) {
			t.Fatal("an entertained colonist picked a book")
		}
	}
	g.joy[ada] = recreation.Entertained - 1
	started := false
	for i := 0; i < 50 && !started; i++ {
		started = booksOf(g).Seek(ada)
	}
	if !started {
		t.Fatal("never started")
	}
	j := readUntil(g, ada, 300, func(j job.Record) bool { return j.Kind == job.Read && j.Count == 1 })
	read, stops := 0, 0
	for i := 0; i < books.ReadTicks+200; i++ {
		g.Step(1)
		if i%200 == 0 {
			g.SetNeeds(ada, needs.Full, needs.Full)
		}
		nj, _ := g.jobs.Get(ada)
		if nj.Kind == job.Read {
			read++
		} else if j.Kind == job.Read {
			stops++
		}
		j = nj
	}
	if stops > 1 || read < books.ReadTicks*9/10 && g.joy[ada] < recreation.Full {
		t.Fatalf("read %d ticks with %d stops (joy %d)", read, stops, g.joy[ada])
	}
}

// TestAX7ReadOrderAndRuinBooks: the float menu offers "Read <title>" on
// a book; Prioritize starts a forced read; ruins and raiders leave books
// minted like any other.
func TestAX7ReadOrderAndRuinBooks(t *testing.T) {
	t.Parallel()
	g := ax7Game(t)
	ada, _ := g.SpawnNamedPawn(5, 5, "Ada")
	g.labor.Set(ada, 0)
	_, st := spawnBook(t, g, item.Novel, 8, 5)
	want := "Read " + book.Title(item.Novel, st.Book)
	var opt OrderOption
	for _, o := range g.OrdersAt(ada, 8, 5) {
		if o.Label == want {
			opt = o
		}
	}
	if opt.Label == "" || opt.Disabled || opt.Cmd.Order != command.OrderRead {
		t.Fatalf("no %q order: %+v", want, g.OrdersAt(ada, 8, 5))
	}
	g.Enqueue(opt.Cmd)
	g.Step(1)
	if j, _ := g.jobs.Get(ada); j.Kind != job.Read || !j.Forced {
		t.Fatalf("order started %+v", j)
	}
	n := 0
	for id := uint64(1); id < 2000; id++ {
		if _, ok := books.RaidLoot(g.streams.seed, id); ok {
			n++
		}
	}
	if n < 2000*books.RaidBookPct/200 || n > 2000*books.RaidBookPct*2/100 {
		t.Fatalf("raid books %d of 2000", n)
	}
}
