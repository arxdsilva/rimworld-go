package sheet

import (
	"math"

	"rimworld-go/internal/platform/config"
)

// The docked card's sizes (config.HUDPane.Height and TabW, config.HUDText
// BodySize, LabelW and ValueW) are set for a 720-tall logical screen.
// DockedFor fits them to the real one, so the pane keeps its share of the
// screen at any size and any UI scale.
const (
	// DockRefH is the logical screen height the docked sizes are set for.
	DockRefH = 720
	// DockTabsH is the pane's tab strip above it (hudlayout.PaneTabsH).
	DockTabsH = 24
	// DockMargin is what the pane takes around the card's body: the name row
	// on top and an edge below (inspector.PaneLayout with a card).
	DockMargin = 34
	// DockSharePct is the most of the screen's height the pane and its tabs
	// take, unless the smallest readable card needs more.
	DockSharePct = 30
	// DockMinFont is the smallest body text the docked card shrinks to.
	DockMinFont = 9
)

// The built-in docked tree's height in body lines and rules, the same for every
// target (TestDockRowsMatchTheDefault).
const (
	dockLines = 8
	dockRules = 1
)

// DockedFor is the docked card's settings on a logical screen sh tall. With
// k = sh/720 the text grows with k above 720, and the pane too, never past 30 %
// of the screen with its tabs. Below 720 the pane keeps its height while that
// fits in 30 % and takes 30 % otherwise; the text is the largest, up to its size
// times k, that leaves the layout room, and never smaller than 9 (the pane grows
// past 30 % for that only on a screen under 550). The column and tab widths
// follow the text. It copies values only.
func DockedFor(h config.HUD, sh int) config.HUD {
	t := h.Text
	if sh <= 0 || t.BodySize <= 0 {
		return h
	}
	k := float64(sh) / DockRefH
	floor := min(t.BodySize, DockMinFont)
	f := max(floor, int(math.Round(float64(t.BodySize)*k)))
	if sh >= DockRefH {
		h.Pane.Height = min(h.Pane.Height*sh/DockRefH, sh*DockSharePct/100-DockTabsH)
	} else {
		share := sh*DockSharePct/100 - DockTabsH
		h.Pane.Height = max(min(share, h.Pane.Height), dockNeed(nil, floor, t))
		for f > floor && dockNeed(h.Layouts.Docked, f, t) > h.Pane.Height {
			f--
		}
	}
	size := t.BodySize
	t.BodySize = f
	t.LabelW = t.LabelW * f / size
	t.ValueW = t.ValueW * f / size
	h.Pane.TabW = h.Pane.TabW * f / size
	h.Text = t
	return h
}

// dockNeed is the pane height a docked tree needs at body size f: the
// player's tree, or the built-in one for nil.
func dockNeed(tree *config.HUDNode, f int, t config.HUDText) int {
	t.BodySize = f
	m := MetricsFor(t)
	room := dockLines*m.BodyH + dockRules*m.GapH
	if tree != nil {
		room = Tallest(*tree, m)
	}
	return int(math.Ceil(float64(room))) + DockMargin
}

// Tallest is the tree's height at m for the target that needs the most: the
// height of Prune(n, w) for the tallest w, without building the pruned trees.
func Tallest(n config.HUDNode, m Metrics) float32 {
	var h float32
	for w := WhoColonist; w <= WhoWild; w++ {
		x, _, _ := prunedHeight(n, m, w, false)
		h = max(h, x)
	}
	return h
}

// prunedHeight is Measure(Prune(n, w), m, 0); real is false for a blank a line
// keeps, ok false for a node Prune drops.
func prunedHeight(n config.HUDNode, m Metrics, w Who, inLine bool) (h float32, real, ok bool) {
	if !WhoMatches(n.Who, w) {
		if inLine && IsWidget(n.Kind) && n.Kind != KeyGap {
			return WidgetHeight(n.Kind, "", m), false, true
		}
		return 0, false, false
	}
	if !IsContainer(n.Kind) {
		return WidgetHeight(n.Kind, n.Def, m), !isBlank(n), true
	}
	var sum, tallest float32
	reals := 0
	for _, k := range n.Kids {
		kh, kr, kok := prunedHeight(k, m, w, n.Kind == KindLine)
		if !kok {
			continue
		}
		if kr {
			reals++
		}
		sum += kh
		tallest = max(tallest, kh)
	}
	if len(n.Kids) > 0 && reals == 0 {
		return 0, false, false
	}
	if n.Kind == KindStrip || n.Kind == KindLine {
		return tallest, true, true
	}
	return sum, true, true
}
