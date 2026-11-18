// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/research"
)

// The remaps move saved IDs of each PLG-026 class onto the run's book by
// key, as remapRecipes does; a save without that class's keys is left as
// it is. Each is one call into savefmt, owned by its class's agent.

// remapItems moves saved item IDs onto book by key (PLG-026, as
// remapRecipes does); a save without item keys is left as it is.
// CONTRACT STUB (wave 0): the items agent fills it in.
func remapItems(s *saveState, book item.Book) { _, _ = s, book }

// remapTraits moves saved trait IDs onto book by key (PLG-026, as
// remapRecipes does); a save without trait keys is left as it is.
// CONTRACT STUB (wave 0): the traits agent fills it in.
func remapTraits(s *saveState, book trait.Book) { _, _ = s, book }

// remapResearch moves saved research IDs onto book by key (PLG-026, as
// remapRecipes does); a save without research keys is left as it is.
// CONTRACT STUB (wave 0): the research agent fills it in.
func remapResearch(s *saveState, book research.Book) { _, _ = s, book }

// remapIncidents moves saved incident IDs onto book by key (PLG-026, as
// remapRecipes does); a save without incident keys is left as it is.
// CONTRACT STUB (wave 0): the incidents agent fills it in.
func remapIncidents(s *saveState, book incident.Book) { _, _ = s, book }
