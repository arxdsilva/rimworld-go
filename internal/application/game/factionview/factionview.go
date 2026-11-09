// Package factionview is the faction rows of the setup wizard and the
// colony's Factions window (game.FactionView, game.FactionViewsOf). Plain
// data built from a faction: no state, no host.
package factionview

import (
	"slices"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/research"
)

// Row is a faction row (setup and colony; game.FactionView).
type Row struct {
	ID          int
	Name, Type  string
	Leader      string
	LeaderTitle string
	Goodwill    int
	Relation    string // "hostile", "neutral", "friendly"
	Settlements int
	Tech        string
	Emblem      string // ui_icon key
	Accent      string // hex
	History     []int16
	Events      []string
	Hidden      bool
	// Enemies names the factions it is at war with, in id order (EAI-015;
	// All fills it, Of alone leaves it empty).
	Enemies []string
}

// Of is faction f's row.
func Of(f faction.Faction) Row {
	d := faction.DefOf(f.Type)
	rel := "neutral"
	switch f.Relation {
	case faction.Hostile:
		rel = "hostile"
	case faction.Friendly:
		rel = "friendly"
	}
	tech := research.LevelOf(d.Tech).String() // RSCH-0: the research scale names the level
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
	return Row{
		ID: f.ID, Name: f.Name, Type: d.Label, Leader: f.Leader.Name, LeaderTitle: f.Leader.Title,
		Goodwill: f.Goodwill, Relation: rel, Settlements: len(f.Settlements), Tech: tech,
		Emblem: "ui_icon/faction_" + d.Key, Accent: d.Accent,
		History: slices.Clone(hist), Events: slices.Clone(events), Hidden: d.Hidden,
	}
}

// All is the rows of fs, in order.
func All(fs []faction.Faction) []Row {
	out := make([]Row, 0, len(fs))
	for _, f := range fs {
		r := Of(f)
		r.Enemies = faction.EnemyNames(fs, f)
		out = append(out, r)
	}
	return out
}
