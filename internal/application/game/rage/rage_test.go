package rage

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/animal"
)

// The chase warning's words (moved from the game's chase_test.go).
func TestChaseText(t *testing.T) {
	t.Parallel()
	if chaseLabel(animal.TimberWolf) != "timber wolf" {
		t.Fatal("label")
	}
	if got := ChaseText(animal.ArcticFox, "Ada", false); got != "An arctic fox is chasing Ada!" {
		t.Fatalf("article: %q", got)
	}
	if got := ChaseText(animal.WildBoar, "Ada", true); !strings.HasSuffix(got, " charges Ada!") || strings.Contains(got, "_") {
		t.Fatalf("charge text: %q", got)
	}
	if WithArticle("hare") != "a hare" || WithArticle("ibex") != "an ibex" {
		t.Fatal("article")
	}
}

// KeepChaseWarned holds the re-warn hour while the warned chase is on, and
// never for another target.
func TestKeepChaseWarned(t *testing.T) {
	t.Parallel()
	a := animal.State{Mode: animal.Attack, Target: 7, ChaseWarnedTarget: 7, ChaseWarnedAt: 10}
	KeepChaseWarned(&a, 50)
	if a.ChaseWarnedAt != 50 {
		t.Fatalf("on the warned chase: %d", a.ChaseWarnedAt)
	}
	a.Target = 8
	KeepChaseWarned(&a, 90)
	if a.ChaseWarnedAt != 50 {
		t.Fatalf("another target: %d", a.ChaseWarnedAt)
	}
}

// FightBack keeps a live foe and takes the attacker otherwise; HurtBlind
// wakes a sleeper and runs a wild skittish animal.
func TestFightBackAndHurtBlind(t *testing.T) {
	t.Parallel()
	a := animal.State{Mode: animal.Attack, Target: 3}
	FightBack(&a, 9, true)
	if a.Target != 3 || a.Mode != animal.Attack {
		t.Fatalf("keeps its live foe: %+v", a.Target)
	}
	FightBack(&a, 9, false)
	if a.Target != 9 {
		t.Fatalf("a dead foe: turns on the attacker, got %d", a.Target)
	}
	d := animal.State{Species: animal.Deer, Mode: animal.Rest}
	HurtBlind(&d, 100)
	if d.Mode != animal.Flee || d.FleeUntil != 100+animal.HurtFleeTicks {
		t.Fatalf("a deer hit blind runs: mode %v until %d", d.Mode, d.FleeUntil)
	}
	if HuntRadius(animal.Def{HuntRadius: 5}) != 5 || HuntRadius(animal.Def{}) != smallPreyRadius {
		t.Fatal("hunt radius")
	}
	if DistSq(Pos{X: 1, Y: 1}, Pos{X: 4, Y: 5}) != 25 {
		t.Fatal("distance")
	}
}
