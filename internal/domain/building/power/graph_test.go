package power

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func conduit(id uint64, x, y int) Node {
	return Node{ID: id, Role: Transmitter, Cells: [][2]int{{x, y}}}
}

func switchAt(id uint64, x, y int, closed bool) Node {
	return Node{ID: id, Role: Switch, Cells: [][2]int{{x, y}}, Closed: closed}
}

// device is a role-r node covering the w×h rectangle at (x, y).
func device(id uint64, r Role, x, y, w, h int) Node {
	n := Node{ID: id, Role: r}
	for dy := range h {
		for dx := range w {
			n.Cells = append(n.Cells, [2]int{x + dx, y + dy})
		}
	}
	return n
}

func build(w, h int, nodes ...Node) Graph {
	var b Builder
	return b.Build(w, h, nodes)
}

func wantNets(t *testing.T, g *Graph, want map[uint64]uint64) {
	t.Helper()
	for id, net := range want {
		if got := g.Net(id); got != net {
			t.Errorf("Net(%d) = %d, want %d", id, got, net)
		}
	}
}

// TestBuildMergesFourNeighbours: conduits on 4-neighbour cells share a
// net named after its smallest conduit id; diagonal cells do not touch,
// and a conduit laid between two nets joins them.
func TestBuildMergesFourNeighbours(t *testing.T) {
	t.Parallel()
	base := []Node{
		conduit(10, 0, 0), conduit(11, 1, 0), conduit(12, 2, 0), // a line
		conduit(5, 5, 0), conduit(4, 5, 1), // a short line further east
		conduit(30, 3, 1), // diagonal to the first line's end only
	}
	cases := []struct {
		name  string
		extra []Node
		nets  map[uint64]uint64
		list  []uint64
	}{
		{"three nets; a diagonal does not link", nil,
			map[uint64]uint64{10: 10, 11: 10, 12: 10, 5: 4, 4: 4, 30: 30}, []uint64{4, 10, 30}},
		{"a conduit at (3,0) joins the line and the diagonal one", []Node{conduit(40, 3, 0)},
			map[uint64]uint64{10: 10, 12: 10, 30: 10, 40: 10, 5: 4}, []uint64{4, 10}},
		{"one more at (4,0) joins everything under the smallest id",
			[]Node{conduit(40, 3, 0), conduit(1, 4, 0)},
			map[uint64]uint64{10: 1, 30: 1, 40: 1, 5: 1, 4: 1, 1: 1}, []uint64{1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := build(8, 4, append(slices.Clone(base), c.extra...)...)
			wantNets(t, &g, c.nets)
			if !slices.Equal(g.Nets(), c.list) {
				t.Errorf("Nets() = %v, want %v", g.Nets(), c.list)
			}
			for id := range c.nets {
				if _, ok := g.Link(id); ok {
					t.Errorf("transmitter %d has a link", id)
				}
			}
		})
	}
}

// TestBuildSwitchCuts: a closed switch conducts; an open one is on no net
// and its cell cuts the line, even through a conduit laid under it, so a
// device beside it joins the nearer side by the tie order.
func TestBuildSwitchCuts(t *testing.T) {
	t.Parallel()
	line := func(closed bool, under bool) []Node {
		n := []Node{conduit(1, 0, 0), conduit(2, 1, 0), conduit(3, 2, 0), switchAt(50, 3, 0, closed),
			conduit(6, 4, 0), conduit(7, 5, 0),
			device(60, Consumer, 6, 1, 1, 1), // beside conduit 7
			device(80, Consumer, 3, 1, 1, 1), // right under the switch
		}
		if under {
			n = append(n, conduit(70, 3, 0))
		}
		return n
	}
	cases := []struct {
		name          string
		closed, under bool
		nets          map[uint64]uint64
		list          []uint64
	}{
		{"closed: one net", true, false, map[uint64]uint64{1: 1, 3: 1, 50: 1, 6: 1, 7: 1, 60: 1, 80: 1}, []uint64{1}},
		{"open: split, the switch on no net", false, false,
			map[uint64]uint64{1: 1, 3: 1, 50: 0, 6: 6, 7: 6, 60: 6, 80: 1}, []uint64{1, 6}},
		{"open over a conduit: still split", false, true,
			map[uint64]uint64{1: 1, 50: 0, 70: 0, 6: 6, 60: 6, 80: 1}, []uint64{1, 6}},
		{"closed over a conduit: one net", true, true,
			map[uint64]uint64{1: 1, 50: 1, 70: 1, 6: 1, 60: 1, 80: 1}, []uint64{1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := build(8, 4, line(c.closed, c.under)...)
			wantNets(t, &g, c.nets)
			if !slices.Equal(g.Nets(), c.list) {
				t.Errorf("Nets() = %v, want %v", g.Nets(), c.list)
			}
			if _, ok := g.Link(50); ok {
				t.Error("a switch never links like a device")
			}
			// A closed switch straight above takes the wire: (2,0), (3,0) and
			// (4,0) are all 1 away, and it is the straightest. An open one
			// carries nothing: (2,0) and (4,0) tie on both, so the smaller x.
			wantX, wantTo := 2, uint64(3)
			if c.closed {
				wantX, wantTo = 3, 50
			}
			if l, ok := g.Link(80); !ok || l.X != wantX || l.Y != 0 || l.To != wantTo || l.Dist != 1 {
				t.Errorf("Link(80) = %+v, %v", l, ok)
			}
		})
	}
}

