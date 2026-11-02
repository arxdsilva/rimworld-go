package travel

import (
	"cmp"
	"maps"
	"slices"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// State is travel's state. The zero State has no sleeper, nothing loaded,
// nothing in flight and no site found; Save is its saved form.
type State struct {
	caskets map[entity.EntityID]entity.EntityID // stasis casket -> its sleeper
	holds   map[entity.EntityID]*Hold           // transport pod -> what it holds
	wants   map[entity.EntityID]*Manifest       // pod group (its lowest pod) -> still to load
	roped   map[entity.EntityID]Beast           // hauler -> the animal it leads to a pod
	cargo   map[uint64]*Hold                    // caravan -> the goods and animals pods set down with it
	hitch   map[uint64]*Gather                  // forming caravan -> its hitching spot
	scans   map[entity.EntityID]Scan            // mineral scanner -> its work and target
	flights []Flight
	sites   []Site
	finds   int // mineral finds so far (each hashes the next)
	nextID  uint64
	groups  groupCache // derived (per structure version)
}

// Hold is what a pod (or a landed caravan's cargo) carries: passengers
// (stashed pawns), animals lifted off the map, and goods.
type Hold struct {
	Pawns   []entity.EntityID
	Animals []Beast
	Items   []item.Stack
}

// Beast is an animal off the map, its state kept whole.
type Beast struct {
	ID    entity.EntityID
	State animal.State
}

// Manifest is what is still to go into a pod group: pawns to board,
// animals to lead in, goods by kind and amount.
type Manifest struct {
	Pawns   []entity.EntityID
	Animals []entity.EntityID
	Items   []Want
}

// Want is an amount of one kind of goods still to load.
type Want struct {
	Kind item.Kind
	Qty  int
}

// Flight is a pod group in the air.
type Flight struct {
	ID             uint64
	From, To       int
	Depart, Arrive uint64
	Mode           uint8 // command.PodArrive*
	Pods           int
	Load           Hold
}

// Site is a mineral lump a scanner found on the world map.
type Site struct {
	Tile    int
	Mineral item.Kind
	Qty     int
	Expires uint64
}

// Gather is a forming caravan's hitching spot: where its members wait,
// and since when all of them have been there (0: not yet).
type Gather struct {
	X, Y  int
	Since uint64
}

// Scan is one mineral scanner's progress and target (0: any mineral).
type Scan struct {
	Work int
	For  item.Kind
}

// Save is State's saved form (the world save's AX9 block; absent in
// older saves).
type Save struct {
	Caskets []Pair
	Holds   []PodHold
	Wants   []PodWant
	Roped   []RopedBeast
	Cargo   []CaravanCargo
	Hitch   []CaravanSpot
	Scans   []ScanEntry
	Flights []Flight
	Sites   []Site
	Finds   int
	NextID  uint64
}

// Pair is a casket and its sleeper.
type Pair struct{ Casket, Sleeper entity.EntityID }

// PodHold is one pod's load.
type PodHold struct {
	Pod  entity.EntityID
	Hold Hold
}

// PodWant is one pod group's manifest.
type PodWant struct {
	Pod  entity.EntityID
	Want Manifest
}

// RopedBeast is an animal a hauler leads.
type RopedBeast struct {
	Hauler entity.EntityID
	Beast  Beast
}

// CaravanCargo is one caravan's cargo.
type CaravanCargo struct {
	Caravan uint64
	Hold    Hold
}

// CaravanSpot is a forming caravan's hitching spot.
type CaravanSpot struct {
	Caravan uint64
	Spot    Gather
}

// ScanEntry is one scanner's progress.
type ScanEntry struct {
	ID   entity.EntityID
	Scan Scan
}

// Empty reports nothing worth saving (nil-safe).
func (s *State) Empty() bool {
	return s == nil || len(s.caskets) == 0 && len(s.holds) == 0 && len(s.wants) == 0 && len(s.roped) == 0 &&
		len(s.cargo) == 0 && len(s.hitch) == 0 && len(s.scans) == 0 && len(s.flights) == 0 && len(s.sites) == 0 &&
		s.finds == 0 && s.nextID == 0
}

// sortedKeys is m's keys, ascending.
func sortedKeys[K cmp.Ordered, V any](m map[K]V) []K { return slices.Sorted(maps.Keys(m)) }

// clone is a deep copy of h.
func (h Hold) clone() Hold {
	return Hold{Pawns: slices.Clone(h.Pawns), Animals: slices.Clone(h.Animals), Items: slices.Clone(h.Items)}
}

func (m Manifest) clone() Manifest {
	return Manifest{Pawns: slices.Clone(m.Pawns), Animals: slices.Clone(m.Animals), Items: slices.Clone(m.Items)}
}

// Export is the saved form (nil when there is nothing to save).
func (s *State) Export() *Save {
	if s.Empty() {
		return nil
	}
	out := &Save{Finds: s.finds, NextID: s.nextID, Sites: slices.Clone(s.sites)}
	for _, k := range sortedKeys(s.caskets) {
		out.Caskets = append(out.Caskets, Pair{k, s.caskets[k]})
	}
	for _, k := range sortedKeys(s.holds) {
		out.Holds = append(out.Holds, PodHold{k, s.holds[k].clone()})
	}
	for _, k := range sortedKeys(s.wants) {
		out.Wants = append(out.Wants, PodWant{k, s.wants[k].clone()})
	}
	for _, k := range sortedKeys(s.roped) {
		out.Roped = append(out.Roped, RopedBeast{k, s.roped[k]})
	}
	for _, k := range sortedKeys(s.cargo) {
		out.Cargo = append(out.Cargo, CaravanCargo{k, s.cargo[k].clone()})
	}
	for _, k := range sortedKeys(s.hitch) {
		out.Hitch = append(out.Hitch, CaravanSpot{k, *s.hitch[k]})
	}
	for _, k := range sortedKeys(s.scans) {
		out.Scans = append(out.Scans, ScanEntry{k, s.scans[k]})
	}
	for _, f := range s.flights {
		f.Load = f.Load.clone()
		out.Flights = append(out.Flights, f)
	}
	return out
}

// Import is a State restored from v (nil: a fresh one).
func Import(v *Save) *State {
	s := &State{}
	if v == nil {
		return s
	}
	s.finds, s.nextID, s.sites = v.Finds, v.NextID, slices.Clone(v.Sites)
	for _, p := range v.Caskets {
		s.setSleeper(p.Casket, p.Sleeper)
	}
	for _, p := range v.Holds {
		h := p.Hold.clone()
		s.holdOf(p.Pod)
		s.holds[p.Pod] = &h
	}
	for _, p := range v.Wants {
		m := p.Want.clone()
		if s.wants == nil {
			s.wants = map[entity.EntityID]*Manifest{}
		}
		s.wants[p.Pod] = &m
	}
	for _, r := range v.Roped {
		s.rope(r.Hauler, r.Beast)
	}
	for _, c := range v.Cargo {
		h := c.Hold.clone()
		s.cargoOf(c.Caravan)
		s.cargo[c.Caravan] = &h
	}
	for _, c := range v.Hitch {
		g := c.Spot
		if s.hitch == nil {
			s.hitch = map[uint64]*Gather{}
		}
		s.hitch[c.Caravan] = &g
	}
	for _, e := range v.Scans {
		s.setScan(e.ID, e.Scan)
	}
	for _, f := range v.Flights {
		f.Load = f.Load.clone()
		s.flights = append(s.flights, f)
	}
	return s
}

func (s *State) setSleeper(casket, sleeper entity.EntityID) {
	if s.caskets == nil {
		s.caskets = map[entity.EntityID]entity.EntityID{}
	}
	s.caskets[casket] = sleeper
}

// holdOf is pod's hold, created empty on first use.
func (s *State) holdOf(pod entity.EntityID) *Hold {
	if s.holds == nil {
		s.holds = map[entity.EntityID]*Hold{}
	}
	h := s.holds[pod]
	if h == nil {
		h = &Hold{}
		s.holds[pod] = h
	}
	return h
}

// cargoOf is caravan id's cargo, created empty on first use.
func (s *State) cargoOf(id uint64) *Hold {
	if s.cargo == nil {
		s.cargo = map[uint64]*Hold{}
	}
	h := s.cargo[id]
	if h == nil {
		h = &Hold{}
		s.cargo[id] = h
	}
	return h
}

func (s *State) rope(hauler entity.EntityID, b Beast) {
	if s.roped == nil {
		s.roped = map[entity.EntityID]Beast{}
	}
	s.roped[hauler] = b
}

func (s *State) setScan(id entity.EntityID, sc Scan) {
	if s.scans == nil {
		s.scans = map[entity.EntityID]Scan{}
	}
	s.scans[id] = sc
}

// Asleep reports pawn id lying in a stasis casket (nil-safe): its
// illness is suspended (the game keeps it out of the patients index).
func (s *State) Asleep(id entity.EntityID) bool {
	if s == nil {
		return false
	}
	for _, p := range s.caskets {
		if p == id {
			return true
		}
	}
	return false
}

// core is a State and its host: the shared helpers (host.go) and the
// base of each concern's operations.
type core struct {
	s *State
	h Host
}

// Ops is travel over a State and its host: the facade the game calls
// (apply.go); each concern's operations hang off their own type.
type Ops struct{ core }

// caskets, pods, scanner and hitching are the stasis caskets', the
// transport pods', the mineral scanner's and the hitching spots'
// operations (casket.go; pods.go, load.go, launch.go; scanner.go;
// hitch.go).
type (
	caskets  struct{ core }
	pods     struct{ core }
	scanner  struct{ core }
	hitching struct{ core }
)

// Ops is travel's operations on s.
func (s *State) Ops(h Host) Ops { return Ops{core{s, h}} }

// massKg is a hold's mass in whole kilograms (rounded up).
func (h *Hold) massKg(animalMass func(animal.Species) int) int {
	g := 0
	for _, st := range h.Items {
		g += st.Qty * st.Kind.MassGrams()
	}
	kg := len(h.Pawns)*humanKg + (g+999)/1000
	for _, b := range h.Animals {
		kg += animalMass(b.State.Species)
	}
	return kg
}

// addItem puts st into h, merged into a like stack.
func (h *Hold) addItem(st item.Stack) {
	st.X, st.Y, st.Carrier = 0, 0, 0
	for i := range h.Items {
		if item.SameLoad(h.Items[i], st) {
			h.Items[i].Qty += st.Qty
			return
		}
	}
	h.Items = append(h.Items, st)
}

// empty reports nothing aboard.
func (h *Hold) empty() bool {
	return h == nil || len(h.Pawns) == 0 && len(h.Animals) == 0 && len(h.Items) == 0
}

// merge moves everything in o into h.
func (h *Hold) merge(o *Hold) {
	if o == nil {
		return
	}
	h.Pawns = append(h.Pawns, o.Pawns...)
	h.Animals = append(h.Animals, o.Animals...)
	for _, st := range o.Items {
		h.addItem(st)
	}
}

// value is the hold's market value (goods and animals; a gift's worth).
func (h *Hold) value() int {
	v := 0
	for _, st := range h.Items {
		v += st.Qty * st.Kind.Value()
	}
	for _, b := range h.Animals {
		v += animalValue(b.State.Species)
	}
	return v
}

// Of is *p, made on first use (the game keeps the state behind a nil-able
// pointer in its world state).
func Of(p **State) *State {
	if *p == nil {
		*p = &State{}
	}
	return *p
}
