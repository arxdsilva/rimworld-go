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
			// Under the switch: conduits (2,0) and (4,0) are both 1 away;
			// same y, so the smaller x wins (a closed switch at distance 1
			// on the same row would win only by x too: (2,0) < (3,0)).
			if l, ok := g.Link(80); !ok || l.X != 2 || l.Y != 0 || l.To != 3 || l.Dist != 1 {
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

// TestBuildConnectRange: a device joins the nearest transmitter within
// ConnectRange (Chebyshev, from any footprint cell; ties by distance, then
// y, then x), at distance 0 when one lies under it; the link remembers
// the transmitter cell and the device cell nearest it.
func TestBuildConnectRange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		nodes []Node
		dev   uint64
		ok    bool
		want  Link
	}{
		{"3 tiles east joins", []Node{conduit(1, 10, 10), device(9, Consumer, 13, 10, 1, 1)}, 9, true,
			Link{To: 1, X: 10, Y: 10, FromX: 13, FromY: 10, Dist: 3}},
		{"3 tiles diagonal joins", []Node{conduit(1, 10, 10), device(9, Consumer, 13, 13, 1, 1)}, 9, true,
			Link{To: 1, X: 10, Y: 10, FromX: 13, FromY: 13, Dist: 3}},
		{"4 tiles is out of range", []Node{conduit(1, 10, 10), device(9, Consumer, 14, 10, 1, 1)}, 9, false, Link{}},
		{"from the footprint's nearest cell", []Node{conduit(1, 10, 10), device(9, Storage, 7, 11, 1, 2)}, 9, true,
			Link{To: 1, X: 10, Y: 10, FromX: 7, FromY: 11, Dist: 3}},
		{"equal distance: the smaller x", []Node{conduit(50, 20, 20), conduit(40, 24, 20),
			device(9, Consumer, 22, 20, 1, 1)}, 9, true, Link{To: 50, X: 20, Y: 20, FromX: 22, FromY: 20, Dist: 2}},
		{"equal distance: the smaller y first", []Node{conduit(60, 30, 32), conduit(61, 30, 28),
			device(9, Consumer, 30, 30, 1, 1)}, 9, true, Link{To: 61, X: 30, Y: 28, FromX: 30, FromY: 30, Dist: 2}},
		{"the nearer beats a smaller y", []Node{conduit(1, 40, 37), conduit(2, 40, 42),
			device(9, Consumer, 40, 40, 1, 1)}, 9, true, Link{To: 2, X: 40, Y: 42, FromX: 40, FromY: 40, Dist: 2}},
		{"a conduit under it joins at 0", []Node{conduit(1, 49, 50), conduit(2, 51, 51),
			device(9, Producer, 50, 50, 3, 3)}, 9, true, Link{To: 2, X: 51, Y: 51, FromX: 51, FromY: 51, Dist: 0}},
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