// TestBuildStableIDs: a net keeps its id when a device (even one with a
// smaller id) or a far conduit is added, and the graph does not depend on
// the order the nodes come in.
func TestBuildStableIDs(t *testing.T) {
	t.Parallel()
	base := []Node{conduit(5, 2, 2), conduit(8, 3, 2), conduit(9, 4, 2), device(100, Producer, 5, 3, 2, 2)}
	g := build(20, 20, base...)
	wantNets(t, &g, map[uint64]uint64{5: 5, 9: 5, 100: 5})
	more := append(slices.Clone(base), device(2, Consumer, 1, 3, 1, 1), conduit(3, 15, 15), device(4, None, 3, 3, 1, 1))
	g2 := build(20, 20, more...)
	wantNets(t, &g2, map[uint64]uint64{5: 5, 9: 5, 100: 5, 2: 5, 3: 3, 4: 0})
	if !slices.Equal(g2.Nets(), []uint64{3, 5}) {
		t.Errorf("Nets() = %v", g2.Nets())
	}
	if _, ok := g2.Link(4); ok {
		t.Error("a node with no role links nowhere")
	}
	r := rand.New(rand.NewPCG(1, 2))
	for range 20 {
		shuf := slices.Clone(more)
		r.Shuffle(len(shuf), func(i, j int) { shuf[i], shuf[j] = shuf[j], shuf[i] })
		g3 := build(20, 20, shuf...)
		sameGraph(t, &g2, &g3, more)
	}
}

func sameGraph(t *testing.T, a, b *Graph, nodes []Node) {
	t.Helper()
	if !slices.Equal(a.Nets(), b.Nets()) {
		t.Fatalf("Nets %v vs %v", a.Nets(), b.Nets())
	}
	for _, n := range nodes {
		la, oka := a.Link(n.ID)
		lb, okb := b.Link(n.ID)
		if a.Net(n.ID) != b.Net(n.ID) || la != lb || oka != okb {
			t.Fatalf("node %d: net %d/%d link %+v/%+v", n.ID, a.Net(n.ID), b.Net(n.ID), la, lb)
		}
	}
}

// TestBuildConnectRange: a device joins the nearest conductor within
// ConnectRange (Chebyshev, from any footprint cell; ties by distance, then
// straight line, then y, then x), at distance 0 when one lies under it; the link remembers
// the conductor cell and the device cell nearest it.
func TestBuildConnectRange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		nodes []Node
		dev   uint64
		ok    bool
		want  Link
	}{
		{"6 tiles east joins", []Node{conduit(1, 10, 10), device(9, Consumer, 16, 10, 1, 1)}, 9, true,
			Link{To: 1, X: 10, Y: 10, FromX: 16, FromY: 10, Dist: 6}},
		{"6 tiles diagonal joins", []Node{conduit(1, 10, 10), device(9, Consumer, 16, 16, 1, 1)}, 9, true,
			Link{To: 1, X: 10, Y: 10, FromX: 16, FromY: 16, Dist: 6}},
		{"7 tiles is out of range", []Node{conduit(1, 10, 10), device(9, Consumer, 17, 10, 1, 1)}, 9, false, Link{}},
		{"from the footprint's nearest cell", []Node{conduit(1, 10, 10), device(9, Storage, 4, 11, 1, 2)}, 9, true,
			Link{To: 1, X: 10, Y: 10, FromX: 4, FromY: 11, Dist: 6}},
		{"equal distance: the smaller x", []Node{conduit(50, 20, 20), conduit(40, 24, 20),
			device(9, Consumer, 22, 20, 1, 1)}, 9, true, Link{To: 50, X: 20, Y: 20, FromX: 22, FromY: 20, Dist: 2, Next: 40}},
		{"equal distance: the smaller y first", []Node{conduit(60, 30, 32), conduit(61, 30, 28),
			device(9, Consumer, 30, 30, 1, 1)}, 9, true, Link{To: 61, X: 30, Y: 28, FromX: 30, FromY: 30, Dist: 2, Next: 60}},
		{"equal distance: the tile straight below, not the diagonal", []Node{conduit(70, 19, 21), conduit(71, 20, 21), conduit(72, 21, 21),
			device(9, Consumer, 20, 20, 1, 1)}, 9, true, Link{To: 71, X: 20, Y: 21, FromX: 20, FromY: 20, Dist: 1}},
		{"equal distance: the straighter beats a smaller y", []Node{conduit(1, 68, 68), conduit(2, 70, 72),
			device(9, Consumer, 70, 70, 1, 1)}, 9, true, Link{To: 2, X: 70, Y: 72, FromX: 70, FromY: 70, Dist: 2, Next: 1}},
		{"the nearer beats a smaller y", []Node{conduit(1, 40, 37), conduit(2, 40, 42),
			device(9, Consumer, 40, 40, 1, 1)}, 9, true, Link{To: 2, X: 40, Y: 42, FromX: 40, FromY: 40, Dist: 2, Next: 1}},
		{"a conduit under it joins at 0", []Node{conduit(1, 49, 50), conduit(2, 51, 51),
			device(9, Consumer, 50, 50, 3, 3)}, 9, true, Link{To: 2, X: 51, Y: 51, FromX: 51, FromY: 51, Dist: 0, Next: 1}},
		{"the device cell straightest to it", []Node{conduit(1, 64, 61), device(9, Producer, 60, 60, 3, 3)}, 9, true,
			Link{To: 1, X: 64, Y: 61, FromX: 62, FromY: 61, Dist: 2}},
		{"two conduits on one cell: the smaller id", []Node{conduit(7, 5, 5), conduit(3, 5, 5),
			device(9, Consumer, 6, 6, 1, 1)}, 9, true, Link{To: 3, X: 5, Y: 5, FromX: 6, FromY: 6, Dist: 1}},
		{"at the map's corner", []Node{conduit(1, 0, 0), device(9, Consumer, 2, 2, 1, 1)}, 9, true,
			Link{To: 1, X: 0, Y: 0, FromX: 2, FromY: 2, Dist: 2}},
		{"no transmitter at all", []Node{device(9, Consumer, 2, 2, 1, 1)}, 9, false, Link{}},
		{"no footprint", []Node{conduit(1, 0, 0), {ID: 9, Role: Consumer}}, 9, false, Link{}},
		{"off the map", []Node{conduit(1, 0, 0), device(9, Consumer, -2, -1, 1, 1)}, 9, false, Link{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := build(80, 80, c.nodes...)
			l, ok := g.Link(c.dev)
			if ok != c.ok || l != c.want {
				t.Fatalf("Link = %+v, %v; want %+v, %v", l, ok, c.want, c.ok)
			}
			wantNet := uint64(0)
			if ok {
				wantNet = g.Net(l.To)
			}
			if g.Net(c.dev) != wantNet || (ok && wantNet == 0) {
				t.Errorf("Net = %d, want %d", g.Net(c.dev), wantNet)
			}
		})
	}
}

