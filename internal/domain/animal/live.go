// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

import "rimworld-go/internal/domain/pawn/body"

// Live points at the independently owned components of one active animal.
// State remains the detached legacy and off-map record.
type Live struct {
	*Identity
	*Vitals
	*Intent
	*Motion
	*Threat
	*Feeding
	*Handling
	*Reproduction
	*Medical
}

func (a *Live) Snapshot() Snapshot {
	return Snapshot{
		Identity: *a.Identity, Vitals: *a.Vitals, Intent: *a.Intent,
		Motion: *a.Motion, Threat: *a.Threat, Feeding: *a.Feeding,
		Handling: *a.Handling, Reproduction: *a.Reproduction,
		Medical: *a.Medical,
	}
}

// State reconstructs a detached legacy record directly from the groups.
func (a *Live) State() State {
	return State{
		Species:           a.Species,
		Tame:              a.Tame,
		Name:              a.Name,
		Female:            a.Female,
		HasSex:            a.HasSex,
		Born:              a.Born,
		Herd:              a.Herd,
		Injury:            a.Injury,
		Hunger:            a.Hunger,
		Food:              a.Food,
		Rest:              a.Rest,
		Starving:          a.Starving,
		Downed:            a.Downed,
		DownedTick:        a.DownedTick,
		Mode:              a.Mode,
		Target:            a.Target,
		NextThink:         a.NextThink,
		Cooldown:          a.Cooldown,
		NextDecide:        a.NextDecide,
		Leaving:           a.Leaving,
		ExitX:             a.ExitX,
		ExitY:             a.ExitY,
		Errand:            a.Errand,
		Dormant:           a.Dormant,
		Path:              a.Path,
		PathI:             a.PathI,
		GoalX:             a.GoalX,
		GoalY:             a.GoalY,
		RetryAt:           a.RetryAt,
		StepGait:          a.StepGait,
		Attacker:          a.Attacker,
		FleeUntil:         a.FleeUntil,
		Manhunter:         a.Manhunter,
		HuntSince:         a.HuntSince,
		ChargeSince:       a.ChargeSince,
		NextCharge:        a.NextCharge,
		ChaseWarnedAt:     a.ChaseWarnedAt,
		ChaseWarnedTarget: a.ChaseWarnedTarget,
		Feed:              a.Feed,
		FeedLeft:          a.FeedLeft,
		NextFeed:          a.NextFeed,
		FeedClaim:         a.FeedClaim,
		Designated:        a.Designated,
		TameMark:          a.TameMark,
		TameRetry:         a.TameRetry,
		NoFollow:          a.NoFollow,
		Master:            a.Master,
		Bond:              a.Bond,
		Trained:           a.Trained,
		TrainDecay:        a.TrainDecay,
		NextBondCheck:     a.NextBondCheck,
		ConfusedUntil:     a.ConfusedUntil,
		Pregnant:          a.Pregnant,
		Mated:             a.Mated,
		Product:           a.Product,
		Mate:              a.Mate,
		MateLeft:          a.MateLeft,
		MateUntil:         a.MateUntil,
		Mother:            a.Mother,
		Body:              a.Body,
	}
}

// AssignState updates each authoritative component in place.
func (a *Live) AssignState(s State) {
	a.Species = s.Species
	a.Tame = s.Tame
	a.Name = s.Name
	a.Female = s.Female
	a.HasSex = s.HasSex
	a.Born = s.Born
	a.Herd = s.Herd
	a.Injury = s.Injury
	a.Hunger = s.Hunger
	a.Food = s.Food
	a.Rest = s.Rest
	a.Starving = s.Starving
	a.Downed = s.Downed
	a.DownedTick = s.DownedTick
	a.Mode = s.Mode
	a.Target = s.Target
	a.NextThink = s.NextThink
	a.Cooldown = s.Cooldown
	a.NextDecide = s.NextDecide
	a.Leaving = s.Leaving
	a.ExitX = s.ExitX
	a.ExitY = s.ExitY
	a.Errand = s.Errand
	a.Dormant = s.Dormant
	a.Path = s.Path
	a.PathI = s.PathI
	a.GoalX = s.GoalX
	a.GoalY = s.GoalY
	a.RetryAt = s.RetryAt
	a.StepGait = s.StepGait
	a.Attacker = s.Attacker
	a.FleeUntil = s.FleeUntil
	a.Manhunter = s.Manhunter
	a.HuntSince = s.HuntSince
	a.ChargeSince = s.ChargeSince
	a.NextCharge = s.NextCharge
	a.ChaseWarnedAt = s.ChaseWarnedAt
	a.ChaseWarnedTarget = s.ChaseWarnedTarget
	a.Feed = s.Feed
	a.FeedLeft = s.FeedLeft
	a.NextFeed = s.NextFeed
	a.FeedClaim = s.FeedClaim
	a.Designated = s.Designated
	a.TameMark = s.TameMark
	a.TameRetry = s.TameRetry
	a.NoFollow = s.NoFollow
	a.Master = s.Master
	a.Bond = s.Bond
	a.Trained = s.Trained
	a.TrainDecay = s.TrainDecay
	a.NextBondCheck = s.NextBondCheck
	a.ConfusedUntil = s.ConfusedUntil
	a.Pregnant = s.Pregnant
	a.Mated = s.Mated
	a.Product = s.Product
	a.Mate = s.Mate
	a.MateLeft = s.MateLeft
	a.MateUntil = s.MateUntil
	a.Mother = s.Mother
	a.Body = s.Body
}

