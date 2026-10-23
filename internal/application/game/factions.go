package game

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/storyteller"
)

// Factions in play (NG-1 §5.5). Owned by SETUP. A legacy colony has none:
// its raids stay unattributed ("Raiders").

// raidRepelled is the goodwill a faction loses when its raid is defeated
// or leaves.
const raidRepelled = -5

// Factions is the colony's Factions window.
func (g *Game) Factions() []FactionView { return FactionViewsOf(g.ng.factions) }

// factionView is one faction row.
func factionView(f faction.Faction) FactionView {
	d := faction.DefOf(f.Type)
	rel := "neutral"
	switch f.Relation {
	case faction.Hostile:
		rel = "hostile"
	case faction.Friendly:
		rel = "friendly"
	}
	tech := techLabel(d.Tech)
	if f.Type == faction.Swarm {
		tech = "—"
	}
	events := f.Events
	if len(events) > faction.EventCount {
		events = events[len(events)-faction.EventCount:]
	}
	hist := f.History
	if len(hist) > faction.HistoryDays {
		hist = hist[len(hist)-faction.HistoryDays:]
	}
	return FactionView{
		ID: f.ID, Name: f.Name, Type: d.Label, Leader: f.Leader.Name, LeaderTitle: f.Leader.Title,
		Goodwill: f.Goodwill, Relation: rel, Settlements: len(f.Settlements), Tech: tech,
		Emblem: "ui_icon/faction_" + d.Key, Accent: d.Accent,
		History: slices.Clone(hist), Events: slices.Clone(events), Hidden: d.Hidden,
	}
}

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
func (g *Game) hostileRaider() bool {
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
// raider of the group is still standing on the map.
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
		}
	}
}