// TestBuildConductors: generators and batteries conduct. Touching a conduit
// or closed switch they are on its net with no wire; a set of only them wires
// out once, the wire merging the sets; alone they are on no net, and several
// touching are their own. Consumers never conduct and never name a net.
func TestBuildConductors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		nodes []Node
		nets  map[uint64]uint64
		wires map[uint64]uint64 // Link.To; 0: no wire
		list  []uint64
	}{
		{"a lamp 5 tiles from a generator wires to it",
			[]Node{device(1, Producer, 10, 10, 1, 1), device(9, Consumer, 15, 10, 1, 1)},
			map[uint64]uint64{1: 1, 9: 1}, map[uint64]uint64{1: 0, 9: 1}, []uint64{1}},
		{"a lamp 7 tiles from a generator doesn't",
			[]Node{device(1, Producer, 10, 10, 1, 1), device(9, Consumer, 17, 10, 1, 1)},
			map[uint64]uint64{1: 0, 9: 0}, map[uint64]uint64{1: 0, 9: 0}, nil},
		{"a lamp's id never names the net",
			[]Node{device(1, Consumer, 15, 10, 1, 1), device(9, Storage, 10, 10, 1, 1)},
			map[uint64]uint64{1: 9, 9: 9}, map[uint64]uint64{1: 9, 9: 0}, []uint64{9}},
		{"a generator touching a conduit has no wire",
			[]Node{conduit(5, 10, 10), device(1, Producer, 11, 10, 1, 1)},
			map[uint64]uint64{1: 1, 5: 1}, map[uint64]uint64{1: 0}, []uint64{1}},
		{"a generator sharing a conduit's cell has no wire",
			[]Node{conduit(5, 10, 10), device(7, Producer, 10, 10, 2, 2)},
			map[uint64]uint64{5: 5, 7: 5}, map[uint64]uint64{7: 0}, []uint64{5}},
		{"a diagonal doesn't touch: it wires",
			[]Node{conduit(5, 10, 10), device(7, Storage, 11, 11, 1, 1)},
			map[uint64]uint64{5: 5, 7: 5}, map[uint64]uint64{7: 5}, []uint64{5}},
		{"a generator's wire merges, its id naming the net",
			[]Node{conduit(5, 10, 10), device(2, Producer, 14, 10, 1, 1)},
			map[uint64]uint64{5: 2, 2: 2}, map[uint64]uint64{2: 5}, []uint64{2}},
		{"a lone generator is on no net",
			[]Node{device(1, Producer, 10, 10, 1, 1)},
			map[uint64]uint64{1: 0}, map[uint64]uint64{1: 0}, nil},
		{"a lone multi-cell generator never wires to itself",
			[]Node{device(1, Producer, 10, 10, 3, 3)},
			map[uint64]uint64{1: 0}, map[uint64]uint64{1: 0}, nil},
		{"a lone battery out of a conduit's range",
			[]Node{conduit(5, 0, 0), device(1, Storage, 7, 0, 1, 1)},
			map[uint64]uint64{5: 5, 1: 0}, map[uint64]uint64{1: 0}, []uint64{5}},
		{"two batteries in range merge once",
			[]Node{device(3, Storage, 10, 10, 1, 1), device(4, Storage, 14, 10, 1, 1)},
			map[uint64]uint64{3: 3, 4: 3}, map[uint64]uint64{3: 4, 4: 3}, []uint64{3}},
		{"two touching batteries need no wire",
			[]Node{device(3, Storage, 10, 10, 1, 1), device(4, Storage, 11, 10, 1, 1)},
			map[uint64]uint64{3: 3, 4: 3}, map[uint64]uint64{3: 0, 4: 0}, []uint64{3}},
		{"a battery pair 2 tiles from a conduit: one wire, the smaller id's",
			[]Node{device(3, Storage, 10, 10, 1, 1), device(4, Storage, 11, 10, 1, 1), conduit(8, 13, 10)},
			map[uint64]uint64{3: 3, 4: 3, 8: 3}, map[uint64]uint64{3: 8, 4: 0}, []uint64{3}},
		{"a generator and battery touching: likewise",
			[]Node{device(3, Producer, 10, 10, 2, 2), device(4, Storage, 12, 10, 1, 2), conduit(8, 15, 10)},
			map[uint64]uint64{3: 3, 4: 3, 8: 3}, map[uint64]uint64{3: 8, 4: 0}, []uint64{3}},
		{"a pair with nothing in range is its own grid, no wire",
			[]Node{device(3, Storage, 10, 10, 1, 1), device(4, Producer, 11, 10, 1, 1), conduit(8, 18, 10)},
			map[uint64]uint64{3: 3, 4: 3, 8: 8}, map[uint64]uint64{3: 0, 4: 0}, []uint64{3, 8}},
		{"the owner is the next id when the smallest has nothing in range",
			[]Node{device(3, Storage, 10, 10, 1, 1), device(4, Producer, 11, 10, 1, 1), conduit(8, 17, 10)},
			map[uint64]uint64{3: 3, 4: 3, 8: 3}, map[uint64]uint64{3: 0, 4: 8}, []uint64{3}},
		{"a pair never wires to itself, whatever is nearest",
			[]Node{device(3, Storage, 10, 10, 3, 1), device(4, Storage, 13, 10, 3, 1)},
			map[uint64]uint64{3: 3, 4: 3}, map[uint64]uint64{3: 0, 4: 0}, []uint64{3}},
		{"a conduit in the set: no wire, even with another grid nearer",
			[]Node{device(3, Storage, 10, 10, 1, 1), conduit(4, 11, 10), conduit(8, 8, 10)},
			map[uint64]uint64{3: 3, 4: 3, 8: 8}, map[uint64]uint64{3: 0}, []uint64{3, 8}},
		{"two sets in range each wire once, and merge",
			[]Node{device(1, Storage, 10, 10, 1, 1), device(3, Producer, 13, 10, 1, 1), device(4, Producer, 14, 10, 1, 1)},
			map[uint64]uint64{1: 1, 3: 1, 4: 1}, map[uint64]uint64{1: 3, 3: 1, 4: 0}, []uint64{1}},
		{"three sets in range: each wires once, one grid",
			[]Node{device(1, Storage, 10, 10, 1, 1), device(2, Producer, 13, 10, 1, 1), device(3, Storage, 16, 10, 1, 1)},
			map[uint64]uint64{1: 1, 2: 1, 3: 1}, map[uint64]uint64{1: 2, 2: 1, 3: 2}, []uint64{1}},
		{"a multi-cell battery wires from its nearest cell",
			[]Node{conduit(5, 20, 10), device(1, Storage, 15, 10, 2, 2)},
			map[uint64]uint64{1: 1, 5: 1}, map[uint64]uint64{1: 5}, []uint64{1}},
		{"a consumer doesn't conduct onward",
			[]Node{conduit(5, 10, 10), device(6, Consumer, 16, 10, 1, 1), device(7, Consumer, 20, 10, 1, 1)},
			map[uint64]uint64{5: 5, 6: 5, 7: 0}, map[uint64]uint64{6: 5, 7: 0}, []uint64{5}},
		{"a closed switch beside a generator joins it",
			[]Node{switchAt(2, 10, 10, true), device(1, Producer, 11, 10, 1, 1)},
			map[uint64]uint64{1: 1, 2: 1}, map[uint64]uint64{1: 0}, []uint64{1}},
		{"an open switch beside a generator doesn't",
			[]Node{switchAt(2, 10, 10, false), device(1, Producer, 11, 10, 1, 1)},
			map[uint64]uint64{1: 0, 2: 0}, map[uint64]uint64{1: 0}, nil},
		{"an open switch cuts the cell a generator shares",
			[]Node{switchAt(2, 10, 10, false), conduit(3, 9, 10), device(4, Producer, 10, 10, 1, 1)},
			map[uint64]uint64{2: 0, 3: 3, 4: 3}, map[uint64]uint64{4: 3}, []uint64{3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := build(40, 40, c.nodes...)
			wantNets(t, &g, c.nets)
			if !slices.Equal(g.Nets(), c.list) {
				t.Errorf("Nets() = %v, want %v", g.Nets(), c.list)
			}
			for id, to := range c.wires {
				l, ok := g.Link(id)
				if ok != (to != 0) || l.To != to {
					t.Errorf("Link(%d) = %+v, %v; want to %d", id, l, ok, to)
				}
			}
		})
	}
}

