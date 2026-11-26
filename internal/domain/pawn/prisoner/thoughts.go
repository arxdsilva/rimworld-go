// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package prisoner

import "rimworld-go/internal/domain/pawn/mood"

// Thoughts describes the penalty only a held prisoner receives.
func Thoughts(held bool) []mood.Thought {
	if !held {
		return nil
	}
	return mood.Active(nil, mood.Imprisoned)
}

// WithThoughts adds the captivity thought once to the pawn's own moods.
func WithThoughts(thoughts []mood.Thought, held bool) []mood.Thought {
	if !held {
		return thoughts
	}
	for _, thought := range thoughts {
		if thought.Kind == mood.Imprisoned {
			return thoughts
		}
	}
	return append(thoughts, Thoughts(true)...)
}
