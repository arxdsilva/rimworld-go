// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savelegacy

import (
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
)

// Current extends the historical skill array without changing its IDs.
func (s SkillSet) Current() (out skill.Set) {
	copy(out[:], s[:])
	out[skill.Artistic].Level = skill.Neutral
	return
}

func (p Pawn) Current() pawngen.Pawn {
	return pawngen.Pawn{
		First: p.First, Nick: p.Nick, Last: p.Last, Look: p.Look,
		AgeDays: p.AgeDays, Childhood: p.Childhood, Adulthood: p.Adulthood,
		Traits: p.Traits, Skills: p.Skills.Current(),
		Conditions: p.Conditions, Temperament: p.Temperament,
	}
}

// MapValues converts historical maps, preserving nil and explicit empty maps.
func mapValues[K comparable, A, B any](src map[K]A, convert func(A) B) map[K]B {
	if src == nil {
		return nil
	}
	out := make(map[K]B, len(src))
	for k, v := range src {
		out[k] = convert(v)
	}
	return out
}

func Bios(src map[entity.EntityID]Pawn) map[entity.EntityID]pawngen.Pawn {
	return mapValues(src, Pawn.Current)
}

func Skills(src map[entity.EntityID]SkillSet) map[entity.EntityID]skill.Set {
	return mapValues(src, SkillSet.Current)
}

func Pawns(src []Pawn) []pawngen.Pawn {
	if src == nil {
		return nil
	}
	out := make([]pawngen.Pawn, len(src))
	for i, p := range src {
		out[i] = p.Current()
	}
	return out
}

func (l Learn) current() (out CurrentLearn) {
	out.Day = l.Day
	copy(out.Today[:], l.Today[:])
	copy(out.Carry[:], l.Carry[:])
	for i := range l.Week {
		copy(out.Week[i][:], l.Week[i][:])
	}
	return
}

func (s *SkillLive) Current() *CurrentSkillLive {
	if s == nil {
		return nil
	}
	return &CurrentSkillLive{
		Learned:   mapValues(s.Learned, Learn.current),
		NeedCarry: s.NeedCarry, Working: s.Working, Research: s.Research,
	}
}

// Current preserves the public v20 setting, including an explicit zero.
// Older saves are given the historical full friendly-fire value by the
// version-specific migration. Other controls start at neutral values.
func (d DifficultySettings) Current() difficulty.Settings {
	ff := d.FriendlyFirePct
	if d.BalanceVersion != 0 {
		ff = d.FriendlyFire
	}
	return difficulty.Settings{
		ThreatScale: d.ThreatScale, BigThreats: d.BigThreats, PeacefulDays: d.PeacefulDays,
		Adaptation: d.Adaptation, AdaptStrength: d.AdaptStrength, AdaptGrowth: 100,
		FoodPoisonChance: 100, TimeEscalation: d.TimeEscalation, EscPerYear: d.EscPerYear,
		IgnoreWealth: d.IgnoreWealth, CropYield: d.CropYield, MineYield: d.MineYield,
		ButcherYield: d.ButcherYield, ResearchSpeed: d.ResearchSpeed, MoodOffset: d.MoodOffset,
		Infection: d.Infection, DiseaseFreq: d.DiseaseFreq, AnimalRevenge: d.AnimalRevenge,
		FriendlyFire: ff, BalanceVersion: 3, PredatorsHuntColonists: d.PredatorsHuntColonists,
		ExtremeWeather: d.ExtremeWeather, PainTolerance: d.PainTolerance,
		QuestRewardPct: d.QuestRewardPct, QuestThreats: d.QuestThreats,
	}
}
