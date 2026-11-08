package maps

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"testing"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// siteHost is a Host for the quest site rules: the methods they call, over
// a nil Host (anything else panics). Hostiles is the census's standing
// hostiles; each Hunt and Ambush adds one.
type siteHost struct {
	Host
	seed     uint64
	approach bool
	foe      int
	ours     int
	hostiles int
	busy     bool
	facs     []faction.Faction
	hunts    []uint64
	ambushes []int
	letters  []string
}

func (h *siteHost) Switch(uint32) bool                    { return true }
func (h *siteHost) Seed() uint64                          { return h.seed }
func (h *siteHost) TileName(t int) string                 { return fmt.Sprint("tile ", t) }
func (h *siteHost) Factions() []faction.Faction           { return h.facs }
func (h *siteHost) RoadRoll(n int) int                    { return 0 }
func (h *siteHost) Census([]int) Census                   { return Census{Ours: h.ours, Hostiles: h.hostiles} }
func (h *siteHost) SiteRules(uint32) (bool, int)          { return h.approach, h.foe }
func (h *siteHost) SiteCensus(_, _ uint32, _ Census) bool { return h.busy }
func (h *siteHost) Hunt(seed uint64, _ int)               { h.hunts = append(h.hunts, seed); h.hostiles++ }
func (h *siteHost) Ambush(_ uint64, fac int)              { h.ambushes = append(h.ambushes, fac); h.hostiles++ }
func (h *siteHost) Letter(_ bool, title string, _ entity.EntityID, _ string, _ ...any) {
	h.letters = append(h.letters, title)
}

// hostileHost is a siteHost with one hostile human faction (ref 1).
func hostileHost(seed uint64) *siteHost {
	return &siteHost{seed: seed, facs: []faction.Faction{{Type: faction.Clans, Relation: faction.Hostile}}}
}

// questMap is a fleet over h with one quest site map open on tile: its id.
func questMap(h *siteHost, tile int) (Lifecycle, uint32) {
	a := &Atlas{}
	id := a.Add(Site, tile, 0)
	a.Meta(id).Quest = 9
	return Of(&[]Caravan{}, a, h).Life(), id
}

var walker = &Caravan{C: caravan.Caravan{ID: 50, Members: []uint64{7}}}

// TestQuestSiteApproachRollsAThirdEach: a quest site whose kind approaches
// rolls the mineral site's threat, a third each over many sites, the same
// way twice; one that does not approach sends nothing, and either way the
// site files its foe (QST-051, 056).
func TestQuestSiteApproachRollsAThirdEach(t *testing.T) {
	t.Parallel()
	var n [3]int
	for tile := range 3000 {
		var got [2]string
		for run := range 2 {
			h := hostileHost(0x5eed)
			h.approach, h.foe = true, 1
			l, id := questMap(h, tile)
			l.questSite(walker, id, 9)
			got[run] = fmt.Sprint(h.hunts, h.ambushes, h.letters)
			want := OreThreat(0x5eed, tile, id)
			if run == 0 {
				n[want]++
			}
			switch {
			case want == OreNone && (len(h.hunts)+len(h.ambushes) > 0 || len(h.letters) > 0),
				want == OreAnimals && (len(h.hunts) != 1 || len(h.ambushes) != 0 || len(h.letters) != 1),
				want == OreAmbush && (len(h.ambushes) != 1 || h.ambushes[0] != 1 || len(h.letters) != 1):
				t.Fatalf("tile %d rolled %d: hunts %v ambushes %v letters %v", tile, want, h.hunts, h.ambushes, h.letters)
			}
			if m := l.f.a.Meta(id); m.Foe != 1 {
				t.Fatalf("the site filed foe %d, want 1", m.Foe)
			}
		}
		if got[0] != got[1] {
			t.Fatalf("tile %d: %s, then %s", tile, got[0], got[1])
		}
	}
	for k, c := range n {
		if c < 850 || c > 1150 {
			t.Fatalf("threat %d came %d times in 3000 (counts %v)", k, c, n)
		}
	}
	for tile := range 300 {
		h := hostileHost(0x5eed)
		h.foe = 1
		l, id := questMap(h, tile)
		l.questSite(walker, id, 9)
		if len(h.hunts)+len(h.ambushes)+len(h.letters) > 0 || l.f.a.Meta(id).Foe != 1 {
			t.Fatalf("a site that does not approach rolled: %v %v %v", h.hunts, h.ambushes, h.letters)
		}
	}
}

