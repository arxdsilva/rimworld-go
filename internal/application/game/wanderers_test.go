package game

// wanderers_test.go: new wanderers after a game over (MAP-19, wiki: Endings).

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// goneGame is a landed colony with every colonist dead, a tick later.
func goneGame(t *testing.T) *Game {
	t.Helper()
	g := visitorGame(t)
	for _, id := range slices.Clone(g.pawnIDs) {
		g.die(id, 0)
	}
	g.Step(2)
	if !everyoneGone(g) {
		t.Fatal("fixture: someone is left")
	}
	return g
}

// livingColonists are the colonists alive on g's current map (the dead keep
// their place on the list), ascending.
func livingColonists(g *Game) []entity.EntityID {
	return slices.DeleteFunc(slices.Clone(g.pawnIDs), func(id entity.EntityID) bool { return !g.alive(id) })
}

// sendWanderers enqueues the seeds as a group and applies it.
func sendWanderers(g *Game, seeds ...uint64) {
	parts := make([]string, len(seeds))
	for i, s := range seeds {
		parts[i] = fmt.Sprint(s)
	}
	g.Enqueue(command.Command{Kind: command.NewWanderers, Name: strings.Join(parts, ",")})
	g.ApplyPending()
}

// waitOver rewinds the game over's start so the eight hours have passed.
func waitOver(g *Game) {
	g.ng.setup.GoneAt = uint64(g.clock.Now()) - visit.WanderersWait
}

func TestNormalColonyHasNoWanderersOption(t *testing.T) {
	t.Parallel()
	g := visitorGame(t)
	g.Step(2)
	if g.ng.setup.GoneAt != 0 {
		t.Fatalf("gone at %d with colonists here", g.ng.setup.GoneAt)
	}
	ax := g.World().AX
	if ax.WanderersOK || ax.EveryoneGone {
		t.Fatalf("%+v", ax)
	}
	if ax.WanderersLeft != visit.WandererGroups {
		t.Fatalf("left %d", ax.WanderersLeft)
	}
	n := len(livingColonists(g))
	sendWanderers(g, 1)
	if len(livingColonists(g)) != n || g.ng.setup.WandererGroups != 0 {
		t.Fatal("wanderers came while the colony lives")
	}
}

func TestLegacyColonyHasNoWanderersOption(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 20, Height: 20})
	g.Step(2)
	if !everyoneGone(g) {
		t.Fatal("fixture")
	}
	if ax := g.World().AX; ax.WanderersOK || ax.WanderersLeft != 0 {
		t.Fatalf("%+v", ax)
	}
	sendWanderers(g, 1)
	if len(livingColonists(g)) != 0 {
		t.Fatal("wanderers came to a legacy colony")
	}
}

func TestWanderersWaitEightHours(t *testing.T) {
	t.Parallel()
	g := goneGame(t)
	s := g.ng.setup
	at := s.GoneAt
	if at == 0 || at > uint64(g.clock.Now()) {
		t.Fatalf("gone at %d, now %d", at, g.clock.Now())
	}
	if want := uint64(8 * clock.TicksPerHour); visit.WanderersWait != want {
		t.Fatalf("wait %d, want %d", visit.WanderersWait, want)
	}
	g.Step(10)
	if s.GoneAt != at {
		t.Fatalf("the start moved %d -> %d", at, s.GoneAt)
	}
	ax := g.World().AX
	wantIn := int(at + visit.WanderersWait - uint64(g.clock.Now()))
	if !ax.EveryoneGone || ax.WanderersOK || ax.WanderersIn != wantIn || ax.WanderersLeft != 7 {
		t.Fatalf("%+v, want in %d", ax, wantIn)
	}
	// refused early, and the refusal says why
	sendWanderers(g, 11)
	if len(livingColonists(g)) != 0 || s.WandererGroups != 0 {
		t.Fatal("wanderers came early")
	}
	if !messageHas(g, "eight hours") {
		t.Fatalf("no reason among %+v", g.messageViews())
	}
	// one tick short, then open
	g.ng.setup.GoneAt = uint64(g.clock.Now()) - visit.WanderersWait + 1
	if ax := g.World().AX; ax.WanderersOK || ax.WanderersIn != 1 {
		t.Fatalf("%+v", ax)
	}
	waitOver(g)
	if ax := g.World().AX; !ax.WanderersOK || ax.WanderersIn != 0 {
		t.Fatalf("%+v", ax)
	}
}

