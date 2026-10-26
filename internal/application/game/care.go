package game

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Medical care policy (MED-03). Every patient has a care level
// (medcare.Policy): one the player set by hand, else its group's default
// (colonists the best medicine, prisoners, guests and animals herbal or
// less; wiki: Doctoring). The level caps the medicine a tend fetches
// (withMedicine, infection.go); "No care" keeps doctors away altogether,
// though a downed patient is still rescued to bed. Colonists tend
// themselves when no one else can, unless the player turned that off.
// A drafted colonist can be sent to tend someone at once (EmergencyTend).

// emergencyMedReach is how far (pathing.Dist tiles) a drafted colonist
// looks for medicine on an emergency tend before tending without
// (original tuning).
const emergencyMedReach = 15

// careState is the colony's care settings: per-patient levels set by
// hand, the group defaults (custom false: medcare.Defaults) and the
// colonists who may not tend themselves. The zero value is every default.
type careState struct {
	per        map[entity.EntityID]medcare.Policy
	def        [medcare.NumGroups]medcare.Policy
	custom     bool
	noSelfTend map[entity.EntityID]bool
}

// careSave is the saved careState, in id order. The save field is nil
// while everything is at its default, so older saves and their hashes
// are unchanged.
type careSave struct {
	Per        []entry[uint8]
	Def        []uint8 // nil: the defaults
	NoSelfTend []entity.EntityID
}

// careGroup is the default group patient id belongs to.
func (g *Game) careGroup(id entity.EntityID) medcare.Group {
	switch {
	case g.isAnimal(id):
		return medcare.Animals
	case g.isPrisoner(id):
		return medcare.Prisoners
	case g.visitor(id):
		return medcare.Guests
	}
	return medcare.Colonists
}

// defaultCare is group grp's care level now.
func (g *Game) defaultCare(grp medcare.Group) medcare.Policy {
	if g.care.custom && grp < medcare.NumGroups {
		return g.care.def[grp]
	}
	return medcare.Default(grp)
}

// careOf is patient id's care level: its own, else its group's default.
func (g *Game) careOf(id entity.EntityID) medcare.Policy {
	if p, ok := g.care.per[id]; ok {
		return p
	}
	return g.defaultCare(g.careGroup(id))
}

// selfTendAllowed: colonist id may tend itself (the player did not turn
// it off, and its care level allows a tend at all).
func (g *Game) selfTendAllowed(id entity.EntityID) bool {
	return !g.care.noSelfTend[id] && g.careOf(id).Tends()
}

// cmdSetMedicalCare sets patient's care level to mode (a medcare.Policy);
// anything else, or someone who is no patient at all, is ignored.
func (g *Game) cmdSetMedicalCare(patient entity.EntityID, mode uint8) {
	p := medcare.Policy(mode)
	if !p.Valid() || !g.isPawn(patient) && !g.isAnimal(patient) {
		return
	}
	if g.care.per == nil {
		g.care.per = map[entity.EntityID]medcare.Policy{}
	}
	g.care.per[patient] = p
}

// cmdSetDefaultCare sets the default care level of group (0 colonists,
// 1 prisoners, 2 guests, 3 animals) to mode. Patients set by hand keep
// theirs.
func (g *Game) cmdSetDefaultCare(group int, mode uint8) {
	p := medcare.Policy(mode)
	if !p.Valid() || group < 0 || group >= int(medcare.NumGroups) {
		return
	}
	if !g.care.custom {
		g.care.def, g.care.custom = medcare.Defaults(), true
	}
	g.care.def[group] = p
	if g.care.def == medcare.Defaults() {
		g.care.def, g.care.custom = [medcare.NumGroups]medcare.Policy{}, false
	}
}

// cmdSetSelfTend lets colonist id tend itself (on) or not.
func (g *Game) cmdSetSelfTend(id entity.EntityID, on bool) {
	if !g.colonist(id) {
		return
	}
	if on {
		delete(g.care.noSelfTend, id)
		return
	}
	if g.care.noSelfTend == nil {
		g.care.noSelfTend = map[entity.EntityID]bool{}
	}
	g.care.noSelfTend[id] = true
}

// canEmergencyTend reports why drafted colonist doctor cannot tend
// patient now ("" when it can).
func (g *Game) canEmergencyTend(doctor, patient entity.EntityID) string {
	switch {
	case !g.colonist(doctor) || !g.alive(doctor) || !g.drafted(doctor) || g.breaking(doctor):
		return ReasonIncapable
	case g.incapableWork(doctor).Enabled(work.Doctor):
		return ReasonIncapable
	case !g.needsTending(patient):
		return ReasonNothingToTend
	case doctor == patient && !g.selfTendAllowed(doctor):
		return ReasonNoSelfTend
	}
	if owner, taken := g.reserve.EntityOwner(patient); taken && owner != doctor {
		return ReasonBusyTarget + g.name(owner)
	}
	pp, _ := g.positions.Get(patient)
	if _, _, ok := g.besideSpot(doctor, pp.X, pp.Y); !ok {
		return ReasonUnreachable
	}
	return ""
}

// Emergency-tend reasons (player-facing).
const (
	ReasonNothingToTend = "nothing to tend"
	ReasonNoSelfTend    = "self-tend is off"
)

