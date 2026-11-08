package kinds

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/skill"
	qd "rimworld-go/internal/domain/quest"
)

// offerTalks offers one peace talks on e (fails the test when refused).
func offerTalks(t *testing.T, e *env) (*quest.Quests, qd.ID) {
	t.Helper()
	q := newRoot(e, Talks{})
	id, ok := q.Offer(qd.Talks)
	if !ok {
		t.Fatal("talks not offered")
	}
	return q, id
}

// hasKey reports whether lines hold key.
func hasKey(lines []quest.Line, key string) bool {
	for _, l := range lines {
		if l.Key == key {
			return true
		}
	}
	return false
}

func TestTalksOfferIsActiveAtASiteOfTheHost(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerTalks(t, e)
	n := get(t, q, id)
	if n.State != qd.Active || n.Giver != 2 || n.Target != 0 || len(n.Rewards) != 0 || n.Deadline != 0 {
		t.Fatalf("quest = %+v, want active, giver 2 (faction 1), no rewards, no deadline", n)
	}
	if len(n.Parts) != 1 || n.Parts[0].Kind != qd.PartTalk || n.Parts[0].Tile != 40 {
		t.Fatalf("parts = %+v, want one talk part on tile 40", n.Parts)
	}
	if at := n.Parts[0].At; at < 30*day+12*day || at > 30*day+28*day {
		t.Fatalf("lifetime ends at %d, want 12..28 days after %d", at, 30*day)
	}
	want := quest.Site{Quest: id, Kind: qd.Talks, Tile: 40, Seed: n.Parts[0].Seed, Faction: 2}
	if len(e.sites.placed) != 1 || e.sites.placed[0] != want {
		t.Fatalf("placed = %+v, want %+v", e.sites.placed, want)
	}
	if len(e.letters.list) != 1 || e.letters.list[0].Kind != quest.LetterOffer || e.letters.list[0].Tile != 40 {
		t.Fatalf("letters = %+v, want one offer letter at tile 40", e.letters.list)
	}
	d := q.View()[0].Detail
	if len(d) != 3 || d[0].Key != "quest.talks.host" || d[0].Args[0] != quest.Faction(2) ||
		d[1].Key != "quest.talks.site" || d[1].Args[0] != quest.Tile(40) || d[2].Key != "quest.talks.caravan" {
		t.Fatalf("detail = %+v", d)
	}
}

func TestTalksNeedAHostWillingToTalk(t *testing.T) {
	t.Parallel()
	for name, edit := range map[string]func(f *quest.FactionView){
		"defeated":       func(f *quest.FactionView) { f.Defeated = true },
		"besieged":       func(f *quest.FactionView) { f.Besieged = true },
		"never friendly": func(f *quest.FactionView) { f.CanBeFriendly = false },
		"not hostile":    func(f *quest.FactionView) { f.Relation = faction.Neutral },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv()
			edit(&e.world.factions[1])
			q := newRoot(e, Talks{})
			if _, ok := q.Offer(qd.Talks); ok {
				t.Fatal("offered without a host (the pirates never host)")
			}
			if (Talks{}).Build(quest.Ctx{R: e.roles()}, &qd.Quest{}) {
				t.Fatal("Build without a host")
			}
		})
	}
}

func TestTalksNeedAFreeTile(t *testing.T) {
	t.Parallel()
	e := newEnv()
	e.world.near = -1
	if _, ok := newRoot(e, Talks{}).Offer(qd.Talks); ok || len(e.sites.placed) != 0 {
		t.Fatal("offered without a free tile")
	}
}