// TestBuildPin: a wire goes to its pin while that conducts on an uncut cell
// in range, falls back to the nearest otherwise and returns when it can.
func TestBuildPin(t *testing.T) {
	t.Parallel()
	lamp := func(pin uint64) Node {
		n := device(9, Consumer, 11, 12, 1, 1)
		n.Pin = pin
		return n
	}
	near, far := conduit(1, 10, 10), conduit(2, 15, 10) // the lamp is 2 and 4 away
	cases := []struct {
		name  string
		nodes []Node
		to    uint64
		x, y  int
	}{
		{"no pin: the nearest", []Node{near, far, lamp(0)}, 1, 10, 10},
		{"pinned: the farther", []Node{near, far, lamp(2)}, 2, 15, 10},
		{"the pin is gone", []Node{near, lamp(2)}, 1, 10, 10},
		{"the pin is out of range", []Node{near, conduit(2, 18, 10), lamp(2)}, 1, 10, 10},
		{"the pin is an open switch", []Node{near, switchAt(2, 15, 10, false), lamp(2)}, 1, 10, 10},
		{"the switch closed: back to the pin", []Node{near, switchAt(2, 15, 10, true), lamp(2)}, 2, 15, 10},
		{"the pin under an open switch", []Node{near, far, switchAt(3, 15, 10, false), lamp(2)}, 1, 10, 10},
		{"the pin is a consumer", []Node{near, device(2, Consumer, 15, 10, 1, 1), lamp(2)}, 1, 10, 10},
		{"the pin is itself", []Node{near, far, lamp(9)}, 1, 10, 10},
		{"a pinned generator: its nearest cell", []Node{near, device(2, Producer, 15, 11, 2, 2), lamp(2)}, 2, 15, 12},
		{"a nearer conductor doesn't pull it", []Node{near, far, conduit(3, 11, 11), lamp(2)}, 2, 15, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := build(40, 40, c.nodes...)
			l, ok := g.Link(9)
			if !ok || l.To != c.to || l.X != c.x || l.Y != c.y {
				t.Fatalf("Link = %+v, %v; want %d at %d,%d", l, ok, c.to, c.x, c.y)
			}
			if g.Net(9) != g.Net(c.to) || g.Net(9) == 0 {
				t.Errorf("Net = %d, its conductor's %d", g.Net(9), g.Net(c.to))
			}
		})
	}

	t.Run("a wired battery's pin merges only that set", func(t *testing.T) {
		t.Parallel()
		bat := device(9, Storage, 11, 12, 1, 1)
		bat.Pin = 2
		g := build(40, 40, near, far, bat)
		wantNets(t, &g, map[uint64]uint64{1: 1, 2: 2, 9: 2})
	})
	t.Run("a pinned non-smallest member owns the set's wire", func(t *testing.T) {
		t.Parallel()
		small, pinned := device(5, Storage, 12, 12, 1, 1), device(6, Storage, 13, 12, 1, 1)
		pinned.Pin = 2
		g := build(40, 40, near, far, small, pinned)
		wantNets(t, &g, map[uint64]uint64{1: 1, 2: 2, 5: 2, 6: 2})
		if l, ok := g.Link(5); ok {
			t.Errorf("the smaller id has a wire too: %+v", l)
		}
		if l, ok := g.Link(6); !ok || l.To != 2 {
			t.Errorf("Link(6) = %+v, %v", l, ok)
		}
	})
	t.Run("two pinned members: the smaller id owns it", func(t *testing.T) {
		t.Parallel()
		a, b := device(5, Storage, 12, 12, 1, 1), device(6, Storage, 13, 12, 1, 1)
		a.Pin, b.Pin = 2, 1
		g := build(40, 40, near, far, a, b)
		wantNets(t, &g, map[uint64]uint64{1: 1, 2: 2, 5: 2, 6: 2})
		if l, ok := g.Link(5); !ok || l.To != 2 {
			t.Errorf("Link(5) = %+v, %v", l, ok)
		}
		if l, ok := g.Link(6); ok {
			t.Errorf("Link(6) = %+v", l)
		}
	})
	t.Run("a pin on its own set is no pin", func(t *testing.T) {
		t.Parallel()
		a, b := device(5, Storage, 12, 12, 1, 1), device(6, Storage, 13, 12, 1, 1)
		b.Pin = 5
		g := build(40, 40, near, a, b)
		if l, ok := g.Link(5); !ok || l.To != 1 {
			t.Errorf("Link(5) = %+v, %v", l, ok)
		}
		if l, ok := g.Link(6); ok {
			t.Errorf("Link(6) = %+v", l)
		}
	})
	t.Run("a member out of range of its pin leaves the wire to the smallest", func(t *testing.T) {
		t.Parallel()
		a, b := device(5, Storage, 12, 12, 1, 1), device(6, Storage, 13, 12, 1, 1)
		b.Pin = 3
		g := build(40, 40, near, conduit(3, 30, 30), a, b)
		if l, ok := g.Link(5); !ok || l.To != 1 {
			t.Errorf("Link(5) = %+v, %v", l, ok)
		}
		if l, ok := g.Link(6); ok {
			t.Errorf("Link(6) = %+v", l)
		}
	})
	t.Run("a device touching a conductor ignores its pin", func(t *testing.T) {
		t.Parallel()
		bat := device(9, Storage, 11, 10, 1, 1)
		bat.Pin = 2
		g := build(40, 40, near, far, bat)
		wantNets(t, &g, map[uint64]uint64{1: 1, 2: 2, 9: 1})
		if l, ok := g.Link(9); ok {
			t.Errorf("Link = %+v", l)
		}
	})
}

