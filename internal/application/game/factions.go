package game

import (
	"fmt"

	"rimworld-go/internal/application/game/factionview"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/storyteller"
	"rimworld-go/pkg/randx"
)

// Factions in play (NG-1 §5.5). Owned by SETUP. A legacy colony has none:
// its raids stay unattributed ("Raiders").

// raidRepelled is the goodwill a faction loses when its raid is defeated
// or leaves.
const raidRepelled = -5

// Factions is the colony's Factions window.
func (g *Game) Factions() []FactionView { return FactionViewsOf(g.ng.factions) }

// factionView is one faction row (game/factionview).
func factionView(f faction.Faction) FactionView { return factionview.Of(f) }

// Story is the colony's storyteller, difficulty and save mode.
func (g *Game) Story() StoryView {
	if !g.hasSetup() {
		p, _ := storyteller.ByKey("legacy")
		return StoryView{Storyteller: p.Name, Title: p.Title}
	}
	s := g.ng.setup
	p := g.storyProfile()
	v := StoryView{
		Storyteller: p.Name, Title: p.Title, Preset: s.Preset.String(), Difficulty: s.Difficulty,
		Commitment: s.SaveMode == difficulty.Commitment,
	}
	if s.Difficulty.Adaptation {
		v.Adaptation = int(g.ng.adapt.Value)
	}
	return v
}

// factionOf is the faction of a raid group's Faction field (1 + index;
// nil: unattributed).
func (g *Game) factionOf(ref int) *faction.Faction {
	if ref <= 0 || ref > len(g.ng.factions) {
		return nil
	}
	return &g.ng.factions[ref-1]
}

// hostileRaider reports a hostile human faction that can raid.
func hostileRaider(g *Game) bool {
	for _, f := range g.ng.factions {
		if f.Type.Human() && f.Relation == faction.Hostile && !f.Defeated {
			return true
		}
	}
	return false
}

// factionsDay is the factions' day rollover: goodwill drifts toward each
// type's baseline and the history grows.
func (g *Game) factionsDay() {
	for i := range g.ng.factions {
		faction.DailyDrift(&g.ng.factions[i])
	}
}

// checkRaidsOver costs a faction goodwill once its raid is over: no
// raider of the group is still standing on the map. The factions at war
// with it give goodwill for it (faction.EnemyBeaten, EAI-015).
func (g *Game) checkRaidsOver(now uint64) {
	for gi := range g.raidGroups {
		grp := &g.raidGroups[gi]
		if grp.Faction == 0 || grp.Over {
			continue
		}
		standing := false
		for _, id := range g.raiderIDs {
			if g.raiders[id].Group == gi && g.standingRaider(id) {
				standing = true
				break
			}
		}
		if standing {
			continue
		}
		grp.Over = true
		if f := g.factionOf(grp.Faction); f != nil {
			faction.Adjust(f, raidRepelled, fmt.Sprintf("Day %d: raid repelled (%d goodwill)", g.relDay(now)+1, raidRepelled))
			// EAI-015: its enemies thank the colony, once for this raid
			// (Over) and at most once per cooldown each
			day := g.relDay(now)
			faction.EnemyBeaten(g.ng.factions, f.ID, day, fmt.Sprintf("Day %d: a raid of their enemies, %s, beaten (%+d goodwill)", day+1, f.Name, faction.EnemyBonus))
		}
	}
}

// RivalDelay is how long after the colony takes a faction's side (its
// military aid arrives) that faction's enemy raids it (EAI-015, original
// tuning).
const RivalDelay = 3 * clock.TicksPerDay

// queueRival: the colony has taken faction of's side (index); one of its
// enemies hostile to the colony queues a raid for RivalDelay from now
// (incident_queue.go), sized when it comes. False: of has no such enemy.
func queueRival(g *Game, of int, now uint64) bool {
	ri := faction.Rival(g.ng.factions, of, g.rng.IntN)
	if ri < 0 {
		return false
	}
	g.QueueRivalRaid(now+RivalDelay, ri, 0)
	return true
}

// rivalRaid sends faction fi's (index) raid at the colony: a queued rival
// raid firing (EAI-015), of points threat points (0: the colony's today).
// False: fi can no longer raid (defeated, or no longer hostile), and the
// queue sends whoever can instead.
func rivalRaid(g *Game, fi, points int, now uint64) bool {
	if !g.hasSetup() || fi < 0 || fi >= len(g.ng.factions) || !faction.Raider(g.ng.factions[fi]) {
		return false
	}
	if points <= 0 {
		points = g.threatPoints()
	}
	factionRaid(g, fi, points, g.relDay(now)) // story.go
	return true
}

// DevAddFactions gives a colony with no factions the default slots
// (dev scenarios: a legacy capture colony has none, so traders and
// visitors never come). The draw uses its own fixed-seed generator so no
// simulation stream moves.
func (g *Game) DevAddFactions() {
	if len(g.ng.factions) > 0 {
		return
	}
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.ng.factions = faction.Generate(faction.DefaultSlots(), randx.New(g.streams.seed^0xfac7))
}
