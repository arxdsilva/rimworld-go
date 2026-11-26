// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
)

// Snapshot is a grouped animal record for persistence and ownership
// migration. State remains the current live representation while consumers
// move to component-owned state.
type Snapshot struct {
	Identity     Identity
	Vitals       Vitals
	Intent       Intent
	Motion       Motion
	Threat       Threat
	Feeding      Feeding
	Handling     Handling
	Reproduction Reproduction
	Medical      Medical
}

// Identity is the animal's origin, species and colony ownership.
type Identity struct {
	Species Species
	Tame    bool
	Name    string
	Female  bool
	HasSex  bool
	Born    uint64
	Herd    uint32
}

type Vitals struct {
	Injury     int
	Hunger     uint64
	Food       int
	Rest       int
	Starving   uint64
	Downed     bool
	DownedTick uint64
}

type Intent struct {
	Mode       Mode
	Target     entity.EntityID
	NextThink  uint64
	Cooldown   int
	NextDecide uint64
	Leaving    bool
	ExitX      int
	ExitY      int
	Errand     bool
	Dormant    bool
}

type Motion struct {
	Path     []pathing.Point
	PathI    int
	GoalX    int
	GoalY    int
	RetryAt  uint64
	StepGait Gait
}

type Threat struct {
	Attacker          entity.EntityID
	FleeUntil         uint64
	Manhunter         uint64
	HuntSince         uint64
	ChargeSince       uint64
	NextCharge        uint64
	ChaseWarnedAt     uint64
	ChaseWarnedTarget entity.EntityID
}

type Feeding struct {
	Feed      FeedTarget
	FeedLeft  int
	NextFeed  uint64
	FeedClaim entity.EntityID
}

type Handling struct {
	Designated    bool
	TameMark      bool
	TameRetry     uint64
	NoFollow      bool
	Master        uint64
	Bond          uint64
	Trained       [training.LegacySkills]uint8
	TrainDecay    uint64
	NextBondCheck uint64
	ConfusedUntil uint64
}

type Reproduction struct {
	Pregnant  uint64
	Mated     int
	Product   int
	Mate      entity.EntityID
	MateLeft  int
	MateUntil uint64
	Mother    uint64
}

type Medical struct {
	Body *body.Body
}