func TestTalksOutcomes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		roll int
		want qd.TalkResult
		key  string
	}{
		{0, qd.TalkTriumph, "quest.talks.triumph"},
		{300, qd.TalkSuccess, "quest.talks.success"},
		{700, qd.TalkFlounder, "quest.talks.flounder"},
		{900, qd.TalkBackfire, "quest.talks.backfire"},
		{999, qd.TalkDisaster, "quest.talks.disaster"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()
			e := newEnv()
			q, id := offerTalks(t, e)
			out := qd.Negotiate(e.pawns.social, tc.roll, 500)
			if out.Result != tc.want {
				t.Fatalf("roll %d gives %d, want %d", tc.roll, out.Result, tc.want)
			}
			e.rolls.script = []int{tc.roll, 500}
			q.Signal(quest.Signal{Kind: quest.SigCaravanArrived, Tile: 40, Caravan: 3})
			n := get(t, q, id)
			if n.State != qd.Succeeded || !n.Parts[0].Done || n.Parts[0].Count != int(tc.want)+1 {
				t.Fatalf("quest = %+v, want succeeded with result %d recorded", n, tc.want)
			}
			if e.pawns.xp[skill.Social] != qd.TalkXP {
				t.Fatalf("social xp = %d, want %d", e.pawns.xp[skill.Social], qd.TalkXP)
			}
			wantGW := [][2]int{{1, out.Goodwill}}
			if out.Goodwill == 0 {
				wantGW = nil
			}
			if !reflect.DeepEqual(e.gw.adjusted, wantGW) {
				t.Fatalf("goodwill = %v, want %v", e.gw.adjusted, wantGW)
			}
			if got := qd.Value(e.items.into[3]); got != out.Gift {
				t.Fatalf("gift worth %d (%v), want %d", got, e.items.into[3], out.Gift)
			}
			for _, l := range e.items.into[3] {
				if l.Qty <= 0 || l.Qty > l.Kind.StackLimit() {
					t.Fatalf("gift line %+v outside a stack", l)
				}
			}
			ambushed := len(e.threats.calls) == 1 && e.threats.calls[0].Kind == "ambush" &&
				e.threats.calls[0].Caravan == 3 && e.threats.calls[0].Faction == 1 && e.threats.calls[0].Points > 0
			if ambushed != out.Ambush || (!out.Ambush && len(e.threats.calls) != 0) {
				t.Fatalf("threats = %+v, want ambush %v", e.threats.calls, out.Ambush)
			}
			if !reflect.DeepEqual(e.sites.removed, []int{40}) || len(e.pawns.walked) != 0 || len(e.items.pods) != 0 {
				t.Fatalf("removed %v, walked %v, pods %v", e.sites.removed, e.pawns.walked, e.items.pods)
			}
			if l := e.letters.list[len(e.letters.list)-1]; l.Kind != quest.LetterSucceeded {
				t.Fatalf("last letter = %+v, want success", l)
			}
			d := q.View()[0].Detail
			if !hasKey(d, tc.key) || hasKey(d, "quest.talks.caravan") || hasKey(d, "quest.talks.gift") != (out.Gift > 0) {
				t.Fatalf("detail = %+v", d)
			}
			// The talks happen once.
			q.Signal(quest.Signal{Kind: quest.SigCaravanArrived, Tile: 40, Caravan: 3})
			if e.pawns.xp[skill.Social] != qd.TalkXP {
				t.Fatal("talks held twice")
			}
		})
	}
}

func TestTalksOnNegotiateOrder(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerTalks(t, e)
	// Moving, then elsewhere: nothing.
	if err := q.Apply(quest.Negotiate(id, 3)); err != nil {
		t.Fatal(err)
	}
	e.items.at[3] = 41
	if err := q.Apply(quest.Negotiate(id, 3)); err != nil {
		t.Fatal(err)
	}
	if n := get(t, q, id); n.State != qd.Active || e.pawns.xp[skill.Social] != 0 {
		t.Fatalf("talks held away from the tile: %+v", n)
	}
	e.items.at[3] = 40
	if err := q.Apply(quest.Negotiate(id, 3)); err != nil {
		t.Fatal(err)
	}
	if n := get(t, q, id); n.State != qd.Succeeded || e.pawns.xp[skill.Social] != qd.TalkXP {
		t.Fatalf("quest = %+v, want succeeded on the tile", n)
	}
}

func TestTalksNeedSomeoneToTalk(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerTalks(t, e)
	e.pawns.social = -1
	e.items.at[3] = 40
	q.Signal(quest.Signal{Kind: quest.SigCaravanArrived, Tile: 40, Caravan: 3})
	q.Signal(quest.Signal{Kind: quest.SigCaravanArrived, Tile: 40, Quest: id})
	q.Signal(quest.Signal{Kind: quest.SigCaravanArrived, Tile: 41, Caravan: 3, Quest: id})
	if err := q.Apply(quest.Negotiate(id, 3)); err != nil {
		t.Fatal(err)
	}
	if n := get(t, q, id); n.State != qd.Active || len(e.gw.adjusted) != 0 || len(e.pawns.xp) != 0 {
		t.Fatalf("talks without a talker: %+v", n)
	}
	e.pawns.social = 0
	if err := q.Apply(quest.Negotiate(id, 3)); err != nil {
		t.Fatal(err)
	}
	if n := get(t, q, id); n.State != qd.Succeeded {
		t.Fatalf("quest = %+v, want the later talks held", n)
	}
}

func TestTalksTimeOut(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerTalks(t, e)
	at := get(t, q, id).Parts[0].At
	run(e, q, at-hour)
	if get(t, q, id).State != qd.Active {
		t.Fatal("ended before the lifetime")
	}
	run(e, q, at+hour)
	n := get(t, q, id)
	if n.State != qd.Failed || n.Reason != qd.Timeout {
		t.Fatalf("quest = %+v, want failed by timeout", n)
	}
	if !reflect.DeepEqual(e.sites.removed, []int{40}) || len(e.pawns.xp) != 0 || len(e.gw.adjusted) != 0 {
		t.Fatalf("removed %v, xp %v, goodwill %v", e.sites.removed, e.pawns.xp, e.gw.adjusted)
	}
	if l := e.letters.list[len(e.letters.list)-1]; l.Kind != quest.LetterFailed || l.Reason != qd.Timeout {
		t.Fatalf("last letter = %+v", l)
	}
}