func (a *Live) Enraged(now uint64) bool { return a.Manhunter != 0 && now < a.Manhunter }

func (a *Live) HP() int {
	return max(0, DefOf(a.Species).MaxHP-a.Injury)
}

func (a *Live) Dead() bool { return a.HP() <= 0 }

func (a *Live) Moving() int32 {
	if a.Body == nil {
		return 1000
	}
	return a.Body.Capacity(body.Moving)
}

func (a *Live) bloodLoss() int32 {
	if a.Body == nil {
		return 0
	}
	return a.Body.BloodLoss
}

func (a *Live) DownsNow() bool {
	if a.Downed || a.Dead() || IsMech(a.Species) {
		return false
	}
	return a.Injury*100 >= DefOf(a.Species).MaxHP*DownInjuryPct ||
		a.bloodLoss() >= DownBloodLoss || a.Moving() < DownMoving ||
		a.Body != nil && a.Body.Capacity(body.Consciousness) < DownConsciousness
}

func (a *Live) StandsUp() bool {
	return a.Downed && a.Injury*100 <= DefOf(a.Species).MaxHP*UpInjuryPct &&
		a.bloodLoss() < UpBloodLoss && a.Moving() >= UpMoving &&
		(a.Body == nil || a.Body.Capacity(body.Consciousness) >= UpConsciousness)
}

func (a *Live) Hungry() bool {
	d := DefOf(a.Species)
	return d.Predator && (a.Hunger >= d.HungerTicks || a.HasSex && a.Food < FeedStart)
}

func (a *Live) Step() int {
	d := DefOf(a.Species)
	st := d.StepTicks
	if a.Mode == Flee && d.FleeStep > 0 {
		st = d.FleeStep
	}
	return a.slowedByBody(st)
}

func (a *Live) slowedByBody(st int) int {
	if a.Body != nil {
		if m := a.Moving(); m < 1000 {
			st = (st*1000 + int(max(m, DownMoving)) - 1) / int(max(m, DownMoving))
		}
	}
	return st
}

func (a *Live) StepCost(diagonal bool) int {
	st := a.Step()
	if diagonal {
		return (st*1414 + 500) / 1000
	}
	return st
}

func (a *Live) GaitAt(now uint64) Gait {
	switch {
	case a.Mode == Flee || a.Mode == Attack || a.Mode == Hunt || a.Enraged(now):
		return GaitRun
	case a.Errand || a.Leaving || a.Mode == Feed || a.Mode == Rest || a.Mode == Leave:
		return GaitWalk
	}
	return GaitAmble
}

func (a *Live) StepAt(g Gait) int {
	d := DefOf(a.Species)
	st := d.StepTicks
	switch g {
	case GaitNone:
		return a.Step()
	case GaitAmble:
		st = (st*ambleMilli + 500) / 1000
	case GaitRun:
		if a.Mode == Flee && d.FleeStep > 0 {
			st = d.FleeStep
		}
	}
	return a.slowedByBody(st)
}

func (a *Live) StepCostAt(g Gait, diagonal bool) int {
	st := a.StepAt(g)
	if diagonal {
		return (st*1414 + 500) / 1000
	}
	return st
}

func (a *Live) Normalise(id uint64) {
	state := a.State()
	state.Normalise(id)
	s := SnapshotFromState(state)
	*a.Identity, *a.Vitals = s.Identity, s.Vitals
}

func (a *Live) SnapshotRef() *State {
	if a == nil {
		return nil
	}
	state := a.State()
	return &state
}
