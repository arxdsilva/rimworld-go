package cue

import (
	"reflect"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/projectile"
)

func TestHeavyTurretAndMortarCues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		tr   game.TraceView
		want []string
	}{
		{"autocannon fired", game.TraceView{Weapon: "autocannon", FromBuilding: true, Stage: projectile.Fired}, []string{"sfx.autocannon_shot"}},
		{"autocannon whole", game.TraceView{Weapon: "autocannon", FromBuilding: true, Hit: true, Damage: 9}, []string{"sfx.autocannon_shot", "sfx.bullet_flesh"}},
		{"slug fired", game.TraceView{Weapon: "slug turret", FromBuilding: true, Stage: projectile.Fired}, []string{"sfx.slug_turret_shot"}},
		{"mortar fired", game.TraceView{Weapon: "mortar", FromBuilding: true, Stage: projectile.Fired}, []string{"sfx.mortar_launch", "sfx.mortar_incoming"}},
		{"mortar landing is the blast's", game.TraceView{Weapon: "mortar", Stage: projectile.Landed, Hit: true, Damage: 3}, nil},
	}
	for _, c := range cases {
		if got := combatIDs(c.tr); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: cues %v, want %v", c.name, got, c.want)
		}
	}
}

func TestMortarWhistleSoundsAtTheTarget(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	r.b.TraceSound(game.TraceView{Weapon: "mortar", Stage: projectile.Fired, FromX: 1, FromY: 1, ToX: 9, ToY: 7})
	c := r.b.Take()
	if len(c) != 2 || c[0].X != 1.5 || c[1].ID != "sfx.mortar_incoming" || c[1].X != 9.5 || c[1].Y != 7.5 {
		t.Fatalf("cues %+v", c)
	}
}

func TestBenchWorkLoopsNearestOnly(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	acting := "work at " + construction.FueledSmithy.String()
	var ps []game.PawnView
	for i := 0; i < 4; i++ {
		ps = append(ps, game.PawnView{Acting: acting, ActX: 4 + i, ActY: 4})
	}
	ps = append(ps, game.PawnView{Acting: "work at " + construction.ArtBench.String(), ActX: 8, ActY: 8},
		game.PawnView{Acting: "work at crafting spot", ActX: 2, ActY: 2}, game.PawnView{Acting: "mine"})
	r.b.ListenWorld(r.v, game.WorldView{}, ps)
	want := map[string]Loop{}
	r.b.Machines(r.v, want, game.WorldView{}, camera.ColonyViewport(1280, 720))
	smithy, sculpt := 0, 0
	for _, l := range want {
		switch l.ID {
		case "amb.bench_smithy":
			smithy++
		case "amb.bench_sculpt":
			sculpt++
		default:
			t.Fatalf("unexpected loop %+v", l)
		}
	}
	if smithy != MaxMachineLoops || sculpt != 1 {
		t.Fatalf("%d smithy, %d sculpt loops in %+v", smithy, sculpt, want)
	}
	r.b.ListenWorld(r.v, game.WorldView{}, nil) // the crafters stop
	want = map[string]Loop{}
	r.b.Machines(r.v, want, game.WorldView{}, camera.ColonyViewport(1280, 720))
	if len(want) != 0 {
		t.Fatalf("loops after work stopped: %+v", want)
	}
}

func TestEveryBenchKindHasALoop(t *testing.T) {
	t.Parallel()
	got := map[string]bool{}
	for _, id := range benchLoops {
		got[id] = true
	}
	if len(got) != 6 {
		t.Fatalf("%d distinct bench loops, want 6: %v", len(got), got)
	}
}

func TestTurretRearmCueOnceAndNotOnWear(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	frame := func(barrel int) game.WorldView {
		return game.WorldView{Buildings: []game.BuildingView{{ID: 3, Kind: "autocannon", X: 5, Y: 5, Barrel: barrel}}}
	}
	ids := func(barrel int) []string {
		r.b.ListenWorld(r.v, frame(barrel), nil)
		var out []string
		for _, c := range r.b.Take() {
			out = append(out, c.ID)
		}
		return out
	}
	if got := ids(300); len(got) != 0 {
		t.Fatalf("first frame: %v", got)
	}
	if got := ids(290); len(got) != 0 {
		t.Fatalf("wear: %v", got)
	}
	if got := ids(1000); !reflect.DeepEqual(got, []string{"sfx.turret_rearm"}) {
		t.Fatalf("fresh barrel: %v", got)
	}
	if got := ids(1000); len(got) != 0 {
		t.Fatalf("heard twice: %v", got)
	}
}

func TestChargeArmsOnceWhenSet(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	kind := construction.IEDHighExplosive.String()
	ids := func(bs ...game.BuildingView) []string {
		r.b.ListenWorld(r.v, game.WorldView{Buildings: bs}, nil)
		var out []string
		for _, c := range r.b.Take() {
			out = append(out, c.ID)
		}
		return out
	}
	old := game.BuildingView{ID: 1, Kind: kind, X: 2, Y: 2}
	if got := ids(old); len(got) != 0 {
		t.Fatalf("a charge already there: %v", got)
	}
	fresh := game.BuildingView{ID: 2, Kind: kind, X: 3, Y: 2}
	if got := ids(old, fresh); !reflect.DeepEqual(got, []string{"sfx.ied_arm"}) {
		t.Fatalf("new charge: %v", got)
	}
	if got := ids(old, fresh); len(got) != 0 {
		t.Fatalf("heard twice: %v", got)
	}
}

func TestAutodoorCues(t *testing.T) {
	t.Parallel()
	r := newBus(1)
	door := func(held bool) game.WorldView {
		return game.WorldView{Buildings: []game.BuildingView{{ID: 4, Kind: autodoorKind, X: 6, Y: 6, HoldOpen: held}}}
	}
	ids := func(held bool) []string {
		r.b.ListenWorld(r.v, door(held), nil)
		var out []string
		for _, c := range r.b.Take() {
			out = append(out, c.ID)
		}
		return out
	}
	ids(false)
	if got := ids(true); !reflect.DeepEqual(got, []string{"sfx.autodoor_open"}) {
		t.Fatalf("open: %v", got)
	}
	if got := ids(true); len(got) != 0 {
		t.Fatalf("held open repeats: %v", got)
	}
	if got := ids(false); !reflect.DeepEqual(got, []string{"sfx.autodoor_close"}) {
		t.Fatalf("close: %v", got)
	}
}
