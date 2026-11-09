package game

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/incident"
)

// warsOf sets the factions' wars by hand: a and b at war, nobody else.
func warsOf(fs []faction.Faction, a, b int) {
	for i := range fs {
		fs[i].Enemies = nil
	}
	fs[a].Enemies, fs[b].Enemies = []int{fs[b].ID}, []int{fs[a].ID}
}

// TestRivalRaidFromEnemy (EAI-015): when the colony takes a faction's side
// (its military aid arrives), an enemy of that faction hostile to the
// colony queues a raid (the incident queue, EAI-030) for RivalDelay later;
// when it fires the raid is that enemy's, sized then. An enemy defeated by
// then leaves the raid to whoever is hostile; a friend with no hostile
// enemy brings none.
func TestRivalRaidFromEnemy(t *testing.T) {
	t.Parallel()
	g, f := aidGame(t)
	fs := g.ng.factions
	friend := slices.IndexFunc(fs, func(o faction.Faction) bool { return o.ID == f.ID })
	enemy := slices.IndexFunc(fs, func(o faction.Faction) bool { return o.Type.Human() && o.Relation == faction.Hostile })
	if enemy < 0 {
		t.Fatal("no hostile faction")
	}
	warsOf(fs, friend, enemy)
	aidHere(t, g, f)
	now := uint64(g.clock.Now())
	q := g.incidents.Queue
	if len(q) != 1 || q[0].ID != incident.RivalRaid || q[0].Faction != enemy+1 || q[0].At < now || q[0].At > now+RivalDelay {
		t.Fatalf("queue %+v: want one rival raid of faction %d about %d ticks on", q, enemy+1, RivalDelay)
	}
	groups := len(g.raidGroups)
	g.fireQueue(q[0].At)
	if len(g.raidGroups) != groups+1 || len(g.incidents.Queue) != 0 {
		t.Fatalf("%d raid groups (was %d), queue %+v: the rival raid did not fire", len(g.raidGroups), groups, g.incidents.Queue)
	}
	grp := g.raidGroups[len(g.raidGroups)-1]
	if max(grp.Faction, grp.PodFaction) != enemy+1 || grp.Size == 0 { // a drop-pod raid holds it in PodFaction until it lands
		t.Fatalf("raid group %+v, want faction %d (%s)", grp, enemy+1, fs[enemy].Name)
	}
	if msg := g.messages[len(g.messages)-1].Text; !strings.Contains(msg, fs[enemy].Name) {
		t.Fatalf("arrival message %q does not name %s", msg, fs[enemy].Name)
	}
	// Defeated before it comes: rivalRaid declines and the queue falls back.
	g.QueueRivalRaid(now+1, enemy, 300)
	fs[enemy].Defeated = true
	if rivalRaid(g, enemy, 300, now) {
		t.Fatal("a defeated faction still raids")
	}
	g.fireQueue(now + 1)
	if len(g.incidents.Queue) != 0 {
		t.Fatal("the fallen rival's raid stayed queued")
	}
	if queueRival(g, friend, now) || len(g.incidents.Queue) != 0 {
		t.Fatal("a friend with no standing hostile enemy brought a rival raid")
	}
	// The same state and roll queue and send the same raid.
	a, _ := aidGame(t)
	b, _ := aidGame(t)
	for _, x := range []*Game{a, b} {
		warsOf(x.ng.factions, friend, enemy)
		queueRival(x, friend, uint64(x.clock.Now()))
		x.fireQueue(uint64(x.clock.Now()) + RivalDelay)
	}
	if a.StateHash() != b.StateHash() {
		t.Fatal("two equal games sent different rival raids")
	}
}

// TestEnemyRaidBeatenThanks (EAI-015, in the game): the colony drives off
// a raid of faction X; X's enemies thank it once for that raid, and not
// again for another within the cooldown.
func TestEnemyRaidBeatenThanks(t *testing.T) {
	t.Parallel()
	g := buildSetup(t, setupFor(t, "broken_hull", nil))
	stepUntilLanded(t, g)
	fs := g.ng.factions
	x := slices.IndexFunc(fs, func(f faction.Faction) bool { return f.Type == faction.Clans })
	y := slices.IndexFunc(fs, func(f faction.Faction) bool { return f.Type == faction.Settlers })
	hostileOnly(fs, x)
	warsOf(fs, x, y)
	before := fs[y].Goodwill
	beat := func() {
		g.storyRaid(20, 200)
		for _, id := range slices.Clone(g.raiderIDs) {
			g.removeRaider(id)
		}
		g.checkRaidsOver(uint64(g.clock.Now()))
		g.checkRaidsOver(uint64(g.clock.Now())) // the same raid is over once
	}
	beat()
	if fs[y].Goodwill != before+faction.EnemyBonus || !strings.Contains(fs[y].Events[len(fs[y].Events)-1], fs[x].Name) {
		t.Fatalf("goodwill %d (was %d), events %v: want +%d naming %s", fs[y].Goodwill, before, fs[y].Events, faction.EnemyBonus, fs[x].Name)
	}
	beat() // a second raid the same day: the cooldown holds
	if fs[y].Goodwill != before+faction.EnemyBonus {
		t.Fatalf("goodwill %d: thanked twice within the cooldown", fs[y].Goodwill)
	}
	var v FactionView
	for _, r := range g.Factions() {
		if r.ID == fs[y].ID {
			v = r
		}
	}
	if !slices.Equal(v.Enemies, []string{fs[x].Name}) {
		t.Fatalf("the Factions window lists enemies %q, want %q", v.Enemies, fs[x].Name)
	}
}

// TestNewWorldHasWars: a new colony's factions come with their wars.
func TestNewWorldHasWars(t *testing.T) {
	t.Parallel()
	a := buildSetup(t, setupFor(t, "broken_hull", nil))
	wars := 0
	for _, f := range a.ng.factions {
		wars += len(f.Enemies)
	}
	if wars == 0 {
		t.Fatal("no faction is at war with another")
	}
}
