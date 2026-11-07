package quest

import (
	"slices"

	qd "rimworld-go/internal/domain/quest"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/pawngen"
)

// Helpers for the game's role adapters (quest_host.go): pure functions of
// domain values, so the glue stays a thin layer.

// FactionViews is fs as the World role returns them (QST-038):
// settlements ascending.
func FactionViews(fs []faction.Faction) []FactionView {
	out := make([]FactionView, len(fs))
	for i, f := range fs {
		out[i] = FactionView{Index: i, Relation: f.Relation, Goodwill: f.Goodwill,
			CanBeFriendly: faction.DefOf(f.Type).CanBeFriendly, Defeated: f.Defeated, Besieged: f.Besieged,
			Settlements: slices.Sorted(slices.Values(f.Settlements))}
	}
	return out
}

// PickSettlement is one of faction f's (index) settlement tiles, picked
// from seed (ok false: no such faction, or it has none).
func PickSettlement(fs []faction.Faction, f int, seed uint64) (int, bool) {
	if f < 0 || f >= len(fs) || len(fs[f].Settlements) == 0 {
		return 0, false
	}
	ts := fs[f].Settlements
	return ts[seed%uint64(len(ts))], true
}

// PawnOf is the person of seed: the preview and the walk-in agree
// (QST-087).
func PawnOf(seed uint64) pawngen.Pawn { return pawngen.Generate(seed, 0, 0, pawngen.Rules{}) }

// PreviewOf is PawnOf(seed) as the window shows it.
func PreviewOf(seed uint64) PawnPreview {
	p := PawnOf(seed)
	name := p.First + " " + p.Last
	if p.Nick != "" {
		name = p.Nick
	}
	bs := p.Adulthood
	if bs == 0 {
		bs = p.Childhood
	}
	return PawnPreview{Name: name, Age: int(p.AgeDays) / pawngen.DaysPerYear, Backstory: backstory.DefOf(bs).Title}
}

// KindName is kind k's name (ours, clean room; "" for none).
func KindName(k qd.KindID) string {
	names := [qd.NumKinds]string{"", "A stranger in need", "Mineral deposit", "Hidden cache",
		"Stranded survivor", "Outlaw camp", "Cell break", "Supply request", "Truce talks"}
	if k >= qd.NumKinds {
		return ""
	}
	return names[k]
}

// ReasonText is why a quest failed, as a letter phrase (ours).
func ReasonText(r qd.Reason) string {
	switch r {
	case qd.Timeout:
		return "time ran out"
	case qd.GiverHostile:
		return "its giver turned hostile"
	case qd.TargetNotHostile:
		return "its target is no longer hostile"
	case qd.PawnLost:
		return "the person it was about was lost"
	case qd.SiteLost:
		return "its site was lost"
	}
	return "it could not go on"
}