// TestBuildNextCycles: Next walks the sets in range by ascending net id and
// wraps; it picks the next set's nearest cell, and is 0 with one set.
func TestBuildNextCycles(t *testing.T) {
	t.Parallel()
	nodes := []Node{conduit(10, 8, 8), conduit(20, 14, 8), conduit(30, 11, 3), device(9, Consumer, 11, 8, 1, 1)}
	lamp := &nodes[3]
	want := []uint64{30, 10, 20, 30, 10}
	g := build(40, 40, nodes...)
	if l, _ := g.Link(9); l.To != 10 || l.Next != 20 { // 3 away from both: the smaller x
		t.Fatalf("start %+v", l)
	}
	lamp.Pin = 20
	for i, to := range want {
		g = build(40, 40, nodes...)
		l, ok := g.Link(9)
		if !ok || l.Next != to || g.Net(9) != lamp.Pin {
			t.Fatalf("step %d: pin %d, link %+v (%v), net %d; want next %d", i, lamp.Pin, l, ok, g.Net(9), to)
		}
		lamp.Pin = l.Next
	}

	cases := []struct {
		name  string
		nodes []Node
		next  uint64
	}{
		{"one set in range: 0", []Node{conduit(10, 8, 8), conduit(11, 9, 8), device(9, Consumer, 11, 8, 1, 1)}, 0},
		{"the other set's out of range: 0", []Node{conduit(10, 8, 8), conduit(20, 18, 8), device(9, Consumer, 11, 8, 1, 1)}, 0},
		{"the next set's nearest cell, not its smallest id", []Node{conduit(10, 8, 8), conduit(20, 16, 8), conduit(21, 15, 8), device(9, Consumer, 11, 8, 1, 1)}, 21},
		{"a tie: the straighter", []Node{conduit(10, 8, 8), conduit(20, 15, 8), conduit(21, 15, 7), device(9, Consumer, 11, 8, 1, 1)}, 20},
		{"a tie: the smaller y, equally straight", []Node{conduit(10, 8, 8), conduit(20, 15, 9), conduit(21, 15, 7), conduit(23, 16, 9), conduit(24, 16, 8), conduit(25, 16, 7), device(9, Consumer, 11, 8, 1, 1)}, 21},
		{"a tie: the smaller x, equally straight", []Node{conduit(10, 8, 8), conduit(20, 13, 12), conduit(22, 9, 12),
			conduit(30, 9, 13), conduit(31, 10, 13), conduit(32, 11, 13), conduit(33, 12, 13), conduit(34, 13, 13), device(9, Consumer, 11, 8, 1, 1)}, 22},
		{"a lone generator counts", []Node{conduit(10, 8, 8), device(20, Producer, 15, 8, 1, 1), device(9, Consumer, 11, 8, 1, 1)}, 20},
		{"an open switch doesn't", []Node{conduit(10, 8, 8), switchAt(20, 15, 8, false), device(9, Consumer, 11, 8, 1, 1)}, 0},
		{"a closed switch does", []Node{conduit(10, 8, 8), switchAt(20, 15, 8, true), device(9, Consumer, 11, 8, 1, 1)}, 20},
		{"a wired battery skips its own cells and set", []Node{conduit(10, 8, 8), conduit(20, 15, 8), device(9, Storage, 11, 8, 2, 2)}, 20},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := build(40, 40, c.nodes...)
			l, ok := g.Link(9)
			if !ok || l.Next != c.next {
				t.Fatalf("Link = %+v, %v; want next %d", l, ok, c.next)
			}
		})
	}
}

