// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"

	"rimworld-go/internal/application/game/tabview"
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
	var rows tabview.SocialRows
	graph := famGraph{g}
	own := social.AncestryOf(social.ID(id), graph)
	for _, o := range others {
		other := social.AncestryOf(social.ID(o), graph)
		pair := g.pairOf(id, o)
		ks := kinFromAncestry(graph, &own, &other, pair)
		to, termsTo := g.opinionTermsWithKin(id, o, true, func() []social.Kind { return ks })
		from, termsFrom := g.opinionTermsWithKin(o, id, true, func() []social.Kind {
			return kinFromAncestry(graph, &other, &own, pair)
		})
		if len(ks) == 0 && g.edgeOf(id, o) == nil && to == 0 && from == 0 {
			continue
		}
		rows.Add(tabview.SocialRowFacts{
			Other: uint64(o), Name: g.personName(o), Status: g.personStatus(o),
			RomanceNote: g.romanceNote(id, o), Male: g.socialPerson(o).Male,
			Kin: ks, To: to, From: from, TermsTo: termsTo, TermsFrom: termsFrom,
			Friend: g.friendOf(id, o), Rival: g.rivalOf(id, o),
		})
	}
	return rows.Build(g.socialLogViews(id)), true
}

// kinFromAncestry reads family relations from one view's snapshot and
// includes the couple edge, in the same order as Game.kinOf.
func kinFromAncestry(graph famGraph, a, b *social.Ancestry, pair social.Pair) []social.Kind {
	ks := social.AppendKinBetween(nil, a, b, graph)
	if k, ok := pair.Kind.Relation(); ok {
		ks = append(ks, k)
	}
	return ks
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

// centiPoints is retained for existing game test and read call sites.
func centiPoints(c int32) int { return tabview.CentiPoints(c) }

// fillSocialPawn fills colonist pv's social fields: the smoothed mood it
// shows (and the target it moves to) and its latest interaction bubble.
func (g *Game) fillSocialPawn(pv *PawnView, target int) {
	pv.MoodTarget = target
	pv.Mood = target
	if m, ok := g.smoothedMood(pv.ID); ok {
		pv.Mood = m
	}
	pv.MoodLabel = mood.Label(pv.Mood)
	pv.Interaction, pv.InteractTick = g.lastInteraction(pv.ID)
	pv.SocialFight = g.mind.InFight(pv.ID) // mind/fight.go (M6)
	pv.Prisoner = g.prison.Is(pv.ID)       // prisoners.go (M8)
}
