// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

// journey_care_helpers_test.go: fixtures for the pawn-to-pawn order journeys
// (SIM-062): beds, downed pawns, the float menu's rows and a Shift choice.

import (
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// careBed builds a finished bed of kind ("hospital bed", "sleeping spot")
// at (x, y) and returns its id.
func careBed(j *journey, kind string, x, y int) entity.EntityID {
	j.t.Helper()
	if !j.g.BuildNow(kind, "wood", "south", x, y) {
		j.t.Fatalf("could not build a %s at (%d, %d)", kind, x, y)
	}
	for _, b := range j.world().Buildings {
		if b.Kind == kind && b.X == x && b.Y == y {
			return b.ID
		}
	}
	j.t.Fatalf("no %s at (%d, %d) in the read model", kind, x, y)
	return 0
}

// carePrisonBed provides a reachable enclosed room, then marks its bed for prisoners.
func carePrisonBed(j *journey, bed entity.EntityID) {
	j.t.Helper()
	careEnclosePrisonBed(j, bed)
	careMarkPrisonBed(j, bed)
}

func careEnclosePrisonBed(j *journey, bed entity.EntityID) {
	j.t.Helper()
	for _, b := range j.world().Buildings {
		if b.ID != bed {
			continue
		}
		wcRoom(j, b.X-2, b.Y-1, b.X+2, b.Y+2, b.X, b.Y+2)
		if _, ok := j.g.RoomInfo(b.X, b.Y); !ok {
			j.t.Fatal("fixture: the prisoner bed must be enclosed")
		}
		return
	}
	j.t.Fatalf("fixture: missing prison bed %d", bed)
}

// careMarkPrisonBed also permits the deliberately invalid outdoor fixture.
func careMarkPrisonBed(j *journey, bed entity.EntityID) {
	j.t.Helper()
	j.g.Enqueue(command.Command{Kind: command.SetBedForPrisoners, Target: uint64(bed), Enabled: true})
	j.g.ApplyPending()
	if !j.g.BedForPrisoners(bed) {
		j.t.Fatal("the bed did not become a prisoner bed")
	}
}

// careRefusedPrisonOrder checks the live UI row and clicks it: no command
// may leave the client when the only designated prison bed is outdoors.
func careRefusedPrisonOrder(j *journey, label string) {
	j.t.Helper()
	for _, b := range j.a.ord.FloatButtons() {
		if !strings.HasPrefix(b.Label, label) {
			continue
		}
		if !b.Disabled || b.Reason != care.ReasonPrisonRoom || !strings.Contains(b.Label, care.ReasonPrisonRoom) {
			j.t.Fatalf("outdoor prison order not refused with reason: %+v", b)
		}
		before := len(j.cmds)
		j.pick(label)
		if len(j.cmds) != before {
			j.t.Fatalf("disabled prison order dispatched: %+v", j.cmds[before:])
		}
		j.a.ord.CloseFloat()
		return
	}
	j.t.Fatalf("missing prison order %q", label)
}

// careDown injures id until it is downed.
func careDown(j *journey, id uint64) {
	j.t.Helper()
	for dmg := 20; dmg <= 400 && j.pawn(id).Status != "downed"; dmg += 20 {
		j.g.Injure(entity.EntityID(id), dmg)
	}
	if st := j.pawn(id).Status; st != "downed" {
		j.t.Fatalf("could not down pawn %d (status %q)", id, st)
	}
}

// careRightClick opens the float menu on (x, y) with the camera on it and
// returns the row labels.
func careRightClick(j *journey, x, y int) []string {
	j.t.Helper()
	j.lookAt(x, y)
	j.clickTile(x, y, inputframe.Right, 0)
	var labels []string
	for _, b := range j.a.ord.FloatButtons() {
		labels = append(labels, b.Label)
	}
	return labels
}

// carePickShift chooses the row starting label with Shift held.
func carePickShift(j *journey, label string) {
	j.t.Helper()
	var labels []string
	for _, b := range j.a.ord.FloatButtons() {
		labels = append(labels, b.Label)
		if strings.HasPrefix(b.Label, label) {
			s := float32(j.a.scale.S)
			j.f.Cur.Mx, j.f.Cur.My = int((b.X+b.W/2)*s), int((b.Y+b.H/2)*s)
			j.f.Click(j.t, j.a, inputframe.Left, input.ModShift)
			j.f.Cur.Mx, j.f.Cur.My = -50, -50
			return
		}
	}
	j.t.Fatalf("no menu row starting %q; rows: %q", label, labels)
}

// careRun runs the colony at speed key (Digit1..3) until cond holds.
func careRun(j *journey, key string, max int, cond func() bool) {
	j.t.Helper()
	j.key(key, 0)
	defer j.key("Space", 0)
	j.until(cond, max)
}

// careFeed keeps every colonist fed and rested.
func careFeed(j *journey) {
	for _, p := range j.g.Snapshot() {
		if !p.Hostile {
			j.g.SetNeeds(p.ID, needs.Full, needs.Full)
		}
	}
}