// TestBuildLaws: over seeded layouts with pins, nothing wires to itself,
// every wire's conductor conducts and shares its net, Next is on another
// set, Nets() holds exactly the nets on nodes, and node order doesn't count.
func TestBuildLaws(t *testing.T) {
	t.Parallel()
	for seed := range uint64(6) {
		nodes := mapNodes(60, 40, 60, seed+20)
		r := rand.New(rand.NewPCG(seed, 99))
		for i := range nodes {
			if nodes[i].Role.Device() && r.IntN(3) == 0 {
				nodes[i].Pin = nodes[r.IntN(len(nodes))].ID
			}
		}
		g := build(60, 40, nodes...)
		byID := map[uint64]*Node{}
		for i := range nodes {
			byID[nodes[i].ID] = &nodes[i]
		}
		seen := map[uint64]bool{}
		for i := range nodes {
			n := &nodes[i]
			net := g.Net(n.ID)
			if net != 0 {
				seen[net] = true
				if !slices.Contains(g.Nets(), net) {
					t.Fatalf("seed %d: node %d on net %d, not in Nets", seed, n.ID, net)
				}
				if to := byID[net]; to == nil || !conducts(to) {
					t.Fatalf("seed %d: net %d names no conductor", seed, net)
				}
			}
			l, ok := g.Link(n.ID)
			if !ok {
				continue
			}
			if l.To == n.ID || l.Next == n.ID || !conducts(byID[l.To]) || net == 0 || g.Net(l.To) != net {
				t.Fatalf("seed %d: node %d net %d link %+v", seed, n.ID, net, l)
			}
			if l.Next != 0 && (!conducts(byID[l.Next]) || (g.Net(l.Next) == net && net != 0)) {
				t.Fatalf("seed %d: node %d net %d next %d on net %d", seed, n.ID, net, l.Next, g.Net(l.Next))
			}
		}
		if len(seen) != len(g.Nets()) {
			t.Fatalf("seed %d: %d nets on nodes, Nets %v", seed, len(seen), g.Nets())
		}
		for range 8 {
			shuf := slices.Clone(nodes)
			r.Shuffle(len(shuf), func(i, j int) { shuf[i], shuf[j] = shuf[j], shuf[i] })
			g2 := build(60, 40, shuf...)
			sameGraph(t, &g, &g2, nodes)
		}
	}
}