// cmdEmergencyTend sends drafted doctor to tend patient now: it fetches
// the best medicine the patient's care level allows within
// emergencyMedReach (none when there is none) and tends beside it. It
// stays drafted; undrafting, or the patient healing, dying or leaving,
// ends the job (doEmergencyTend).
func (g *Game) cmdEmergencyTend(doctor, patient entity.EntityID) {
	if g.canEmergencyTend(doctor, patient) != "" {
		return
	}
	pp, _ := g.positions.Get(patient)
	sx, sy, _ := g.besideSpot(doctor, pp.X, pp.Y)
	wl := TendWork
	if doctor == patient {
		wl = TendWork * 3 / 2
	}
	delete(g.orderQueue, doctor) // a plain order replaces the queue
	g.interrupt(doctor, job.Cancelled)
	rec := g.emergencyMedicine(doctor, job.Record{Kind: job.EmergencyTend, Target: patient, TX: sx, TY: sy,
		WorkLeft: wl, Forced: true})
	if g.startJob(doctor, rec) && rec.Dest != 0 {
		g.reserve.TryEntity(doctor, rec.Dest)
	}
}

// emergencyMedicine adds a fetch of the best medicine rec.Target's care
// level allows to rec, if there is some within emergencyMedReach of
// doctor; withMedicine's route (medicine first, then the patient).
func (g *Game) emergencyMedicine(doctor entity.EntityID, rec job.Record) job.Record {
	pol := g.careOf(rec.Target)
	pos, _ := g.positions.Get(doctor)
	for _, k := range [...]item.Kind{item.IndustrialMedicine, item.HerbalMedicine} {
		if !medcare.Allows(pol, k) {
			continue
		}
		sid, st := g.ingredientFor(doctor, recipe.Input{Kind: k, Qty: 1, Max: 1})
		if sid == 0 || pathing.Dist(pos.X, pos.Y, st.X, st.Y) > emergencyMedReach {
			continue
		}
		if _, taken := g.reserve.EntityOwner(sid); taken {
			continue
		}
		rec.Dest, rec.DestX, rec.DestY = sid, rec.TX, rec.TY
		rec.TX, rec.TY = st.X, st.Y
		return rec
	}
	return rec
}

// doEmergencyTend runs an emergency tend like any tend (doTend), while
// the doctor stays drafted; undrafted (setDraft interrupts it anyway) or
// downed, it ends.
func (g *Game) doEmergencyTend(id entity.EntityID, j job.Record) {
	if v, _ := g.vital.Get(id); !g.drafted(id) || v.Status != health.Alive {
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	g.doTend(id, j)
}

// exportCare is the Care save field (nil: everything at its default).
func (g *Game) exportCare() *careSave {
	c := &g.care
	if len(c.per) == 0 && !c.custom && len(c.noSelfTend) == 0 {
		return nil
	}
	s := &careSave{}
	for id, p := range c.per {
		s.Per = append(s.Per, entry[uint8]{id, uint8(p)})
	}
	slices.SortFunc(s.Per, func(a, b entry[uint8]) int { return int(a.ID) - int(b.ID) })
	if c.custom {
		s.Def = make([]uint8, medcare.NumGroups)
		for i, p := range c.def {
			s.Def[i] = uint8(p)
		}
	}
	for id := range c.noSelfTend {
		s.NoSelfTend = append(s.NoSelfTend, id)
	}
	slices.Sort(s.NoSelfTend)
	return s
}

// importCare restores exportCare's state (nil: an older save, all
// defaults). Out-of-range levels fall back to the default.
func (g *Game) importCare(s *careSave) {
	g.care = careState{}
	if s == nil {
		return
	}
	for _, e := range s.Per {
		if p := medcare.Policy(e.V); p.Valid() {
			if g.care.per == nil {
				g.care.per = map[entity.EntityID]medcare.Policy{}
			}
			g.care.per[e.ID] = p
		}
	}
	if len(s.Def) == int(medcare.NumGroups) {
		g.care.def, g.care.custom = medcare.Defaults(), true
		for i, v := range s.Def {
			if p := medcare.Policy(v); p.Valid() {
				g.care.def[i] = p
			}
		}
	}
	for _, id := range s.NoSelfTend {
		if g.care.noSelfTend == nil {
			g.care.noSelfTend = map[entity.EntityID]bool{}
		}
		g.care.noSelfTend[id] = true
	}
}

// DefaultCare is every group's default care level (medcare.Policy), in
// medcare.Group order (colonists, prisoners, guests, animals). Read-only.
func (g *Game) DefaultCare() [medcare.NumGroups]uint8 {
	var out [medcare.NumGroups]uint8
	for grp := medcare.Group(0); grp < medcare.NumGroups; grp++ {
		out[grp] = uint8(g.defaultCare(grp))
	}
	return out
}

// tendOrders are drafted pawn's "Tend <name>" entries for the wounded
// colonists and prisoners lying or standing on (x, y), each disabled with
// the reason it can't happen (canEmergencyTend). None when not drafted.
func (g *Game) tendOrders(pawn entity.EntityID, x, y int) []OrderOption {
	if !g.drafted(pawn) {
		return nil
	}
	var out []OrderOption
	add := func(pid entity.EntityID) {
		if p, ok := g.positions.Get(pid); !ok || p.X != x || p.Y != y || g.carriedBy(pid) != 0 || !g.needsTending(pid) {
			return
		}
		o := OrderOption{Label: "Tend " + g.name(pid),
			Cmd: command.Command{Kind: command.EmergencyTend, Pawn: uint64(pawn), Target: uint64(pid)}}
		if why := g.canEmergencyTend(pawn, pid); why != "" {
			o.Disabled, o.Reason = true, why
		}
		out = append(out, o)
	}
	for _, pid := range g.pawnIDs {
		add(pid)
	}
	for _, pid := range g.raiderIDs {
		if g.isPrisoner(pid) {
			add(pid)
		}
	}
	return out
}
