package game

import (
	"slices"

	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// socialView builds pawn id's Social tab: everyone it is related to, has
// a social edge with, or (colonists on the map) holds a non-zero opinion
// of, strongest relation first, then the strongest opinion. It only
// reads: no one is registered and no saved state moves.
func (g *Game) socialView(id entity.EntityID) (SocialView, bool) {
	if g.personOf(id) == nil && !slices.Contains(g.pawnIDs, id) {
		return SocialView{}, false
	}
	others := g.tiesOf(id)
	for _, c := range g.pawnIDs {
		if c != id && g.alive(c) {
			if i, found := slices.BinarySearch(others, c); !found {
				others = slices.Insert(others, i, c)
			}
		}
	}
	type ranked struct {
		row      RelationRowView
		strength int32
	}
	var rows []ranked
	for _, o := range others {
		ks := g.kinOf(id, o)
		to, termsTo := g.opinionTerms(id, o, true)
		from, termsFrom := g.opinionTerms(o, id, true)
		if len(ks) == 0 && g.edgeOf(id, o) == nil && to == 0 && from == 0 {
			continue
		}
		otherMale := g.socialPerson(o).Male
		label := ""
		var strength int32
		for _, k := range ks {
			if b := absI32(social.Bonus(k)); label == "" || b > strength {
				label, strength = social.Label(k, otherMale), b
			}
		}
		if label == "" {
			switch {
			case g.friendOf(id, o):
				label = "friend"
			case g.rivalOf(id, o):
				label = "rival"
			default:
				label = "acquaintance"
			}
		}
		rows = append(rows, ranked{RelationRowView{
			Other: uint64(o), Name: g.personName(o), Label: label, Status: g.personStatus(o),
			OpinionTo: centiPoints(to), OpinionFrom: centiPoints(from),
			TermsTo: termViews(termsTo), TermsFrom: termViews(termsFrom),
			RomanceNote: g.romanceNote(id, o), // social_romance.go (M5)
		}, strength})
	}
	slices.SortStableFunc(rows, func(a, b ranked) int {
		if a.strength != b.strength {
			return int(b.strength - a.strength)
		}
		if da, db := abs(a.row.OpinionTo), abs(b.row.OpinionTo); da != db {
			return db - da
		}
		if a.row.Other < b.row.Other {
			return -1
		}
		return 1
	})
	v := SocialView{Log: g.socialLogViews(id)}
	for _, r := range rows {
		v.Relations = append(v.Relations, r.row)
	}
	return v, true
}

// personStatus is the Social tab's status word for id.
func (g *Game) personStatus(id entity.EntityID) string {
	p := g.personOf(id)
	if p == nil {
		return ""
	}
	switch p.Where {
	case whereDead:
		return "dead"
	case whereOffMap:
		return "off-map"
	case wherePrisoner:
		return "prisoner"
	case whereKidnapped:
		return "kidnapped"
	case whereCaravan:
		return "in caravan"
	case whereRaider:
		return "raider"
	case whereVisitor:
		return "visitor"
	}
	return ""
}

func absI32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// centiPoints rounds centi-points to whole points (half away from zero).
func centiPoints(c int32) int {
	if c < 0 {
		return -int((-c + 50) / 100)
	}
	return int((c + 50) / 100)
}

func termViews(ts []social.Term) []OpinionTermView {
	if len(ts) == 0 {
		return nil
	}
	out := make([]OpinionTermView, len(ts))
	for i, t := range ts {
		out[i] = OpinionTermView{Label: t.Label, Value: centiPoints(t.Value)}
	}
	return out
}

// fillSocialPawn fills colonist pv's social fields: the smoothed mood it
// shows (and the target it moves to) and its latest interaction bubble.
func (g *Game) fillSocialPawn(pv *PawnView, target int) {
	pv.MoodTarget = target
	pv.Mood = g.smoothedMood(pv.ID, target)
	pv.MoodLabel = mood.Label(pv.Mood)
	pv.Interaction, pv.InteractTick = g.lastInteraction(pv.ID)
	pv.SocialFight = g.mind.InFight(pv.ID) // mind/fight.go (M6)
	pv.Prisoner = g.prison.Is(pv.ID)       // prisoners.go (M8)
}
