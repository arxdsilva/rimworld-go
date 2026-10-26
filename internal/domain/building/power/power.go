package power

import "rimworld-go/internal/domain/building/construction"

// WTicksPerWh is how many W·ticks make one Wh (one game hour of 1 W:
// clock.TicksPerHour).
const WTicksPerWh = 2500

// ConnectRange is how far (Chebyshev, in tiles, from its footprint) a
// device reaches for a transmitter to join.
const ConnectRange = 3

// Role is what a building does on a power grid.
type Role uint8

const (
	None        Role = iota // not electrical
	Producer                // makes power (generators, solar, wind, geothermal)
	Consumer                // draws power (lamps, heaters, coolers, electric benches)
	Storage                 // stores power (batteries)
	Transmitter             // carries power between neighbours (conduits)
	Switch                  // a transmitter while closed; open, it cuts the grid
)

// String is the role's view name ("" for None).
func (r Role) String() string {
	switch r {
	case Producer:
		return "producer"
	case Consumer:
		return "consumer"
	case Storage:
		return "storage"
	case Transmitter:
		return "transmitter"
	case Switch:
		return "switch"
	}
	return ""
}

// Class is a consumer's place in the shed order of a brownout: the lowest
// class is switched off first (lamps, then benches, coolers, heaters).
type Class uint8

const (
	ClassNone Class = iota // not a consumer
	ClassLamp
	ClassBench
	ClassCooler
	ClassHeater
)

// Spec is a building kind's electrical definition (our numbers). Draw is
// the W a consumer uses while working, Idle while idle; Peak the most W a
// producer makes; CapWh what a battery holds. Toggle: the player can
// switch it off; Hidden: a buried conduit (drawn only in the overlay,
// never chosen by a short circuit).
type Spec struct {
	Role                    Role
	Class                   Class
	Draw, Idle, Peak, CapWh int
	Toggle, Hidden          bool
}

// SpecOf is kind k's electrical definition (the zero Spec, Role None, for
// a kind that is not electrical).
func SpecOf(k construction.Kind) Spec {
	return Spec{} // TODO(POWERDOM): the PLAY-021 table (PLAY021_PLAN.md Model and Appendix)
}

// Node is one electrical building for Build: its id (an entity id), role,
// footprint cells and, for a switch, whether it is closed (conducting).
type Node struct {
	ID     uint64
	Role   Role
	Cells  [][2]int
	Closed bool
}

// Link is the transmitter a device joins: its node id and the transmitter
// cell the link line is drawn to.
type Link struct {
	To   uint64
	X, Y int
}

// Graph is the topology Build makes: the net every node is on and the
// transmitter every device links to. Read only.
type Graph struct{} // TODO(POWERDOM): union-find result

// Net is node id's net: the smallest transmitter id on it; 0 when id is
// on no net (a device with no transmitter in range).
func (g *Graph) Net(id uint64) uint64 {
	return 0 // TODO(POWERDOM)
}

// Link is the transmitter device id joins (ok false for a transmitter, a
// switch or a device with none in range).
func (g *Graph) Link(id uint64) (Link, bool) {
	return Link{}, false // TODO(POWERDOM)
}

// Nets lists every net id, ascending.
func (g *Graph) Nets() []uint64 {
	return nil // TODO(POWERDOM)
}

// Builder builds graphs, keeping its scratch between builds. The zero
// value is ready to use; it is not safe for concurrent use.
type Builder struct{} // TODO(POWERDOM): union-find scratch

// Build joins nodes on a w×h map into nets. Transmitters (conduits and
// closed switches) on 4-neighbour cells share a net; an open switch cuts.
// Each device (producer, consumer, storage) joins the nearest transmitter
// within ConnectRange of its footprint (ties by distance, then y, then
// x). It is deterministic in the nodes given, whatever their order.
func (b *Builder) Build(w, h int, nodes []Node) Graph {
	return Graph{} // TODO(POWERDOM)
}

// Dev is one device of a net for Balance. Want is the W a consumer would
// draw now (0 while switched off), Out the W a producer makes now (after
// fuel, daylight, wind or a flare). Stored and Cap are a battery's energy
// and capacity in W·ticks. ShedUntil is the tick a shed consumer stays off
// until (0: not shed). Balance sets Powered, Stored and ShedUntil.
type Dev struct {
	ID          uint64
	Role        Role
	Class       Class
	Want, Out   int
	Stored, Cap int64
	ShedUntil   uint64
	Powered     bool
}

// NetOut is one net's balance: W generated and W drawn by the consumers
// that ran, the batteries' energy and capacity (W·ticks) after it, and
// whether consumers had to be shed.
type NetOut struct {
	Gen, Load   int
	Stored, Cap int64
	Brownout    bool
}

// Balance integrates one net over dt ticks ending at tick now: generation
// covers the load, a surplus charges the batteries (with a loss) and a
// deficit draws on them; past that, consumers are shed by Class, the
// newest id first within a class, for a minimum time. It updates devs in
// place and returns the net's totals.
func Balance(devs []Dev, dt int, now uint64) NetOut {
	return NetOut{} // TODO(POWERDOM)
}