// TestQuestSiteApproachLetterWantsANewThreat: the site's guards stand
// already, so a roll that sends nobody (a barren biome's pack) posts no
// letter.
func TestQuestSiteApproachLetterWantsANewThreat(t *testing.T) {
	t.Parallel()
	tile := 0
	for OreThreat(1, tile, 1) != OreAnimals {
		tile++
	}
	h := &barrenHost{siteHost: hostileHost(1)}
	h.approach, h.hostiles = true, 4
	a := &Atlas{}
	id := a.Add(Site, tile, 0)
	a.Meta(id).Quest = 9
	Of(&[]Caravan{}, a, h).Life().questSite(walker, id, 9)
	if len(h.letters) != 0 {
		t.Fatalf("a pack that never came was announced: %v", h.letters)
	}
}

// barrenHost breeds no pack.
type barrenHost struct{ *siteHost }

func (h *barrenHost) Hunt(uint64, int) {}

// hours runs the hourly census of site map id from hour from to hour to
// (exclusive), on whole hours.
func hours(l Lifecycle, id uint32, from, to int) {
	for hr := from; hr < to; hr++ {
		if l.f.a.Meta(id) != nil {
			l.hourly(id, uint64(hr)*clock.TicksPerHour)
		}
	}
}

// TestQuestSiteReinforcementsOnceAt60Hours: the job done, the site posts
// the reinforcements letter and holds its pack-up; at +60 h with ours still
// there the foe's raid comes once, saved, and only then does the pack-up
// start (QST-017).
func TestQuestSiteReinforcementsOnceAt60Hours(t *testing.T) {
	t.Parallel()
	h := hostileHost(3)
	h.foe, h.ours = 1, 2
	l, id := questMap(h, 4)
	l.questSite(walker, id, 9)
	m := l.f.a.Meta(id)
	m.Hostile = true // the guards were seen, and have fallen
	hours(l, id, 1, 4)
	if m.Reinforce != 3*clock.TicksPerHour+reinforceTicks || len(h.letters) != 1 || h.letters[0] != "Reinforcements on the way" {
		t.Fatalf("job done: reinforce at %d, letters %v", m.Reinforce, h.letters)
	}
	if got := l.f.a.Reinforcing(id, 3*clock.TicksPerHour); got != reinforceTicks {
		t.Fatalf("the read model shows %d ticks to go, want %d", got, reinforceTicks)
	}
	// Saved and loaded halfway: the clock holds.
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(l.f.a); err != nil {
		t.Fatal(err)
	}
	a2 := &Atlas{}
	if err := gob.NewDecoder(&buf).Decode(a2); err != nil {
		t.Fatal(err)
	}
	l = Of(&[]Caravan{}, a2, h).Life()
	m = a2.Meta(id)
	if m.Reinforce != 3*clock.TicksPerHour+reinforceTicks || m.Foe != 1 || m.Reinforced {
		t.Fatalf("loaded: %+v", *m)
	}
	hours(l, id, 4, 63)
	if len(h.ambushes) != 0 || m.PackBy != 0 {
		t.Fatalf("before +60 h: ambushes %v, pack-up by %d", h.ambushes, m.PackBy)
	}
	hours(l, id, 63, 64)
	if len(h.ambushes) != 1 || h.ambushes[0] != 1 || !m.Reinforced || m.PackBy != 0 {
		t.Fatalf("at +60 h: ambushes %v, reinforced %v, pack-up by %d", h.ambushes, m.Reinforced, m.PackBy)
	}
	if l.f.a.Reinforcing(id, 63*clock.TicksPerHour) != 0 {
		t.Fatal("the read model still counts down after they came")
	}
	// They stand for a while, then fall: the pack-up follows, the raid never again.
	hours(l, id, 64, 80)
	h.hostiles = 0
	hours(l, id, 80, 83)
	if len(h.ambushes) != 1 || m.PackBy != 82*clock.TicksPerHour+packTicks {
		t.Fatalf("after the raid: ambushes %v, pack-up by %d", h.ambushes, m.PackBy)
	}
}