func TestTalksSiteRefusedFailsSoon(t *testing.T) {
	t.Parallel()
	e := newEnv()
	e.sites.refuse = true
	q, id := offerTalks(t, e)
	if d := q.View()[0].Detail; !hasKey(d, "quest.talks.nosite") || hasKey(d, "quest.talks.caravan") {
		t.Fatalf("detail = %+v", d)
	}
	q.Signal(quest.Signal{Kind: quest.SigCaravanArrived, Tile: 40, Caravan: 3})
	run(e, q, e.clock.now+hour)
	n := get(t, q, id)
	if n.State != qd.Failed || n.Reason != qd.SiteLost || len(e.sites.removed) != 0 || len(e.pawns.xp) != 0 {
		t.Fatalf("quest = %+v, removed %v, want failed (site lost) with nothing removed", n, e.sites.removed)
	}
}

func TestTalksFailWhenTheSiteOrHostIsLost(t *testing.T) {
	t.Parallel()
	for name, s := range map[string]quest.Signal{
		"site lost":     {Kind: quest.SigSiteLost, Tile: 40},
		"host defeated": {Kind: quest.SigFactionDefeated, Faction: 2},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv()
			q, id := offerTalks(t, e)
			// Another tile, another faction, an unrelated signal: nothing.
			q.Signal(quest.Signal{Kind: quest.SigSiteLost, Tile: 41, Quest: id})
			q.Signal(quest.Signal{Kind: quest.SigFactionDefeated, Faction: 3, Quest: id})
			q.Signal(quest.Signal{Kind: quest.SigPawnDied, Quest: id})
			if get(t, q, id).State != qd.Active {
				t.Fatal("ended by an unrelated signal")
			}
			q.Signal(s)
			n := get(t, q, id)
			if n.State != qd.Failed || n.Reason != qd.SiteLost || !reflect.DeepEqual(e.sites.removed, []int{40}) {
				t.Fatalf("quest = %+v, removed %v", n, e.sites.removed)
			}
		})
	}
}

func TestTalksSurviveRelationChanges(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerTalks(t, e)
	run(e, q, e.clock.now+hour)
	e.gw.rel[1] = faction.Neutral
	run(e, q, e.clock.now+hour)
	if n := get(t, q, id); n.State != qd.Active {
		t.Fatalf("quest = %+v, want the talks still open", n)
	}
}

func TestTalksDeterministic(t *testing.T) {
	t.Parallel()
	play := func() (qd.Quest, *env) {
		e := newEnv()
		q, id := offerTalks(t, e)
		q.Signal(quest.Signal{Kind: quest.SigCaravanArrived, Tile: 40, Caravan: 3})
		return get(t, q, id), e
	}
	a, ea := play()
	b, eb := play()
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(ea.gw.adjusted, eb.gw.adjusted) ||
		!reflect.DeepEqual(ea.items.into, eb.items.into) || !reflect.DeepEqual(ea.threats.calls, eb.threats.calls) {
		t.Fatalf("same seed, different quests:\n%+v\n%+v", a, b)
	}
}

func TestTalksWithoutATalkPart(t *testing.T) {
	t.Parallel()
	e := newEnv()
	c := quest.Ctx{R: e.roles()}
	var k Talks
	q := &qd.Quest{Giver: 2, Parts: []qd.Part{{Kind: qd.PartThreat, At: 5}}}
	k.Accepted(c, q)
	k.Ended(c, q)
	if v := k.Signal(c, q, quest.Signal{Kind: quest.SigCaravanArrived, Tile: -1, Caravan: 3}); v != quest.VerdictNone {
		t.Fatalf("signal verdict %v", v)
	}
	if v := k.PartDue(c, q, 0); v != quest.VerdictNone {
		t.Fatalf("part due verdict %v", v)
	}
	if k.Detail(*q) != nil || len(e.sites.placed)+len(e.sites.removed) != 0 {
		t.Fatal("acted on a quest without a talk part")
	}
	p := &qd.Part{Kind: qd.PartTalk, Tile: 40}
	if v := talk(c, q, p, 0); v != quest.VerdictNone || p.Done {
		t.Fatal("talks held without a caravan")
	}
	done := &qd.Quest{Parts: []qd.Part{{Kind: qd.PartTalk, Done: true, At: 5}}}
	if v := k.PartDue(c, done, 0); v != quest.VerdictNone {
		t.Fatalf("held talks timed out: %v", v)
	}
	ended := qd.Quest{State: qd.Failed, Giver: 2, Parts: []qd.Part{{Kind: qd.PartTalk, Tile: 40}}}
	if d := k.Detail(ended); len(d) != 2 {
		t.Fatalf("ended detail = %+v, want host and site only", d)
	}
}
