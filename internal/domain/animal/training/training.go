// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package training

import "rimworld-go/internal/domain/sim/entity"

// Skill is a trainable behaviour (append only, saved).
type Skill uint8

const (
	Obedience Skill = iota
	Guard
	Release
	Haul
	NumSkills
)

// Steps is how many successful steps it takes to learn each skill.
const LegacySkills = int(Haul) // historical fixed-width save array

var Steps = [NumSkills]uint8{Obedience: 3, Guard: 3, Release: 2, Haul: 7}

// Maintenance belongs to the world roster across map and caravan travel.
type Maintenance struct {
	DecayAt      uint64
	TameLoss     uint8
	HaulProgress uint8
	Task         HaulTask
}

// HaulTask owns two physical reservations and the exact carried stack.
type HaulTask struct {
	Source, Carry entity.EntityID
	X, Y, Qty     int
	Until         uint64
}

// Bond chances, permille per event (Proximity per BondCheckTicks check).
const (
	BondTame       int32  = 10
	BondTrain      int32  = 7
	BondTend       int32  = 4
	BondProximity  int32  = 1
	BondCheckTicks        = 2500
	DecayMaxTicks  uint64 = 12 * 60000
	DecayMinTicks  uint64 = 6 * 60000
	// BondedFactor multiplies the step chance for a bonded trainer, permille.
	BondedFactor int32 = 5000
)

// Needed returns the steps to learn s (0 for an invalid skill).
func Needed(s Skill) uint8 {
	if s >= NumSkills {
		return 0
	}
	return Steps[s]
}

// Learned reports whether steps complete skill s.
func Learned(s Skill, steps uint8) bool {
	n := Needed(s)
	return n > 0 && steps >= n
}

// StepChance is the permille chance one training session succeeds for a
// trainer of the given Animals skill (0..20); a bonded trainer gets x5,
// capped at 1000.
func StepChance(animalsSkill int, bonded bool) int32 {
	s := int32(min(max(animalsSkill, 0), 20))
	c := 100 + s*25
	if bonded {
		c = c * BondedFactor / 1000
	}
	return min(c, 1000)
}

// DecayInterval interpolates vanilla's 12-to-6-day training interval.
func DecayInterval(wildness int) uint64 {
	w := uint64(min(max(wildness, 0), 1000))
	return DecayMaxTicks - (DecayMaxTicks-DecayMinTicks)*w/1000
}
