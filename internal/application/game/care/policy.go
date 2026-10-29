package care

import (
	"maps"
	"slices"

	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/entity"
)

// Medical care policy (MED-03). Every patient has a care level
// (medcare.Policy): one the player set by hand, else its group's default
// (colonists the best medicine, prisoners, guests and animals herbal or
// less; wiki: Doctoring). The level caps the medicine a tend fetches;
// "No care" keeps doctors away altogether, though a downed patient is
// still rescued to bed. Colonists tend themselves when no one else can,
// unless the player turned that off.

// Policy is the colony's care settings: per-patient levels set by hand
// (colonists, prisoners, guests and animals), the group defaults (custom
// false: medcare.Defaults) and the colonists who may not tend themselves.
// The zero value is every default.
type Policy struct {
	per        map[entity.EntityID]medcare.Policy
	def        [medcare.NumGroups]medcare.Policy
	custom     bool
	noSelfTend map[entity.EntityID]bool
}

// PolicyEntry is one patient's level in the save.
type PolicyEntry = Entry[uint8]

// PolicySave is the saved Policy, in id order. The save field is nil
// while everything is at its default, so older saves and their hashes are
// unchanged. Its field names are the game's pre-SIM-061 careSave.
type PolicySave struct {
	Per        []PolicyEntry
	Def        []uint8 // nil: the defaults
	NoSelfTend []entity.EntityID
}

// Own is the level the player set for id by hand, if any.
func (p *Policy) Own(id entity.EntityID) (medcare.Policy, bool) {
	l, ok := p.per[id]
	return l, ok
}

// Default is group grp's care level now.
func (p *Policy) Default(grp medcare.Group) medcare.Policy {
	if p.custom && grp < medcare.NumGroups {
		return p.def[grp]
	}
	return medcare.Default(grp)
}

// Defaults is every group's default care level, in medcare.Group order.
func (p *Policy) Defaults() [medcare.NumGroups]uint8 {
	var out [medcare.NumGroups]uint8
	for grp := medcare.Group(0); grp < medcare.NumGroups; grp++ {
		out[grp] = uint8(p.Default(grp))
	}
	return out
}

// SelfTendOff: the player turned colonist id's self-tend off.
func (p *Policy) SelfTendOff(id entity.EntityID) bool { return p.noSelfTend[id] }

// Set sets patient's level to mode; an invalid mode is ignored. The
// caller checks the patient is one.
func (p *Policy) Set(patient entity.EntityID, mode uint8) {
	l := medcare.Policy(mode)
	if !l.Valid() {
		return
	}
	if p.per == nil {
		p.per = map[entity.EntityID]medcare.Policy{}
	}
	p.per[patient] = l
}

// SetDefault sets the default level of group (0 colonists, 1 prisoners,
// 2 guests, 3 animals) to mode. Patients set by hand keep theirs.
func (p *Policy) SetDefault(group int, mode uint8) {
	l := medcare.Policy(mode)
	if !l.Valid() || group < 0 || group >= int(medcare.NumGroups) {
		return
	}
	if !p.custom {
		p.def, p.custom = medcare.Defaults(), true
	}
	p.def[group] = l
	if p.def == medcare.Defaults() {
		p.def, p.custom = [medcare.NumGroups]medcare.Policy{}, false
	}
}

// SetSelfTend lets colonist id tend itself (on) or not. The caller checks
// id is a colonist.
func (p *Policy) SetSelfTend(id entity.EntityID, on bool) {
	if on {
		delete(p.noSelfTend, id)
		return
	}
	if p.noSelfTend == nil {
		p.noSelfTend = map[entity.EntityID]bool{}
	}
	p.noSelfTend[id] = true
}

// Export is the save field (nil: everything at its default).
func (p *Policy) Export() *PolicySave {
	if len(p.per) == 0 && !p.custom && len(p.noSelfTend) == 0 {
		return nil
	}
	s := &PolicySave{}
	for _, id := range slices.Sorted(maps.Keys(p.per)) {
		s.Per = append(s.Per, PolicyEntry{id, uint8(p.per[id])})
	}
	if p.custom {
		s.Def = make([]uint8, medcare.NumGroups)
		for i, l := range p.def {
			s.Def[i] = uint8(l)
		}
	}
	s.NoSelfTend = slices.Sorted(maps.Keys(p.noSelfTend))
	return s
}

// Import restores Export's state (nil: an older save, all defaults).
// Out-of-range levels fall back to the default.
func (p *Policy) Import(s *PolicySave) {
	*p = Policy{}
	if s == nil {
		return
	}
	for _, e := range s.Per {
		p.Set(e.ID, e.V)
	}
	if len(s.Def) == int(medcare.NumGroups) {
		p.def, p.custom = medcare.Defaults(), true
		for i, v := range s.Def {
			if l := medcare.Policy(v); l.Valid() {
				p.def[i] = l
			}
		}
	}
	for _, id := range s.NoSelfTend {
		p.SetSelfTend(id, false)
	}
}
