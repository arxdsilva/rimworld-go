// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/research"
)

// The remaps move saved IDs of each PLG-026 class onto the run's book by
// key, as remapRecipes does; a save without that class's keys is left as
// it is. Each is one call into savefmt, owned by its class's agent.

// remapItems moves saved item kinds onto book by key (PLG-026, as
// remapRecipes does): every item.Kind and storage filter the state holds
// (savefmt.ItemKinds.Walk), and the bill-ingredient commands still
// pending; a gone plugin's stacks are removed. A save without item keys,
// or with the run's own, is left as it is.
func remapItems(s *saveState, book item.Book) {
	to := s.ContentKeys.Items(book)
	s.Pending = to.Commands(s.Pending)
	to.Walk(s)
}

// remapBackstories moves saved backstory IDs onto book by key (PLG-044):
// colonists, setup candidates and the world's people.
func remapBackstories(s *saveState, book backstory.Book) {
	var colonists []pawngen.Pawn
	if s.NG1 != nil {
		colonists = s.NG1.Setup.Colonists
	}
	s.ContentKeys.RemapBackstories(book, s.Bios, colonists, peopleBios(s)...)
}

// peopleBios is every saved world person's bio, to remap in place.
func peopleBios(s *saveState) []*pawngen.Pawn {
	var out []*pawngen.Pawn
	if s.People != nil {
		for i := range s.People.Persons {
			out = append(out, s.People.Persons[i].Bio)
		}
	}
	return out
}

// remapTraits moves saved trait IDs onto book by key (PLG-026, as
// remapRecipes does) in the colonists' bios, the setup's colonists and
// every other person's bio; a trait whose plugin is gone is taken off the
// pawn. A save without trait keys is left as it is.
func remapTraits(s *saveState, book trait.Book) {
	var colonists []pawngen.Pawn
	if s.NG1 != nil {
		colonists = s.NG1.Setup.Colonists
	}
	s.ContentKeys.RemapTraits(book, s.Bios, colonists, peopleBios(s)...)
}

// remapResearch moves saved research IDs onto book by key (PLG-026, as
// remapRecipes does): progress, the current project and pending picks; a
// gone plugin's project loses its points and its selection. A save
// without research keys is left as it is.
func remapResearch(s *saveState, book research.Book) {
	to := s.ContentKeys.Research(book)
	if s.ResearchAll != nil {
		s.ResearchAll = to.Progress(s.ResearchAll)
	}
	s.ResearchCur, s.ResearchSet = to.Current(s.ResearchCur, s.ResearchSet)
	s.Pending = to.Commands(s.Pending)
}

// remapIncidents moves saved incident IDs onto book by key (PLG-026, as
// remapRecipes does); a save without incident keys is left as it is.
func remapIncidents(s *saveState, book incident.Book) {
	s.ContentKeys.RemapIncidents(book, &s.Incidents)
}
