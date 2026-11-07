package kinds

import (
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/mapgen"
)

func offerRescue(t *testing.T, e *env) (*quest.Quests, qd.Quest) {
	t.Helper()
	q := newRoot(e, Rescue{})
	id, ok := q.Offer(qd.Rescue)
	if !ok {
		t.Fatal("rescue not offered")
	}
	return q, get(t, q, id)
}

// openedRescue is an offered rescue whose site map (id 5) is open.
func openedRescue(t *testing.T) (*env, *quest.Quests, qd.Quest) {
	t.Helper()
	e := newEnv()
	q, n := offerRescue(t, e)
	openSite(e, q, n.ID, n.Parts[0].Tile, 5)
	return e, q, get(t, q, n.ID)
}

func TestRescueOfferShape(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, n := offerRescue(t, e)
	if n.State != qd.Active || n.Deadline != e.clock.now+23*day || n.Giver != 0 || n.Target < 2 {
		t.Fatalf("state %v deadline %d giver %d target %d", n.State, n.Deadline, n.Giver, n.Target)
	}
	if len(n.Parts) != 2 || n.Parts[0].Kind != qd.PartSite || n.Parts[0].Tile != 40 || n.Parts[1].Kind != qd.PartPawn || n.Parts[1].Pawn != 0 {
		t.Fatalf("parts %+v", n.Parts)
	}
	if len(n.Rewards) != 1 || n.Rewards[0].Pawn != n.Parts[1].Seed {
		t.Fatalf("the pawn is the one option: %+v", n.Rewards)
	}
	if len(e.sites.placed) != 1 || e.sites.placed[0].Tile != 40 || e.sites.placed[0].Quest != n.ID || e.sites.placed[0].Faction != n.Target {
		t.Fatalf("site %+v", e.sites.placed)
	}
	if len(e.builder.camps) != 0 || len(e.builder.pawns) != 0 {
		t.Fatal("nothing is laid before the site opens")
	}
	d := q.View()[0].Detail
	if len(d) != 4 || d[0].Key != "quest.rescue.site" || d[3].Key != "quest.rescue.calling" {
		t.Fatalf("detail %+v", d)
	}
}

func TestRescueSiteOpenedLaysCampGuardsAndPrisoner(t *testing.T) {
	t.Parallel()
	e, q, n := openedRescue(t)
	if len(e.builder.camps) != 1 || len(e.builder.pawns) != 1 {
		t.Fatalf("camps %d pawns %d", len(e.builder.camps), len(e.builder.pawns))
	}
	tech := faction.Neolithic
	if n.Parts[0].Layout == rescueIndustrial {
		tech = faction.Industrial
	}
	camp, cell := mapgen.CellCamp(mapgen.CompoundConfig{W: 150, H: 150, Tech: tech, Seed: n.Parts[0].Seed})
	if !reflect.DeepEqual(e.builder.camps[0], camp) {
		t.Fatal("the compound is the seeded one")
	}
	if g := len(e.builder.arms[0]); g < 2 || g > len(camp.Posts) || n.Parts[0].Count != g {
		t.Fatalf("guards %d of %d posts, recorded %d", g, len(camp.Posts), n.Parts[0].Count)
	}
	p := e.builder.pawns[0]
	if !p.Held || p.Faction != 0 || p.Seed != n.Parts[1].Seed || p.X != cell.Inside.X || p.Y != cell.Inside.Y || !p.MapCurrent {
		t.Fatalf("prisoner %+v, cell inside %v", p, cell.Inside)
	}
	if n.Parts[1].Pawn != uint64(p.ID) || n.Parts[0].Map != 5 || n.State != qd.Active {
		t.Fatalf("parts %+v", n.Parts)
	}
	d := q.View()[0].Detail
	if d[3].Key != "quest.rescue.held" {
		t.Fatalf("detail %+v", d)
	}
	// The site opening again (the map is current once more) lays nothing new,
	// and neither does a census.
	openSite(e, q, n.ID, n.Parts[0].Tile, 5)
	q.Signal(quest.Signal{Kind: quest.SigSiteCensus, Quest: n.ID, Map: 5, Count: 0})
	if len(e.builder.camps) != 1 || len(e.builder.pawns) != 1 || get(t, q, n.ID).State != qd.Active {
		t.Fatal("a second open or a census changed the site")
	}
}

func TestRescueFreedSucceedsAndPawnJoins(t *testing.T) {
	t.Parallel()
	e, q, n := openedRescue(t)
	id := entity.EntityID(n.Parts[1].Pawn)
	q.Signal(quest.Signal{Kind: quest.SigPawnFreed, Pawn: id + 1}) // someone else
	if get(t, q, n.ID).State != qd.Active {
		t.Fatal("another pawn's freeing ended the quest")
	}
	q.Signal(quest.Signal{Kind: quest.SigPawnFreed, Pawn: id})
	g := get(t, q, n.ID)
	if g.State != qd.Succeeded || !g.Paid || !g.Parts[1].Done {
		t.Fatalf("not succeeded: %+v", g)
	}
	if !slices.Equal(e.pawns.joined, []entity.EntityID{id}) || len(e.pawns.walked) != 0 {
		t.Fatalf("joined %v walked-in %v: the freed pawn joins, no second one walks in", e.pawns.joined, e.pawns.walked)
	}
	if !slices.Contains(e.sites.removed, 40) {
		t.Fatal("the camp's claim stays")
	}
	if d := q.View()[0].Detail; d[3].Key != "quest.rescue.freed" {
		t.Fatalf("detail %+v", d)
	}
	// Ended: nothing else moves it.
	q.Signal(quest.Signal{Kind: quest.SigPawnDied, Pawn: id})
	if get(t, q, n.ID).State != qd.Succeeded {
		t.Fatal("a later death changed a finished quest")
	}
}

