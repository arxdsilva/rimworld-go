// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

// Aging and chronic conditions (MED-08): the definitions. The numbers are
// the wiki's (ENG-003/MED exception), as permille; the names are the
// real-world illnesses'. How they start, grow and end is in aging.go.
//
// The stage mods are capacity offsets, so a per-part effect is folded
// into its capacity: a cataract's "eye −50 %" is sight −25 % (sight is the
// mean of the two eyes, so two cataracts give 50 % sight, as on the
// wiki), hearing loss the same per ear. Artery blockage's heart efficiency
// is blood pumping, which the heart alone does.
//
// None of them build immunity, take the race engine's tending or kill
// through it (they are not Lethal): the game's aging step (StepAging)
// grows them, kills at full severity and removes them. Only the heart
// attack takes tending (TendEvery), which StepHeartAttack reads.
var condsAging = []CondEntry{
	{CondBadBack, CondDef{
		Label:      "bad back",
		NoImmunity: true,
		Stages: []Stage{{At: 0, // wiki: Ailments (bad back: moving −30 %, manipulation −10 %)
			Mods: CapMods{off(Moving, -300), off(Manipulation, -100)}}},
	}},
	{CondCataract, CondDef{
		Label:      "cataract",
		NoImmunity: true,
		Stages:     []Stage{{At: 0}}, // wiki: Ailments, -50% of the affected eye efficiency
	}},
	{CondFrail, CondDef{
		Label:      "frail",
		NoImmunity: true,
		Stages: []Stage{{At: 0, // wiki: Ailments (frail: moving and manipulation −30 %)
			Mods: CapMods{off(Moving, -300), off(Manipulation, -300)}}},
	}},
	{CondDementia, CondDef{
		Label:      "dementia",
		NoImmunity: true,
		Stages: []Stage{{At: 0, // wiki: Ailments (dementia: brain −15 %, talking and hearing −25 %)
			Mods: CapMods{off(Consciousness, -150), off(Talking, -250), off(Hearing, -250)}}},
	}},
	{CondHearingLoss, CondDef{
		Label:      "hearing loss",
		NoImmunity: true,
		Stages:     []Stage{{At: 0}}, // wiki: Ailments, -50% of the affected ear efficiency
	}},
	{CondArteryBlockage, CondDef{
		Label:      "artery blockage",
		NoImmunity: true,
		Stages: []Stage{ // wiki: Ailments (artery blockage: heart efficiency by stage)
			{At: 0, Label: "minor", Mods: CapMods{off(BloodPumping, -50)}},
			{At: 200, Label: "minor", Mods: CapMods{off(BloodPumping, -100)}},
			{At: 400, Label: "major", Mods: CapMods{off(BloodPumping, -150)}},
			{At: 600, Label: "major", Mods: CapMods{off(BloodPumping, -350)}},
			{At: 900, Label: "extreme", Mods: CapMods{off(BloodPumping, -600)}},
		},
	}},
	{CondCarcinoma, CondDef{
		Label:      "carcinoma",
		NoImmunity: true,
		TendEvery:  4 * 24, // wiki: Carcinoma, every 240000 ticks
		// wiki: Carcinoma; part efficiency offsets at 0/15/40/60/80%
		// are applied to the affected part by Body.eff.
		Stages: []Stage{
			{At: 0, Label: "minor", PainAdd: 100},
			{At: 150, Label: "minor", PainAdd: 200},
			{At: 400, Label: "major", PainAdd: 350},
			{At: 600, Label: "major", PainAdd: 500},
			{At: 800, Label: "extreme", PainAdd: 600},
		},
	}},
	{CondAsthma, CondDef{
		Label:      "asthma",
		NoImmunity: true,
		TendEvery:  7 * 24, // wiki: Asthma, every 420000 ticks
		Stages: []Stage{
			{At: 0, Label: "minor"},
			{At: 300, Label: "major"},
			{At: 450, Label: "major"},
		}, // wiki: Asthma, affected lung efficiency -10/-30/-50%
	}},
	{CondAlzheimers, CondDef{
		Label:      "alzheimers",
		NoImmunity: true,
		Stages: []Stage{
			{At: 0, Label: "minor"},
			{At: 200, Label: "minor"},
			{At: 500, Label: "major"},
			{At: 800, Label: "major"},
		}, // wiki: Alzheimer's, brain efficiency -5/-10/-15/-20%
	}},
	{CondHeartAttack, CondDef{
		Label:      "heart attack",
		NoImmunity: true,
		TendEvery:  heartAttackTendEvery,
		Stages: []Stage{ // wiki: Ailments (heart attack)
			// painful: consciousness ×50 % (an offset of −50 % on a
			// healthy pawn), pain +40 %
			{At: 0, Label: "painful", PainAdd: 400, Mods: CapMods{off(Consciousness, -500)}},
			// debilitating: consciousness at most 10 %, pain +60 %
			{At: 600, Label: "debilitating", PainAdd: 600, Mods: CapMods{capMax(Consciousness, 100)}},
		},
	}},
}
