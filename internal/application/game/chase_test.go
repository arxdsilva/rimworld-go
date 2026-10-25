package game

import (
	"bytes"
	"strings"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-017 §11: an animal that starts chasing a colonist posts one danger
// message (the UI pauses on it); repeats need a new target, or the chase
// over for an hour.

// dangerAbout lists the danger messages about animal id.
func dangerAbout(g *Game, id entity.EntityID) []MessageView {
	var out []MessageView
	for _, m := range g.messages {
		if m.Danger && m.Focus == id {
			out = append(out, m)
		}
	}
	return out
}

// chaseWarnings counts the chase warnings about animal id.
func chaseWarnings(g *Game, id entity.EntityID) int {
	n := 0
	for _, m := range dangerAbout(g, id) {
		if strings.Contains(m.Text, " is chasing ") || strings.Contains(m.Text, " charges ") {
			n++
		}
	}
	return n
}

// chaseScene: a wild animal of species s and a standing colonist Ada 3
// tiles away on an empty map.
func chaseFixture(t *testing.T, s string) (*Game, entity.EntityID, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 4, Width: 40, Height: 20})
	bareGround(g)
	beast, err := g.SpawnAnimal(s, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	ada := draftedPawn(t, g, 13, 10)
	g.names.Set(ada, "Ada")
	g.SetNeeds(ada, needs.Full, needs.Full)
	return g, beast, ada
}

func TestHungryPredatorChaseWarns(t *testing.T) {
	g, wolf, ada := chaseFixture(t, "timber_wolf")
	setAnimal(g, wolf, func(a *animal.State) { a.Food, a.NextDecide = 2000, 0 })
	g.Step(1)
	if a := animalOf(g, wolf); a.Target != ada || a.HuntSince == 0 {
		t.Fatalf("a hungry wolf with no prey goes for Ada: %+v", a)
	}
	ms := dangerAbout(g, wolf)
	if len(ms) != 1 || ms[0].Text != "A timber wolf is chasing Ada!" {
		t.Fatalf("one warning: %+v", ms)
	}
	if ms[0].Focus != wolf || ms[0].X != 10 || ms[0].Y != 10 {
		t.Fatalf("the message is where the wolf set off, and follows it: %+v", ms[0])
	}
	for i := 0; i < 200; i++ {
		healPawn(g, ada)
		g.Step(1)
	}
	if n := chaseWarnings(g, wolf); n != 1 {
		t.Fatalf("the chase goes on without new warnings: %d", n)
	}
	if a := animalOf(g, wolf); a.ChaseWarnedTarget != ada || a.ChaseWarnedAt == 0 {
		t.Fatalf("warn state %+v", a)
	}
}

func TestRevengeChaseWarns(t *testing.T) {
	g, wolf, ada := chaseFixture(t, "timber_wolf")
	setAnimal(g, wolf, func(a *animal.State) { a.Attacker, a.NextDecide = ada, 1<<40 })
	g.Step(1)
	if a := animalOf(g, wolf); a.Target != ada || a.Mode != animal.Attack {
		t.Fatalf("revenge on Ada: %+v", a)
	}
	if ms := dangerAbout(g, wolf); len(ms) != 1 || ms[0].Text != "A timber wolf is chasing Ada!" {
		t.Fatalf("one warning: %+v", ms)
	}
}

func TestChargeWarnsOnceAcrossRepeatedCharges(t *testing.T) {
	g := New(Config{Seed: 13, Width: 32, Height: 20})
	cougar, ada := chargeScene(t, g, "cougar")
	g.names.Set(ada, "Ada")
	var charges, warnings int
	var lastChase uint64
	for i := 0; i < 16*animal.ChargeCooldown; i++ {
		keepCalm(g, cougar)
		healPawn(g, ada)
		before := animalOf(g, cougar)
		if before.ChargeSince != 0 { // end each charge at once: the colonist steps away
			g.positions.Set(ada, Position{X: 30, Y: 18})
			g.invalidateCreatures()
		} else if p := pos(g, ada); p.X != 17 {
			g.positions.Set(ada, Position{X: 17, Y: 10})
			g.invalidateCreatures()
		}
		g.messages = g.messages[:0]
		g.Step(1)
		now := uint64(g.clock.Now())
		a := animalOf(g, cougar)
		if a.ChargeSince != 0 && before.ChargeSince == 0 {
			charges++
		}
		for _, m := range dangerAbout(g, cougar) {
			warnings++
			if m.Text != "A cougar charges Ada!" {
				t.Fatalf("text %q", m.Text)
			}
			if warnings > 1 && now-lastChase < clock.TicksPerHour {
				t.Fatalf("tick %d: re-warned %d ticks after the last chase", i, now-lastChase)
			}
		}
		if a.Mode == animal.Attack && a.Target == ada {
			lastChase = now
		}
	}
	if charges < 3 || warnings == 0 || warnings >= charges {
		t.Fatalf("charges %d, warnings %d: repeated charges must not re-warn", charges, warnings)
	}
}