func TestWanderersArriveAsColonistsWalkingIn(t *testing.T) {
	t.Parallel()
	g := goneGame(t)
	waitOver(g)
	s := g.ng.setup
	rules := g.World().AX.WanderersRules
	seeds := []uint64{101, 202}
	sendWanderers(g, seeds...)
	ids := livingColonists(g)
	if len(ids) != 2 {
		t.Fatalf("%d arrived", len(ids))
	}
	if s.WandererGroups != 1 || s.GoneAt != 0 || everyoneGone(g) {
		t.Fatalf("groups %d, gone at %d, gone %v", s.WandererGroups, s.GoneAt, everyoneGone(g))
	}
	w, h := g.grid.Width(), g.grid.Height()
	for i, id := range ids {
		want := WandererPawn(seeds[i], rules)
		if p := g.personOf(id); p == nil || p.Where != whereColony {
			t.Fatalf("%d: %+v", id, p)
		}
		if b := g.sk.bios[id]; b.First != want.First || b.Childhood != want.Childhood {
			t.Fatalf("%d: %+v, want the previewed %+v", id, b, want)
		}
		if WandererBio(seeds[i], rules).First != want.First {
			t.Fatal("the preview's bio differs")
		}
		pos, _ := g.positions.Get(id)
		if min(pos.X, pos.Y, w-1-pos.X, h-1-pos.Y) > 15 {
			t.Fatalf("%d did not come from an edge: %+v", id, pos)
		}
		if !g.colonist(id) {
			t.Fatalf("%d is no colonist", id)
		}
	}
	if !slices.ContainsFunc(g.letters.list, func(l savedLetter) bool {
		return l.Title == wanderersTitle && ids[0] == l.Focus
	}) {
		t.Fatalf("no letter: %+v", g.letters.list)
	}
	g.Step(50)
	if v := g.World().AX; v.EveryoneGone || v.WanderersLeft != 6 || v.WanderersOK {
		t.Fatalf("%+v", v)
	}
	// they work: a job or a walk, undrafted
	if g.drafted(ids[0]) {
		t.Fatal("drafted")
	}
}

func TestSevenGroupsAtMost(t *testing.T) {
	t.Parallel()
	g := goneGame(t)
	s := g.ng.setup
	for n := 0; n < visit.WandererGroups; n++ {
		waitOver(g)
		sendWanderers(g, uint64(1000+n))
		ids := livingColonists(g)
		if s.WandererGroups != n+1 || len(ids) != 1 {
			t.Fatalf("group %d: groups %d, %d pawns", n, s.WandererGroups, len(ids))
		}
		g.die(ids[0], 0)
		g.Step(2)
		if !everyoneGone(g) {
			t.Fatal("fixture: the group lives")
		}
	}
	waitOver(g)
	if ax := g.World().AX; ax.WanderersOK || ax.WanderersLeft != 0 {
		t.Fatalf("%+v", ax)
	}
	sendWanderers(g, 9)
	if len(livingColonists(g)) != 0 {
		t.Fatal("an eighth group came")
	}
}

func TestWanderersRefuseBadSeeds(t *testing.T) {
	t.Parallel()
	g := goneGame(t)
	waitOver(g)
	limit := g.World().AX.WanderersMax
	if limit != g.ng.fx.People || limit < 1 {
		t.Fatalf("limit %d for %d people", limit, g.ng.fx.People)
	}
	tooMany := make([]string, limit+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprint(i + 1)
	}
	for _, name := range []string{"", "abc", "1,,2", "1,1", "-3", strings.Join(tooMany, ",")} {
		g.Enqueue(command.Command{Kind: command.NewWanderers, Name: name})
		g.ApplyPending()
		if len(livingColonists(g)) != 0 || g.ng.setup.WandererGroups != 0 {
			t.Fatalf("%q was accepted", name)
		}
	}
}

// TestGoneAtResetsWhenSomeoneExists: the wait starts over if someone turns up.
func TestGoneAtResetsWhenSomeoneExists(t *testing.T) {
	t.Parallel()
	g := goneGame(t)
	x, y := g.home()
	id, err := g.SpawnNamedPawn(x, y, "Latecomer")
	if err != nil {
		t.Fatal(err)
	}
	g.Step(1)
	if g.ng.setup.GoneAt != 0 {
		t.Fatal("still counting with someone here")
	}
	g.die(id, 0)
	g.Step(1)
	if g.ng.setup.GoneAt == 0 {
		t.Fatal("the wait did not start again")
	}
}

func TestWanderersSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g := goneGame(t)
	waitOver(g)
	sendWanderers(g, 7)
	g.die(livingColonists(g)[0], 0)
	g.Step(3)
	at := g.ng.setup.GoneAt
	h := saveLoad(t, g)
	if h.ng.setup.GoneAt != at || at == 0 || h.ng.setup.WandererGroups != 1 {
		t.Fatalf("gone at %d (was %d), groups %d", h.ng.setup.GoneAt, at, h.ng.setup.WandererGroups)
	}
	if ax := h.World().AX; ax.WanderersLeft != 6 {
		t.Fatalf("%+v", ax)
	}
}

func TestCommitmentKeepsWanderers(t *testing.T) {
	t.Parallel()
	g := goneGame(t)
	g.ng.setup.SaveMode = difficulty.Commitment
	waitOver(g)
	if !g.World().AX.WanderersOK {
		t.Fatal("no option on commitment")
	}
	sendWanderers(g, 5)
	if len(livingColonists(g)) != 1 {
		t.Fatal("no wanderers on commitment")
	}
}

// TestWanderersAfterAbandoningTheLastColony: the first map stays, emptied
// (maps/found.go), and they walk in there.
func TestWanderersAfterAbandoningTheLastColony(t *testing.T) {
	t.Parallel()
	g := visitorGame(t)
	g.Enqueue(command.Command{Kind: command.AbandonMap, Target: 0})
	g.Step(2)
	if len(livingColonists(g)) != 0 || !everyoneGone(g) {
		t.Fatalf("fixture: %d pawns, gone %v", len(livingColonists(g)), everyoneGone(g))
	}
	waitOver(g)
	sendWanderers(g, 77)
	if len(livingColonists(g)) != 1 || everyoneGone(g) {
		t.Fatalf("%d pawns on the first map", len(livingColonists(g)))
	}
}

// TestWanderersComeToTheFirstMapWhateverIsCurrent: the player may be looking
// at another map, and the command is applied on that map's pass.
func TestWanderersComeToTheFirstMapWhateverIsCurrent(t *testing.T) {
	t.Parallel()
	g := goneGame(t)
	waitOver(g)
	first := g.maps[0]
	dead := len(first.pawnIDs)
	other := newMapState(Config{Seed: 9, Width: 30, Height: 30}, 7)
	g.maps = append(g.maps, other)
	g.mapState, g.view = other, other
	sendWanderers(g, 88)
	if g.mapState != other {
		t.Fatal("the command left another map current")
	}
	if len(first.pawnIDs) != dead+1 || len(other.pawnIDs) != 0 {
		t.Fatalf("first %d (was %d), other %d: %+v", len(first.pawnIDs), dead, len(other.pawnIDs), g.messageViews())
	}
}

// messageHas reports a message or letter containing text.
func messageHas(g *Game, text string) bool {
	return slices.ContainsFunc(g.messageViews(), func(m MessageView) bool { return strings.Contains(m.Text, text) })
}

// TestWandererEventRestartsAGoneColony (MAP-22; wiki: Ending: "a wanderer event
// or refugee chased quest can restart the colony"): the storyteller's event
// still fits with nobody left, however long the list of the dead, and whoever
// comes ends the game over.
func TestWandererEventRestartsAGoneColony(t *testing.T) {
	t.Parallel()
	g := goneGame(t)
	for len(g.pawnIDs) < wandererMaxColony { // the dead keep their place on the list
		id, err := g.SpawnNamedPawn(5, 5, "Late")
		if err != nil {
			t.Fatal(err)
		}
		g.die(id, 0)
	}
	g.Step(2)
	if !everyoneGone(g) || g.ng.setup.GoneAt == 0 {
		t.Fatalf("fixture: gone %v at %d", everyoneGone(g), g.ng.setup.GoneAt)
	}
	if !g.FireIncident("wanderer_joins") {
		t.Fatal("the event doesn't fit a colony with nobody")
	}
	g.Step(2)
	ax := g.World().AX
	if len(livingColonists(g)) != 1 || ax.EveryoneGone || g.ng.setup.GoneAt != 0 {
		t.Fatalf("colonists %d, gone %v, at %d", len(livingColonists(g)), ax.EveryoneGone, g.ng.setup.GoneAt)
	}
}

// TestWandererEventKeepsTheColonyCap: with people here, the cap still counts.
func TestWandererEventKeepsTheColonyCap(t *testing.T) {
	t.Parallel()
	g := visitorGame(t)
	for len(g.pawnIDs) < wandererMaxColony {
		if _, err := g.SpawnNamedPawn(5, 5, "More"); err != nil {
			t.Fatal(err)
		}
	}
	if g.wandererFits(uint64(g.clock.Now())) {
		t.Fatal("a full colony takes one more")
	}
}