func TestRescueFailPaths(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		do   func(e *env, q *quest.Quests, n qd.Quest)
		want qd.Reason
	}{
		{"deadline", func(e *env, q *quest.Quests, n qd.Quest) { run(e, q, n.Deadline+hour) }, qd.Timeout},
		{"prisoner died", func(e *env, q *quest.Quests, n qd.Quest) {
			q.Signal(quest.Signal{Kind: quest.SigPawnDied, Pawn: entity.EntityID(n.Parts[1].Pawn)})
		}, qd.PawnLost},
		{"freed dead", func(e *env, q *quest.Quests, n qd.Quest) {
			e.pawns.dead = append(e.pawns.dead, entity.EntityID(n.Parts[1].Pawn))
			q.Signal(quest.Signal{Kind: quest.SigPawnFreed, Pawn: entity.EntityID(n.Parts[1].Pawn)})
		}, qd.PawnLost},
		{"map closed", func(e *env, q *quest.Quests, n qd.Quest) {
			q.Signal(quest.Signal{Kind: quest.SigMapClosed, Quest: n.ID, Map: 9}) // another map: nothing
			if get(t, q, n.ID).State != qd.Active {
				t.Fatal("another map's close ended it")
			}
			q.Signal(quest.Signal{Kind: quest.SigMapClosed, Quest: n.ID, Map: 5})
		}, qd.SiteLost},
		{"site lost", func(e *env, q *quest.Quests, n qd.Quest) {
			q.Signal(quest.Signal{Kind: quest.SigSiteLost, Tile: n.Parts[0].Tile})
		}, qd.SiteLost},
		{"captor made peace", func(e *env, q *quest.Quests, n qd.Quest) {
			e.gw.rel[n.Target-1] = faction.Neutral
			run(e, q, e.clock.now+2*hour)
		}, qd.TargetNotHostile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, q, n := openedRescue(t)
			tc.do(e, q, n)
			g := get(t, q, n.ID)
			if g.State != qd.Failed || g.Reason != tc.want {
				t.Fatalf("state %v reason %v, want %v", g.State, g.Reason, tc.want)
			}
			if !slices.Contains(e.sites.removed, 40) || len(e.pawns.joined) != 0 {
				t.Fatalf("removed %v joined %v", e.sites.removed, e.pawns.joined)
			}
		})
	}
}

func TestRescueFailsWhenNothingCanBeLaid(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, n := offerRescue(t, e)
	e.builder.refuse = true
	openSite(e, q, n.ID, n.Parts[0].Tile, 5)
	if g := get(t, q, n.ID); g.State != qd.Failed || g.Reason != qd.SiteLost {
		t.Fatalf("state %v reason %v", g.State, g.Reason)
	}
}

func TestRescueMapClosedBeforeOpenIsIgnored(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, n := offerRescue(t, e)
	q.Signal(quest.Signal{Kind: quest.SigMapClosed, Quest: n.ID, Map: 0})
	q.Signal(quest.Signal{Kind: quest.SigSiteOpened, Quest: n.ID, Tile: n.Parts[0].Tile + 1, Map: 5}) // not its tile
	if g := get(t, q, n.ID); g.State != qd.Active || len(e.builder.camps) != 0 {
		t.Fatalf("state %v camps %d", g.State, len(e.builder.camps))
	}
}

func TestRescueCannotBeOffered(t *testing.T) {
	t.Parallel()
	e := newEnv()
	e.world.threats = false
	if _, ok := newRoot(e, Rescue{}).Offer(qd.Rescue); ok {
		t.Fatal("offered with quest threats off")
	}
	e = newEnv()
	e.world.factions[1].Defeated, e.world.factions[2].Defeated = true, true
	if _, ok := newRoot(e, Rescue{}).Offer(qd.Rescue); ok {
		t.Fatal("offered with no enemy")
	}
	e = newEnv()
	e.world.near = -1
	if _, ok := newRoot(e, Rescue{}).Offer(qd.Rescue); ok {
		t.Fatal("offered with no free tile")
	}
	e = newEnv()
	e.world.factions = nil
	if (Rescue{}).Build(quest.Ctx{R: e.roles()}, &qd.Quest{}) {
		t.Fatal("built with no enemy")
	}
	k := Rescue{}
	if k.PartDue(quest.Ctx{}, &qd.Quest{}, 0) != quest.VerdictNone || k.Detail(qd.Quest{}) != nil {
		t.Fatal("part due or empty detail")
	}
	k.Accepted(quest.Ctx{R: newEnv().roles()}, &qd.Quest{})
	k.Ended(quest.Ctx{R: newEnv().roles()}, &qd.Quest{})
	if k.Signal(quest.Ctx{}, &qd.Quest{}, quest.Signal{Kind: quest.SigPawnDied}) != quest.VerdictNone {
		t.Fatal("signal on an empty quest")
	}
}

func TestRescueDeterministic(t *testing.T) {
	t.Parallel()
	_, _, a := openedRescue(t)
	_, _, b := openedRescue(t)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("same seed, different quests:\n%+v\n%+v", a, b)
	}
}
