package rage

import "rimworld-go/internal/domain/sim/entity"

// Memory is the rage brain's saved memory (EAI-0 skeleton: nothing writes
// it yet). It lives in the game's wildlife state and saves with it; the
// maps are exported so gob writes them and the state hash reads them in
// key order. The zero value is empty.
//
//   - Bash: animals battering a door or wall to reach a foe.
//   - Lock: an enraged animal's chosen foe, kept until Until.
//   - Call: a herd's last call to arms, which its members answer.
type Memory struct {
	Bash map[entity.EntityID]Bash
	Lock map[entity.EntityID]Lock
	Call map[uint32]Call
}

// Bash is an animal battering the cell (X, Y) since tick Since.
type Bash struct {
	X, Y  int
	Since uint64
}

// Lock is an animal's chosen foe, held until tick Until.
type Lock struct {
	Foe   entity.EntityID
	Until uint64
}

// Call is a herd's call to arms against Foe, raised at (X, Y) on tick At.
type Call struct {
	Foe  entity.EntityID
	X, Y int
	At   uint64
}

// Forget drops animal id's entries (it died, left or was tamed).
func (m *Memory) Forget(id entity.EntityID) {
	delete(m.Bash, id)
	delete(m.Lock, id)
}