// TestQuestSiteReinforcementsNeedOurs: nobody of ours on the map at +60 h
// (a caravan still walking out keeps the map open) means no raid, and the
// site is done with its reinforcements for good.
func TestQuestSiteReinforcementsNeedOurs(t *testing.T) {
	t.Parallel()
	h := hostileHost(3)
	h.foe, h.ours = 1, 2
	list := []Caravan{{C: caravan.Caravan{ID: 51, State: caravan.Forming}, Map: 1, Exiting: []entity.EntityID{7}}}
	a := &Atlas{}
	id := a.Add(Site, 4, 0)
	a.Meta(id).Quest, a.Meta(id).Hostile = 9, true
	l := Of(&list, a, h).Life()
	l.questSite(walker, id, 9)
	hours(l, id, 1, 4)
	h.ours = 0
	hours(l, id, 4, 64)
	m := a.Meta(id)
	if m == nil || len(h.ambushes) != 0 || !m.Reinforced {
		t.Fatalf("with nobody of ours: ambushes %v, meta %+v", h.ambushes, m)
	}
}

// TestQuestSiteWithoutFoePacksUpAsBefore: a site with no hostile faction
// behind it (a refugee nobody guards) or a foe no longer hostile has no
// reinforcements: the pack-up starts when the job is done.
func TestQuestSiteWithoutFoePacksUpAsBefore(t *testing.T) {
	t.Parallel()
	for _, foe := range []int{0, 1, 2} {
		h := hostileHost(3)
		h.facs[0].Relation = faction.Neutral // foe 1 made peace; foe 2 does not exist
		h.foe, h.ours = foe, 1
		l, id := questMap(h, 4)
		l.questSite(walker, id, 9)
		hours(l, id, 1, 4)
		m := l.f.a.Meta(id)
		if m.PackBy != 3*clock.TicksPerHour+packTicks || m.Reinforce != 0 || len(h.letters) != 1 || h.letters[0] != "Time to pack up" {
			t.Fatalf("foe %d: pack-up by %d, reinforce %d, letters %v", foe, m.PackBy, m.Reinforce, h.letters)
		}
	}
}

// TestQuestSiteReinforcementsWaitForTheQuest: while the quest still needs
// the map (busy) the clock does not start, and alternating results never
// start it (no flap: closeStreak done hours in a row).
func TestQuestSiteReinforcementsWaitForTheQuest(t *testing.T) {
	t.Parallel()
	h := hostileHost(3)
	h.foe, h.ours, h.busy = 1, 1, true
	l, id := questMap(h, 4)
	l.questSite(walker, id, 9)
	m := l.f.a.Meta(id)
	hours(l, id, 1, 20)
	for hr := 20; hr < 40; hr++ {
		h.busy = hr%2 == 0
		hours(l, id, hr, hr+1)
	}
	if m.Reinforce != 0 || m.PackBy != 0 || len(h.letters) != 0 {
		t.Fatalf("busy or flapping: reinforce %d, pack-up %d, letters %v", m.Reinforce, m.PackBy, h.letters)
	}
}

// TestQuestSiteReinforcementsOutlastAStuckReform: a reform the player asked
// for that found no way out (PackBy pushed on) neither cancels nor delays
// the reinforcements.
func TestQuestSiteReinforcementsOutlastAStuckReform(t *testing.T) {
	t.Parallel()
	h := hostileHost(3)
	h.foe, h.ours = 1, 1
	l, id := questMap(h, 4)
	l.questSite(walker, id, 9)
	hours(l, id, 1, 4)
	m := l.f.a.Meta(id)
	m.PackBy = 1 << 40 // retried far later
	hours(l, id, 4, 64)
	if len(h.ambushes) != 1 || !m.Reinforced {
		t.Fatalf("with a stuck reform: ambushes %v, reinforced %v", h.ambushes, m.Reinforced)
	}
}
