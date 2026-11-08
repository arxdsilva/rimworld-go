package kinds

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
)

// campMap is the site map id the tests open.
const campMap = 7

// offerCamp offers one hostile camp on e (fails the test when refused).
func offerCamp(t *testing.T, e *env) (*quest.Quests, qd.ID) {
	t.Helper()
	q := newRoot(e, Camp{})
	id, ok := q.Offer(qd.Camp)
	if !ok {
		t.Fatal("camp not offered")
	}
	return q, id
}

// acceptedCamp offers a camp and accepts its first items option (or its
// first option when none pays items): the option index.
func acceptedCamp(t *testing.T, e *env) (*quest.Quests, qd.ID, int) {
	t.Helper()
	q, id := offerCamp(t, e)
	choice := 0
	for i, r := range get(t, q, id).Rewards {
		if len(r.Lines) > 0 {
			choice = i
			break
		}
	}
	if err := q.Apply(quest.Accept(id, choice)); err != nil {
		t.Fatal(err)
	}
	return q, id, choice
}

// campCensus delivers an hourly census of standing hostiles on campMap.
func campCensus(q *quest.Quests, id qd.ID, standing int) {
	q.Signal(quest.Signal{Kind: quest.SigSiteCensus, Quest: id, Map: campMap, Count: standing})
}

func TestCampOfferedByAFriendAgainstAnEnemy(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerCamp(t, e)
	n := get(t, q, id)
	if n.State != qd.Offered || n.AcceptBy != e.clock.now+6*day || n.Deadline != 0 {
		t.Fatalf("quest = %+v, want an offer with 6 days to accept", n)
	}
	if n.Giver != 1 || (n.Target != 2 && n.Target != 3) {
		t.Fatalf("giver %d, target %d: want faction 0 asking against a hostile one", n.Giver, n.Target)
	}
	if len(n.Parts) != 2 || n.Parts[campSite].Kind != qd.PartSite || n.Parts[campSite].Tile != 40 ||
		n.Parts[campGuard].Kind != qd.PartThreat || n.Parts[campGuard].Points <= 0 || n.Parts[campGuard].At != 0 {
		t.Fatalf("parts = %+v", n.Parts)
	}
	if len(n.Rewards) == 0 {
		t.Fatal("no reward options")
	}
	for _, r := range n.Rewards {
		if r.Goodwill != 0 && !r.SiteLoot {
			t.Fatalf("goodwill option %+v without the camp's loot (QST-082)", r)
		}
	}
	if len(e.builder.camps) != 0 {
		t.Fatal("the camp was laid before its map opened")
	}
	d := q.View()[0].Detail
	want := []quest.Line{{Key: "quest.camp.giver", Args: []quest.Arg{quest.Faction(1)}},
		{Key: "quest.camp.target", Args: []quest.Arg{quest.Faction(n.Target)}}, {Key: "quest.camp.site", Args: []quest.Arg{quest.Tile(40)}}}
	if !reflect.DeepEqual(d, want) {
		t.Fatalf("detail = %+v, want %+v", d, want)
	}
}

func TestCampNeedsThreatsAGiverATargetAndATile(t *testing.T) {
	t.Parallel()
	for name, edit := range map[string]func(e *env){
		"threats off":   func(e *env) { e.world.threats = false },
		"no giver":      func(e *env) { e.world.factions[0].Relation = faction.Hostile; e.world.factions[0].Settlements = nil },
		"giver unfit":   func(e *env) { e.world.factions[0].CanBeFriendly = false },
		"no target":     func(e *env) { e.world.factions[1].Defeated, e.world.factions[2].Settlements = true, nil },
		"no free tile":  func(e *env) { e.world.near = -1 },
		"giver defeats": func(e *env) { e.world.factions[0].Defeated = true },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv()
			edit(e)
			if _, ok := newRoot(e, Camp{}).Offer(qd.Camp); ok {
				t.Fatal("offered")
			}
			if name != "no free tile" && name != "threats off" && (Camp{}).Build(quest.Ctx{R: e.roles()}, &qd.Quest{}) {
				t.Fatal("Build without a giver or target")
			}
		})
	}
}