func TestNewTargetRewarns(t *testing.T) {
	g, wolf, ada := chaseFixture(t, "timber_wolf")
	bo := draftedPawn(t, g, 10, 13)
	g.names.Set(bo, "Bo")
	a := animalOf(g, wolf)
	a.Mode, a.Target = animal.Attack, ada
	g.warnChase(wolf, &a, ada, false)
	g.warnChase(wolf, &a, ada, false) // the same chase: once
	a.Target = bo
	g.warnChase(wolf, &a, bo, false)
	g.animals.Set(wolf, a)
	ms := dangerAbout(g, wolf)
	if len(ms) != 2 || ms[1].Text != "A timber wolf is chasing Bo!" {
		t.Fatalf("a new target warns again: %+v", ms)
	}
	// the same target again: only once the chase has been over an hour
	a.Mode, a.Target = animal.Wander, 0
	g.animals.Set(wolf, a)
	g.Step(1)
	g.clock.Advance(clock.TicksPerHour - 10)
	a = animalOf(g, wolf)
	a.Mode, a.Target = animal.Attack, bo
	g.warnChase(wolf, &a, bo, false)
	if n := len(dangerAbout(g, wolf)); n != 2 {
		t.Fatalf("under an hour after the chase: no warning (%d)", n)
	}
	// that chase ends too; an hour later it is a new one
	a.Mode, a.Target = animal.Wander, 0
	g.clock.Advance(clock.TicksPerHour)
	a.Mode, a.Target = animal.Attack, bo
	g.warnChase(wolf, &a, bo, false)
	if n := len(dangerAbout(g, wolf)); n != 3 {
		t.Fatalf("an hour after the chase ended: warned again (%d)", n)
	}
}

func TestFightBackWarnsOnceAndManhuntersNotTwice(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		g := New(Config{Seed: seed, Width: 40, Height: 20})
		bareGround(g)
		bison, _ := g.SpawnAnimal("bison", 10, 10)
		ada := draftedPawn(t, g, 11, 10)
		g.names.Set(ada, "Ada")
		now := uint64(g.clock.Now())
		g.hurt(bison, 2, ada)
		a := animalOf(g, bison)
		if a.Target != ada {
			t.Fatalf("seed %d: the bison fights back: %+v", seed, a)
		}
		// one message: the chase, or the rage (never both)
		ms := dangerAbout(g, bison)
		want := "A bison is chasing Ada!"
		if a.Enraged(now) {
			want = "A bison has gone manhunter!"
		}
		if len(ms) != 1 || ms[0].Text != want {
			t.Fatalf("seed %d: one danger message %q: %+v", seed, want, ms)
		}
		// the same fight goes on: no new chase warning (a rage now has
		// only its own message)
		wasEnraged := a.Enraged(now)
		g.hurt(bison, 2, ada)
		a = animalOf(g, bison)
		ms = dangerAbout(g, bison)
		extra := 0
		if a.Enraged(now) && !wasEnraged {
			extra = 1
			if ms[len(ms)-1].Text != "A bison has gone manhunter!" {
				t.Fatalf("seed %d: %+v", seed, ms)
			}
		}
		if len(ms) != 1+extra || chaseWarnings(g, bison) > 1 {
			t.Fatalf("seed %d: the same fight warned again: %+v", seed, ms)
		}
	}
	// an enraged animal's own chase is never warned again
	g, wolf, ada := chaseFixture(t, "timber_wolf")
	a := animalOf(g, wolf)
	g.enrage(wolf, &a, ada, uint64(g.clock.Now()))
	g.animals.Set(wolf, a)
	for i := 0; i < 100; i++ {
		healPawn(g, ada)
		g.Step(1)
	}
	if n := chaseWarnings(g, wolf); n != 0 {
		t.Fatalf("a manhunter is not warned as a chase: %d", n)
	}
}

func TestChaseWarnStateSurvivesSave(t *testing.T) {
	g, wolf, ada := chaseFixture(t, "timber_wolf")
	setAnimal(g, wolf, func(a *animal.State) { a.Food, a.NextDecide = 2000, 0 })
	g.Step(1)
	if chaseWarnings(g, wolf) != 1 {
		t.Fatal("warned")
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "chase"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	want, got := animalOf(g, wolf), animalOf(h, wolf)
	if got.ChaseWarnedTarget != ada || got.ChaseWarnedAt != want.ChaseWarnedAt {
		t.Fatalf("warn state lost: %+v", got)
	}
	h.messages = nil
	a := animalOf(h, wolf)
	h.warnChase(wolf, &a, ada, false)
	if n := chaseWarnings(h, wolf); n != 0 {
		t.Fatalf("a loaded chase is not warned again: %d", n)
	}
}

// noteChase (combat.go's hook) reads and writes the stored animal.
func TestNoteChaseUsesTheStore(t *testing.T) {
	g, wolf, ada := chaseFixture(t, "timber_wolf")
	setAnimal(g, wolf, func(a *animal.State) { a.Mode, a.Target = animal.Attack, ada })
	g.noteChase(wolf, ada)
	g.noteChase(wolf, ada)
	if n := chaseWarnings(g, wolf); n != 1 {
		t.Fatalf("warnings %d", n)
	}
	if a := animalOf(g, wolf); a.ChaseWarnedTarget != ada {
		t.Fatalf("stored %+v", a)
	}
	g.noteChase(9999, ada) // gone: nothing
	// raiders (and visitors, kept with them) are no colonists
	raider := spawnRaiderAt(t, g, 30, 15, 1)
	setAnimal(g, wolf, func(a *animal.State) { a.Target = raider })
	g.noteChase(wolf, raider)
	if n := chaseWarnings(g, wolf); n != 1 {
		t.Fatalf("a raider chased is no warning: %d", n)
	}
	if a := animalOf(g, wolf); chaseLabel(a.Species) != "timber wolf" {
		t.Fatal("label")
	}
	if got := chaseText(animal.ArcticFox, "Ada", false); got != "An arctic fox is chasing Ada!" {
		t.Fatalf("article: %q", got)
	}
	if got := chaseText(animal.WildBoar, "Ada", true); !strings.HasSuffix(got, " charges Ada!") || strings.Contains(got, "_") {
		t.Fatalf("charge text: %q", got)
	}
}
