// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

var implantBenchValue int32
var implantBenchPart Part

func BenchmarkImplantHealthyCapacity(b *testing.B) {
	var pawn Body
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		implantBenchValue = pawn.Capacity(Moving)
	}
}

func BenchmarkImplantAugmentedCapacity(b *testing.B) {
	var pawn Body
	pawn.SetReplacement(ArmL, item.ServoArm)
	pawn.SetReplacement(LegL, item.ServoLeg)
	pawn.Missing |= bit(ArmL) | bit(LegL)
	AddImplant(&pawn, Brain, item.BlissCircuit)
	AddImplant(&pawn, Brain, item.NerveDampener)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		implantBenchValue = pawn.Capacity(Moving)
	}
}

func BenchmarkImplantPickHitPart(b *testing.B) {
	var pawn Body
	pawn.Missing |= bit(ArmL) | bit(HandL)
	pawn.SetReplacement(ArmL, item.ServoArm)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		implantBenchPart, _ = pawn.PickPart(func(n int) int { return n / 2 })
	}
}

func BenchmarkImplantHealthParts(b *testing.B) {
	var pawn Body
	pawn.Missing |= bit(LegL) | bit(FootL)
	pawn.SetReplacement(LegL, item.ServoLeg)
	AddImplant(&pawn, Brain, item.BlissCircuit)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = pawn.Parts()
	}
}