func TestCampClearedSucceedsOnceNoFlap(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id, choice := acceptedCamp(t, e)
	n := get(t, q, id)
	if n.State != qd.Active || n.Deadline != e.clock.now+17*day {
		t.Fatalf("accepted: %+v, want the 17-day window from now", n)
	}
	// Before the map opens, a census or a close of another map is nothing.
	campCensus(q, id, 0)
	q.Signal(quest.Signal{Kind: quest.SigMapClosed, Quest: id, Map: campMap})
	q.Signal(quest.Signal{Kind: quest.SigSiteOpened, Quest: id, Tile: 41, Map: campMap})
	if get(t, q, id).State != qd.Active || len(e.builder.camps) != 0 {
		t.Fatal("ended or laid before its own site opened")
	}
	openSite(e, q, id, 40, campMap)
	openSite(e, q, id, 40, campMap+1) // a second open lays nothing more
	n = get(t, q, id)
	spawned := n.Parts[campSite].Count
	if len(e.builder.camps) != 1 || spawned <= 0 || spawned > campDefenders || n.Parts[campSite].Map != campMap ||
		len(e.builder.camps[0].Posts) == 0 || len(e.builder.camps[0].Loot) == 0 {
		t.Fatalf("laid %d camps, %d defenders: %+v", len(e.builder.camps), spawned, n.Parts)
	}
	if d := q.View()[0].Detail; d[len(d)-1].Key != "quest.camp.standing" || d[len(d)-2].Key != "quest.camp.deadline" {
		t.Fatalf("detail = %+v", d)
	}
	half := (spawned + 1) / 2 // at least half standing: not yet
	campCensus(q, id, spawned)
	campCensus(q, id, half)
	q.Signal(quest.Signal{Kind: quest.SigSiteCensus, Quest: id, Map: campMap + 1, Count: 0})
	if n := get(t, q, id); n.State != qd.Active || len(e.gw.adjusted) != 0 {
		t.Fatalf("cleared with %d of %d standing: %+v", half, spawned, n)
	}
	campCensus(q, id, half-1)
	n = get(t, q, id)
	if n.State != qd.Succeeded || !n.Parts[campSite].Done || n.Parts[campGuard].Count != half-1 {
		t.Fatalf("quest = %+v, want succeeded", n)
	}
	opt := n.Rewards[choice]
	wantGW := [][2]int{{0, campGoodwill}}
	if opt.Goodwill != 0 {
		wantGW = append(wantGW, [2]int{0, opt.Goodwill})
	}
	if !reflect.DeepEqual(e.gw.adjusted, wantGW) {
		t.Fatalf("goodwill = %v, want %v", e.gw.adjusted, wantGW)
	}
	if len(opt.Lines) > 0 && (len(e.items.pods) != 1 || !reflect.DeepEqual(e.items.pods[0], opt.Lines)) {
		t.Fatalf("pods = %v, want the chosen %v at home", e.items.pods, opt.Lines)
	}
	// No flap: later censuses, above or below, change nothing.
	for _, s := range []int{spawned, 0, half - 1} {
		campCensus(q, id, s)
	}
	if n := get(t, q, id); n.State != qd.Succeeded || len(e.gw.adjusted) != len(wantGW) || len(e.items.pods) > 1 {
		t.Fatalf("after more censuses: %+v, goodwill %v", n, e.gw.adjusted)
	}
}

func TestCampWithoutDefendersSucceedsOnAnEmptyCensus(t *testing.T) {
	t.Parallel()
	e := newEnv()
	e.builder.refuse = true
	q, id, _ := acceptedCamp(t, e)
	openSite(e, q, id, 40, campMap)
	if n := get(t, q, id); n.Parts[campSite].Count != 0 || n.Parts[campSite].Map != campMap {
		t.Fatalf("parts = %+v", n.Parts)
	}
	campCensus(q, id, 2) // someone else hostile stands: not yet
	if get(t, q, id).State != qd.Active {
		t.Fatal("succeeded with hostiles standing")
	}
	campCensus(q, id, 0)
	if get(t, q, id).State != qd.Succeeded {
		t.Fatal("an empty census did not succeed")
	}
}

func TestCampFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		open   bool
		act    func(t *testing.T, e *env, q *quest.Quests, id qd.ID)
		state  qd.State
		reason qd.Reason
	}{
		{"map closed", true, func(t *testing.T, e *env, q *quest.Quests, id qd.ID) {
			q.Signal(quest.Signal{Kind: quest.SigMapClosed, Quest: id, Map: campMap})
		}, qd.Failed, qd.SiteLost},
		{"site lost", false, func(t *testing.T, e *env, q *quest.Quests, id qd.ID) {
			q.Signal(quest.Signal{Kind: quest.SigSiteLost, Quest: id, Tile: 41})
			q.Signal(quest.Signal{Kind: quest.SigSiteLost, Tile: 40})
		}, qd.Failed, qd.SiteLost},
		{"deadline", true, func(t *testing.T, e *env, q *quest.Quests, id qd.ID) {
			run(e, q, get(t, q, id).Deadline+hour)
		}, qd.Failed, qd.Timeout},
		{"giver hostile", false, func(t *testing.T, e *env, q *quest.Quests, id qd.ID) {
			run(e, q, e.clock.now+hour)
			e.gw.rel[0] = faction.Hostile
			run(e, q, e.clock.now+hour)
		}, qd.Failed, qd.GiverHostile},
		{"target at peace", false, func(t *testing.T, e *env, q *quest.Quests, id qd.ID) {
			run(e, q, e.clock.now+hour)
			e.gw.rel[get(t, q, id).Target-1] = faction.Neutral
			run(e, q, e.clock.now+hour)
		}, qd.Failed, qd.TargetNotHostile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv()
			q, id, _ := acceptedCamp(t, e)
			if tc.open {
				openSite(e, q, id, 40, campMap)
			}
			tc.act(t, e, q, id)
			n := get(t, q, id)
			if n.State != tc.state || n.Reason != tc.reason || len(e.gw.adjusted) != 0 || len(e.items.pods) != 0 {
				t.Fatalf("quest = %+v, goodwill %v, pods %v; want %v (%v), nothing paid", n, e.gw.adjusted, e.items.pods, tc.state, tc.reason)
			}
		})
	}
}

func TestCampOfferExpires(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerCamp(t, e)
	run(e, q, e.clock.now+6*day+hour)
	if n := get(t, q, id); n.State != qd.Expired {
		t.Fatalf("quest = %+v, want expired", n)
	}
	openSite(e, q, id, 40, campMap)
	if len(e.builder.camps) != 0 {
		t.Fatal("an expired camp was laid")
	}
}

func TestCampDeterministic(t *testing.T) {
	t.Parallel()
	play := func() (qd.Quest, *env) {
		e := newEnv()
		q, id, _ := acceptedCamp(t, e)
		openSite(e, q, id, 40, campMap)
		return get(t, q, id), e
	}
	a, ea := play()
	b, eb := play()
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(ea.builder.camps, eb.builder.camps) || !reflect.DeepEqual(ea.builder.arms, eb.builder.arms) {
		t.Fatalf("same seed, different camps:\n%+v\n%+v", a, b)
	}
}

func TestCampWithoutItsParts(t *testing.T) {
	t.Parallel()
	e := newEnv()
	c := quest.Ctx{R: e.roles()}
	var k Camp
	q := &qd.Quest{State: qd.Active, Parts: []qd.Part{{Kind: qd.PartSite, Tile: 40}}}
	k.Accepted(c, q)
	k.Ended(c, q)
	if v := k.Signal(c, q, quest.Signal{Kind: quest.SigSiteLost, Tile: 40}); v != quest.VerdictNone {
		t.Fatalf("signal verdict %v", v)
	}
	if v := k.PartDue(c, q, 0); v != quest.VerdictNone || k.Detail(*q) != nil {
		t.Fatal("acted on a quest without a camp's parts")
	}
}
