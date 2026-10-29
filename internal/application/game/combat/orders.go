package combat

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Orders are the defence commands (DEF-3: SetDoorHoldOpen, SetTurretHold,
// SetMortarTarget, ClearMortarTarget, each on the building at X, Y; the
// game routes them here from its command switch) and the defences' facts
// for the game's building read model.
type Orders struct {
	h       Host
	turrets *Turrets
	mortars *Mortars
	doors   *Doors
	rearms  *Rearms
}

// Do is colonist id's defence job j in its Do state (the game's job
// phase): a rearm haul advances (Rearms.Do); a mortar's crew stands at
// its post, which the mortar itself reads (Mortars), so the job just
// stays.
func (o *Orders) Do(id entity.EntityID, j job.Record) {
	if j.Kind == job.RearmTurret {
		o.rearms.Do(id, j)
	}
}

// Apply carries out defence command c; a command on the wrong kind of
// building (or none) does nothing. It reports a door's hold changed: the
// hostiles' way changed with it (the game rebuilds its path caches).
func (o *Orders) Apply(c command.Command) (way bool) {
	id, k := o.h.Object(c.X, c.Y)
	if id == 0 {
		return false
	}
	switch {
	case c.Kind == command.SetDoorHoldOpen && k.IsDoor():
		way = o.doors.Held(id) != c.Enabled
		o.doors.SetHold(id, c.Enabled)
	case c.Kind == command.SetTurretHold && isTurret(k):
		o.turrets.SetHold(id, c.Enabled)
	case c.Kind == command.SetMortarTarget && k == construction.Mortar:
		o.mortars.SetTarget(id, c.X2, c.Y2, entity.EntityID(c.Pawn))
	case c.Kind == command.ClearMortarTarget && k == construction.Mortar:
		o.mortars.ClearTarget(id)
	}
	return way
}

// Facts are a defence's read-model fields (the game's BuildingView): a
// turret's aim frame and shot age (ShotAge 0 for anything else), a door
// held open, a turret holding fire, barrel life (permille, 0: no barrel),
// shells loaded and a mortar's forced target.
type Facts struct {
	Dir                              int8
	ShotAge                          int
	HoldOpen, HoldFire, HasTarget    bool
	Barrel, Shells, TargetX, TargetY int
}

// View is building id's (built as b) Facts at now.
func (o *Orders) View(id entity.EntityID, b construction.Building, now uint64) Facts {
	var f Facts
	switch {
	case isTurret(b.Kind):
		f.Dir, f.ShotAge = o.turrets.View(id, now)
		f.HoldFire, f.Barrel = o.turrets.Held(id), o.turrets.Barrel(id, b.Kind)
	case b.Kind == construction.Mortar:
		st := o.mortars.State(id)
		f.Barrel, f.Shells = o.mortars.Barrel(id), st.shells()
		f.HasTarget, f.TargetX, f.TargetY = st.Forced, st.TX, st.TY
	case b.Kind.IsDoor():
		f.HoldOpen = o.doors.Held(id)
	}
	return f
}
