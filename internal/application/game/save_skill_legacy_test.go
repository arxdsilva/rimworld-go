// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/savelegacy"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/entity"
)

func TestV20SkillArraysMigrateIntoCanonicalFields(t *testing.T) {
	const id entity.EntityID = 71
	var skills savelegacy.SkillSet
	var learned savelegacy.Learn
	learned.Day = 16
	for i := range skills {
		skills[i] = skill.Skill{
			Level: uint8(i + 1), XP: int32(i * 71), Carry: int16(i * 3),
		}
		learned.Today[i] = int32(i + 91)
		learned.Carry[i] = int16(i + 4)
		for day := range learned.Week {
			learned.Week[day][i] = int32(i + day*100)
		}
	}
	skills[skill.Melee].Passion = skill.Major
	oldPawn := savelegacy.Pawn{First: "Ada", Nick: "Ada", Last: "Test",
		AgeDays: 1800, Skills: skills, Traits: []trait.ID{trait.Sunny}}

	// This wire payload has only v20 field names and array widths. It is
	// deliberately encoded through gob, as real historical saves are.
	wire := struct {
		Bios      map[entity.EntityID]savelegacy.Pawn
		Skills    map[entity.EntityID]savelegacy.SkillSet
		SkillLive *savelegacy.SkillLive
		NG1       *savelegacy.SetupState
		People    *savelegacy.People
	}{
		Bios:   map[entity.EntityID]savelegacy.Pawn{id: oldPawn},
		Skills: map[entity.EntityID]savelegacy.SkillSet{id: skills},
		SkillLive: &savelegacy.SkillLive{
			Learned:   map[entity.EntityID]savelegacy.Learn{id: learned},
			NeedCarry: map[entity.EntityID]savelegacy.NeedCarry{id: {Hunger: 8, Rest: 4, Joy: 1}},
			Working:   map[entity.EntityID]savelegacy.Working{id: {Skill: skill.Melee, Tick: 320}},
			Research:  37,
		},
		NG1: &savelegacy.SetupState{
			Adapt: savelegacy.Adapter{Days: 16, Value: 9},
			Setup: savelegacy.Setup{
				Seed: 123, Colonists: []savelegacy.Pawn{oldPawn},
				Difficulty: savelegacy.DifficultySettings{
					ThreatScale: 175, AdaptStrength: 60, ExtremeWeather: true,
					QuestRewardPct: 130, QuestThreats: true,
				},
			},
		},
		People: &savelegacy.People{Version: 1, Seeded: true, Persons: []savelegacy.Person{
			{ID: id + 1, Bio: &oldPawn, Faction: 2, Origin: faction.Type(1)},
		}},
	}
	var raw bytes.Buffer
	if err := gob.NewEncoder(&raw).Encode(wire); err != nil {
		t.Fatal(err)
	}
	var s saveState
	if err := gob.NewDecoder(&raw).Decode(&s); err != nil {
		t.Fatalf("decode v20 wire layout: %v", err)
	}
	s.migrateSkillArrays()

	want := skill.Set{}
	copy(want[:], skills[:])
	want[skill.Artistic].Level = skill.Neutral
	if got := s.Skills21[id]; got != want {
		t.Fatalf("migrated live skills %+v, want %+v", got, want)
	}
	pawn, ok := s.Bios21[id]
	if !ok || pawn.First != oldPawn.First || pawn.Last != oldPawn.Last ||
		!reflect.DeepEqual(pawn.Traits, oldPawn.Traits) || pawn.Skills != want {
		t.Fatalf("migrated identity %+v", pawn)
	}
	if s.SkillLive21 == nil || s.SkillLive21.Research != 37 {
		t.Fatalf("research/skill live %#v", s.SkillLive21)
	}
	log := s.SkillLive21.Learned[id]
	if log.Day != learned.Day || log.Today[skill.Artistic] != 0 ||
		log.Carry[skill.Artistic] != 0 {
		t.Fatalf("learn log header %+v", log)
	}
	for i := range skills {
		if log.Today[i] != learned.Today[i] || log.Carry[i] != learned.Carry[i] {
			t.Fatalf("learn log skill %d changed", i)
		}
		for day := range log.Week {
			if log.Week[day][i] != learned.Week[day][i] {
				t.Fatalf("learn log week %d skill %d changed", day, i)
			}
		}
	}
	if s.SkillLive21.NeedCarry[id].Hunger != 8 ||
		s.SkillLive21.Working[id].Tick != 320 {
		t.Fatal("fractional work and need carries were dropped")
	}
	if s.NG121 == nil || s.NG121.Setup.Seed != 123 ||
		len(s.NG121.Setup.Colonists) != 1 || s.NG121.Setup.Colonists[0].Skills != want {
		t.Fatal("wizard colonist skills were not migrated")
	}
	if got := s.NG121.Adapt; got.Days != 16 || got.Value != 9 || got.GrowthCarry != 0 {
		t.Fatalf("legacy adaptation %+v", got)
	}
	ds := s.NG121.Setup.Difficulty
	if ds.ThreatScale != 175 || ds.AdaptStrength != 60 || !ds.ExtremeWeather ||
		ds.QuestRewardPct != 130 || !ds.QuestThreats ||
		ds.AdaptGrowthPercent() != 100 || ds.FoodPoisonPercent() != 100 {
		t.Fatalf("legacy difficulty settings %+v", ds)
	}
	if s.People21 == nil || len(s.People21.Persons) != 1 ||
		s.People21.Persons[0].Bio == nil || s.People21.Persons[0].Bio.Skills != want || s.People21.Persons[0].Origin != faction.Type(1) {
		t.Fatal("off-map person skills were not migrated")
	}
	if s.Bios != nil || s.Skills != nil || s.SkillLive != nil || s.NG1 != nil || s.People != nil {
		t.Fatal("v20 wire fields remained populated after migration")
	}

	// A v21 body is self-contained: historical fields are absent when
	// loaded again, and no second migration is necessary.
	raw.Reset()
	if err := gob.NewEncoder(&raw).Encode(s); err != nil {
		t.Fatal(err)
	}
	var current saveState
	if err := gob.NewDecoder(&raw).Decode(&current); err != nil {
		t.Fatal(err)
	}
	if current.Skills != nil || current.Bios != nil || current.SkillLive != nil ||
		current.NG1 != nil || current.People != nil || current.Skills21[id] != want {
		t.Fatal("v21 save retained legacy data or lost the migrated skills")
	}
}