// TestBuildOneWirePerDeviceSet: over seeded layouts of mostly touching
// devices, a set of only generators and batteries has one wire if any member
// reaches a conductor outside it and none otherwise, a set with a conduit has
// none, and node order doesn't count.
func TestBuildOneWirePerDeviceSet(t *testing.T) {
	t.Parallel()
	wiredPairs := 0
	for seed := range uint64(12) {
		r := rand.New(rand.NewPCG(seed, 7))
		var nodes []Node
		for id := uint64(1); id <= 40; id++ {
			x, y := r.IntN(28), r.IntN(28)
			switch r.IntN(6) {
			case 0:
				nodes = append(nodes, conduit(id, x, y))
			case 1, 2, 3:
				nodes = append(nodes, device(id, Producer, x, y, 1+r.IntN(3), 1+r.IntN(3)))
			default:
				nodes = append(nodes, device(id, Storage, x, y, 1, 1+r.IntN(2)))
			}
			if n := &nodes[len(nodes)-1]; n.Role != Transmitter && r.IntN(4) == 0 {
				n.Pin = 1 + r.Uint64N(40)
			}
		}
		sets := touchingSets(nodes)
		g := build(32, 32, nodes...)
		for root, members := range sets {
			wires, devicesOnly, reach := 0, true, false
			for _, m := range members {
				if _, ok := g.Link(m.ID); ok {
					wires++
				}
				devicesOnly = devicesOnly && m.Role != Transmitter
				for _, o := range nodes {
					if sets.rootOf(o.ID) != root && within(m, o) {
						reach = true
					}
				}
			}
			want := 0
			if devicesOnly && reach {
				want = 1
			}
			if wires != want {
				t.Fatalf("seed %d: set %v has %d wires, want %d", seed, members, wires, want)
			}
			if want == 1 && len(members) > 1 {
				wiredPairs++
			}
		}
		for range 6 {
			shuf := slices.Clone(nodes)
			r.Shuffle(len(shuf), func(i, j int) { shuf[i], shuf[j] = shuf[j], shuf[i] })
			g2 := build(32, 32, shuf...)
			sameGraph(t, &g, &g2, nodes)
		}
	}
	if wiredPairs == 0 {
		t.Fatal("no layout had a touching device set to wire")
	}
}

// nodeSets is nodes grouped by touching, keyed by a member's id.
type nodeSets map[uint64][]Node

func (s nodeSets) rootOf(id uint64) uint64 {
	for root, members := range s {
		for _, m := range members {
			if m.ID == id {
				return root
			}
		}
	}
	return 0
}

// touchingSets groups nodes that share or 4-neighbour a cell, the plain way.
func touchingSets(nodes []Node) nodeSets {
	touches := func(a, b Node) bool {
		for _, c := range a.Cells {
			for _, d := range b.Cells {
				dx, dy := c[0]-d[0], c[1]-d[1]
				if dx*dx+dy*dy <= 1 {
					return true
				}
			}
		}
		return false
	}
	seen := map[uint64]bool{}
	sets := nodeSets{}
	for _, n := range nodes {
		if seen[n.ID] {
			continue
		}
		seen[n.ID] = true
		group := []Node{n}
		for k := 0; k < len(group); k++ {
			for _, o := range nodes {
				if !seen[o.ID] && touches(group[k], o) {
					seen[o.ID] = true
					group = append(group, o)
				}
			}
		}
		sets[n.ID] = group
	}
	return sets
}

// within: a cell of a lies in ConnectRange of a cell of b.
func within(a, b Node) bool {
	for _, c := range a.Cells {
		for _, d := range b.Cells {
			if cheb(c[0]-d[0], c[1]-d[1]) <= ConnectRange {
				return true
			}
		}
	}
	return false
}

// TestBuildIgnoresOffMapCells: transmitter cells off the map are dropped;
// a conduit with none left is on no net.
func TestBuildIgnoresOffMapCells(t *testing.T) {
	t.Parallel()
	g := build(4, 4, conduit(1, -1, 0), conduit(2, 4, 0), conduit(3, 0, 4),
		Node{ID: 4, Role: Transmitter, Cells: [][2]int{{3, 3}, {4, 3}}})
	wantNets(t, &g, map[uint64]uint64{1: 0, 2: 0, 3: 0, 4: 4})
	if !slices.Equal(g.Nets(), []uint64{4}) {
		t.Errorf("Nets() = %v", g.Nets())
	}
	var zero Graph
	if zero.Net(1) != 0 || len(zero.Nets()) != 0 {
		t.Error("the zero graph has no nets")
	}
	if _, ok := zero.Link(1); ok {
		t.Error("the zero graph has no links")
	}
	if e := build(-3, 0, conduit(1, 0, 0)); e.Net(1) != 0 || len(e.Nets()) != 0 {
		t.Error("an empty map holds no net")
	}
}

