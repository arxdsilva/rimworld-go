package sense

import "rimworld-go/internal/domain/sim/entity"

// JoyRoles is the game's side of TopOff and Relax; CSE-002 and CSE-008 fill it in.
type JoyRoles interface{}

// TopOff has a colonist below full recreation use its free hours (CSE-002).
type TopOff struct {
	s *Sense
	r JoyRoles
}

func newTopOff(s *Sense, r JoyRoles) TopOff { return TopOff{s: s, r: r} }

// Wants reports id, with this much recreation, wants recreation; false sends
// it to look for work first in a free hour.
func (t TopOff) Wants(id entity.EntityID, joy int) bool { return true }

// Relax has a stressed colonist take a drink (CSE-008).
type Relax struct {
	s *Sense
	r JoyRoles
}

func newRelax(s *Sense, r JoyRoles) Relax { return Relax{s: s, r: r} }

// Assign starts a relaxing drink; false: none wanted.
func (r Relax) Assign(id entity.EntityID) bool { return false }
