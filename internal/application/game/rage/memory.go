package rage

import "rimworld-go/internal/domain/sim/entity"

// Memory is the rage brain's saved memory (EAI-001..003). It lives in the game's wildlife state and saves with it; the
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

// hold records animal id battering b (EAI-001).
func (m *Memory) hold(id entity.EntityID, b Bash) {
	if m.Bash == nil {
		m.Bash = map[entity.EntityID]Bash{}
	}
	m.Bash[id] = b
}

// lock records animal id's chosen foe (EAI-002).
func (m *Memory) lock(id entity.EntityID, l Lock) {
	if m.Lock == nil {
		m.Lock = map[entity.EntityID]Lock{}
	}
	m.Lock[id] = l
}

// call records herd's call to arms (EAI-003), first dropping every call
// older than callEvery: deleting is the same whatever order the map gives.
func (m *Memory) call(herd uint32, c Call) {
	if m.Call == nil {
		m.Call = map[uint32]Call{}
	}
	for k, o := range m.Call {
		if c.At >= o.At+callEvery {
			delete(m.Call, k)
		}
	}
	m.Call[herd] = c
}
