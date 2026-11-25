// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package letters

import (
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/pkg/letterdata"
	"strconv"
)

// DiseaseCode names the illness represented by an outbreak letter.
func DiseaseCode(k body.CondKind) string {
	switch k {
	case body.CondFlu:
		return "colony.disease.flu"
	case body.CondPlague:
		return "colony.disease.plague"
	case body.CondMalaria:
		return "colony.disease.malaria"
	case body.CondSleepingSickness:
		return "colony.disease.sleeping_sickness"
	case body.CondGutWorms:
		return "colony.disease.gut_worms"
	case body.CondMuscleParasites:
		return "colony.disease.muscle_parasites"
	}
	return ""
}

// RaidCode names a raid arrival independently of its rendered message.
func RaidCode(plan raid.Plan) string {
	switch plan {
	case raid.Siege:
		return "colony.raid.siege"
	case raid.Sappers:
		return "colony.raid.sappers"
	}
	return "colony.raid"
}

// RaidFields records the number, faction/group and entrance direction.
func RaidFields(count int, who, whoKind, direction string) []letterdata.Field {
	fields := []letterdata.Field{{Name: "count", Kind: "count", Value: strconv.Itoa(count)}}
	if who != "" {
		fields = append(fields, letterdata.Field{Name: "who", Kind: whoKind, Value: who})
	}
	if direction != "" {
		fields = append(fields, letterdata.Field{Name: "direction", Kind: "direction", Value: direction})
	}
	return fields
}

// DiseaseFields records the diagnosed condition and prescribed treatment.
func DiseaseFields(k body.CondKind, who string, count int) []letterdata.Field {
	d := body.DefOfCond(k)
	fields := []letterdata.Field{
		{Name: "who", Kind: "person", Value: who},
		{Name: "what", Kind: "condition", Value: d.Label},
		{Name: "count", Kind: "count", Value: strconv.Itoa(count)},
		{Name: "duration", Kind: "duration", Value: strconv.Itoa(d.TendEvery)},
	}
	if d.CureTendSum > 0 {
		fields = append(fields, letterdata.Field{
			Name: "cure_quality", Kind: "count", Value: strconv.Itoa(int(d.CureTendSum / 10)),
		})
	}
	return fields
}