// TestBuilderReusesScratch: one Builder over maps of changing size gives
// what a fresh one gives, and an earlier Graph stays intact.
func TestBuilderReusesScratch(t *testing.T) {
	t.Parallel()
	big := mapNodes(120, 90, 600, 7)
	small := mapNodes(30, 20, 60, 8)
	other := mapNodes(120, 90, 500, 9)
	var b Builder
	g1 := b.Build(120, 90, big)
	keep := slices.Clone(g1.Nets())
	g2 := b.Build(30, 20, small)
	g3 := b.Build(120, 90, other)
	g4 := b.Build(140, 100, big)
	for _, c := range []struct {
		g     *Graph
		w, h  int
		nodes []Node
	}{{&g1, 120, 90, big}, {&g2, 30, 20, small}, {&g3, 120, 90, other}, {&g4, 140, 100, big}} {
		fresh := build(c.w, c.h, c.nodes...)
		sameGraph(t, c.g, &fresh, c.nodes)
	}
	if !slices.Equal(keep, g1.Nets()) {
		t.Fatal("a later build changed an earlier graph")
	}
}

// TestBuilderMakesNoGarbage: a reused Builder allocates only the graph it
// returns, and a set joined in one build doesn't stay joined in the next.
func TestBuilderMakesNoGarbage(t *testing.T) {
	nodes := mapNodes(120, 90, 600, 7)
	var b Builder
	b.Build(120, 90, nodes)
	if n := testing.AllocsPerRun(20, func() { b.Build(120, 90, nodes) }); n > 2 {
		t.Errorf("%v allocations a build, want the graph's 2", n)
	}
	pair := []Node{device(1, Storage, 2, 2, 1, 1), device(2, Storage, 3, 2, 1, 1)}
	b.Build(10, 10, pair)
	lone := pair[:1]
	if g := b.Build(10, 10, lone); g.Net(1) != 0 || len(g.Nets()) != 0 {
		t.Errorf("a lone battery after a pair: net %d, nets %v", g.Net(1), g.Nets())
	}
}

// mapNodes is a colony-like layout: conduit lines with switches (some
// open), and devices of mixed sizes scattered from seed.
func mapNodes(w, h, devices int, seed uint64) []Node {
	r := rand.New(rand.NewPCG(seed, 1))
	var nodes []Node
	id := uint64(1)
	for y := 10; y < h; y += 50 {
		for x := 5; x < w; x++ {
			if x%40 == 0 {
				nodes = append(nodes, switchAt(id, x, y, x%80 == 0))
			} else {
				nodes = append(nodes, conduit(id, x, y))
			}
			id++
		}
		// a vertical spur every 60 tiles
		for x := 30; x < w; x += 60 {
			for dy := 1; dy < 12 && y+dy < h; dy++ {
				nodes = append(nodes, conduit(id, x, y+dy))
				id++
			}
		}
	}
	roles := []Role{Consumer, Consumer, Consumer, Producer, Storage}
	sizes := [][2]int{{1, 1}, {1, 1}, {2, 1}, {1, 2}, {2, 2}, {3, 3}}
	for range devices {
		s := sizes[r.IntN(len(sizes))]
		nodes = append(nodes, device(id, roles[r.IntN(len(roles))], r.IntN(w-s[0]+1), r.IntN(h-s[1]+1), s[0], s[1]))
		id++
	}
	r.Shuffle(len(nodes), func(i, j int) { nodes[i], nodes[j] = nodes[j], nodes[i] })
	return nodes
}

// BenchmarkBuild is a rebuild at map scale: 250×250 with about 2000
// nodes (conduit lines, switches and devices).
func BenchmarkBuild(b *testing.B) {
	nodes := mapNodes(250, 250, 700, 3)
	b.Logf("%d nodes", len(nodes))
	var bl Builder
	bl.Build(250, 250, nodes)
	b.ReportAllocs()
	for b.Loop() {
		bl.Build(250, 250, nodes)
	}
}

// A battery's wire carries whatever is wired to it (PWR-4): pinned to the
// other conduit, the generator on it goes along and the first conduit is left.
func TestBuildPinCarriesWiredSets(t *testing.T) {
	t.Parallel()
	nodes := func(pin uint64) []Node {
		s := device(1, Storage, 14, 10, 1, 1)
		s.Pin = pin
		return []Node{conduit(5, 8, 10), conduit(6, 20, 10), s, device(2, Producer, 14, 16, 1, 1)}
	}
	g := build(40, 40, nodes(0)...)
	wantNets(t, &g, map[uint64]uint64{1: 1, 2: 1, 5: 1, 6: 6})
	if l, ok := g.Link(1); !ok || l.To != 5 || l.Next != 6 {
		t.Fatalf("before: Link = %+v, %v", l, ok)
	}
	g = build(40, 40, nodes(6)...)
	wantNets(t, &g, map[uint64]uint64{1: 1, 2: 1, 6: 1, 5: 5})
	if l, ok := g.Link(1); !ok || l.To != 6 || l.Next != 5 {
		t.Fatalf("after: Link = %+v, %v", l, ok)
	}
}
