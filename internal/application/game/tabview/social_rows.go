// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package tabview

import (
	"slices"

	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/pkg/mathx"
)

// SocialRowFacts contains one independent pair's read-only social facts.
// The game supplies them; this tab-owned builder only formats and orders rows.
type SocialRowFacts struct {
	Other                     uint64
	Name, Status, RomanceNote string
	Male                      bool
	Kin                       []social.Kind
	To, From                  int32
	TermsTo, TermsFrom        []social.Term
	Friend, Rival             bool
}

type rankedSocialRow struct {
	row      RelationRow
	strength int32
}

// SocialRows accumulates presentation rows for a single Social tab read.
type SocialRows struct{ rows []rankedSocialRow }

func (r *SocialRows) Add(f SocialRowFacts) {
	label := ""
	var strength int32
	for _, k := range f.Kin {
		if b := absCenti(social.Bonus(k)); label == "" || b > strength {
			label, strength = social.Label(k, f.Male), b
		}
	}
	if label == "" {
		switch {
		case f.Friend:
			label = "friend"
		case f.Rival:
			label = "rival"
		default:
			label = "acquaintance"
		}
	}
	r.rows = append(r.rows, rankedSocialRow{RelationRow{
		Other: f.Other, Name: f.Name, Label: label, Status: f.Status,
		OpinionTo: CentiPoints(f.To), OpinionFrom: CentiPoints(f.From),
		TermsTo: socialTerms(f.TermsTo), TermsFrom: socialTerms(f.TermsFrom),
		RomanceNote: f.RomanceNote,
	}, strength})
}

// Build orders by strongest kin, absolute opinion and stable entity id.
func (r *SocialRows) Build(log []SocialLogLine) Social {
	slices.SortStableFunc(r.rows, func(a, b rankedSocialRow) int {
		if a.strength != b.strength {
			return int(b.strength - a.strength)
		}
		if da, db := mathx.Abs(a.row.OpinionTo), mathx.Abs(b.row.OpinionTo); da != db {
			return db - da
		}
		if a.row.Other < b.row.Other {
			return -1
		}
		return 1
	})
	v := Social{Log: log}
	for _, row := range r.rows {
		v.Relations = append(v.Relations, row.row)
	}
	return v
}

func absCenti(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// CentiPoints rounds centi-points to whole points (half away from zero).
func CentiPoints(c int32) int {
	if c < 0 {
		return -int((-c + 50) / 100)
	}
	return int((c + 50) / 100)
}

func socialTerms(ts []social.Term) []OpinionTerm {
	if len(ts) == 0 {
		return nil
	}
	out := make([]OpinionTerm, len(ts))
	for i, t := range ts {
		out[i] = OpinionTerm{Label: t.Label, Value: CentiPoints(t.Value)}
	}
	return out
}
