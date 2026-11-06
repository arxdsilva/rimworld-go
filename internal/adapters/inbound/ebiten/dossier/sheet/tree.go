package sheet

import (
	"errors"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"strconv"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/config"
)

// Keys the tree code itself needs.
const (
	KeyGap   = "gap"
	KeyRule  = "rule"
	KeyTitle = "title"
)

// WhoLetters are the target letters in stored order: colonist, stranger,
// player's creature, wild creature.
const WhoLetters = "cspw"

// Letter is the target's letter in a node's Who attribute.
func (w Who) Letter() byte {
	if int(w) >= len(WhoLetters) {
		return WhoLetters[0]
	}
	return WhoLetters[w]
}

// WhoOf is the target kind of a person. A prisoner, a visitor, an ally and a
// raider are all strangers to the card: only the colony's own are colonists.
func WhoOf(p game.PawnView) Who {
	if p.Hostile || p.Prisoner || p.Visitor || p.Ally {
		return WhoStranger
	}
	return WhoColonist
}

// WhoOfAnimal is the target kind of a creature: a tame one is the player's
// ('p'), anything else, mechs too, is wild ('w').
func WhoOfAnimal(a game.AnimalView) Who {
	if a.Tame {
		return WhoPrisoner
	}
	return WhoWild
}

// WhoMatches reports whether a node with the Who attribute shows for w; an
// empty attribute shows for all.
func WhoMatches(letters string, w Who) bool {
	return letters == "" || strings.IndexByte(letters, w.Letter()) >= 0
}

// IsContainer reports whether kind is column, strip, block or line.
func IsContainer(kind string) bool {
	switch kind {
	case KindColumn, KindStrip, KindBlock, KindLine:
		return true
	}
	return false
}

// IsWidget reports whether key is a value widget's key.
func IsWidget(key string) bool {
	switch key {
	case "gap", "rule", "title", "diet_pick", "clothes_pick", "area_pick", "hour_pick",
		"standing", "identity", "vigour_bar", "body_state", "mood_bar", "mind_state", "hunger_bar",
		"rest_bar", "fun_bar", "need_bar", "skill", "train", "stat", "tally", "keeper", "doing",
		"next_up", "in_hand", "hauling", "traits", "notes", "captive":
		return true
	}
	return false
}

// NeedsDef reports whether a widget key takes a definition (skill, stat...).
func NeedsDef(key string) bool {
	switch key {
	case "skill", "train", "stat", "tally", "need_bar":
		return true
	}
	return false
}

// IsBar reports whether a widget key draws a bar and so takes a bar style.
func IsBar(key string) bool {
	switch key {
	case "vigour_bar", "mood_bar", "hunger_bar", "rest_bar", "fun_bar", "need_bar":
		return true
	}
	return false
}

// CanHold reports whether a node of kind parent may hold a node of kind
// child: a block holds lines only, a line holds widgets only, a widget holds
// nothing, a column or strip holds anything.
func CanHold(parent, child string) bool {
	switch parent {
	case KindColumn, KindStrip:
		return IsContainer(child) || IsWidget(child)
	case KindBlock:
		return child == KindLine
	case KindLine:
		return IsWidget(child)
	}
	return false
}

// Prune is the tree as one target sees it. A node whose Who excludes the
// target is gone with everything under it; a widget in a line keeps its slot
// as a blank gap carrying its key in Def, so a line's columns hold still; a
// container left with only blanks goes too. A root that does not match
// returns the zero node. The input is not changed.
func Prune(n config.HUDNode, w Who) config.HUDNode {
	out, _ := prune(n, w, false)
	return out
}

func isBlank(n config.HUDNode) bool { return n.Kind == KeyGap && n.Def != "" }

func prune(n config.HUDNode, w Who, inLine bool) (config.HUDNode, bool) {
	if !WhoMatches(n.Who, w) {
		if inLine && IsWidget(n.Kind) && n.Kind != KeyGap {
			return config.HUDNode{Kind: KeyGap, Def: n.Kind}, true
		}
		return config.HUDNode{}, false
	}
	if !IsContainer(n.Kind) {
		return n, true
	}
	out := n
	out.Kids = nil
	real := 0
	for _, k := range n.Kids {
		pk, ok := prune(k, w, n.Kind == KindLine)
		if !ok {
			continue
		}
		if !isBlank(pk) {
			real++
		}
		out.Kids = append(out.Kids, pk)
	}
	if len(n.Kids) > 0 && real == 0 {
		return config.HUDNode{}, false
	}
	return out, true
}

// Metrics from the text style: the sizes Measure needs.
func MetricsFor(t config.HUDText) Metrics {
	line := func(size, pct int) float32 { return float32((size + 6) * pct / 100) }
	body := line(t.BodySize, t.BodyLinePct)
	return Metrics{
		BodyH:  body,
		LargeH: line(t.BodySize+t.LargeDelta, t.LargeLinePct),
		SmallH: line(t.BodySize+t.SmallDelta, t.SmallLinePct),
		BarH:   body,
		GapH:   float32(max(2, t.BodySize/3)),
		LabelW: float32(t.LabelW),
		ValueW: float32(t.ValueW),
	}
}

// WidgetHeight is one widget's height. A gap carrying a key (a blank left by
// Prune) is as tall as that widget.
func WidgetHeight(key, def string, m Metrics) float32 {
	switch key {
	case KeyGap:
		if def != "" && def != KeyGap {
			return WidgetHeight(def, "", m)
		}
		return m.GapH
	case KeyRule:
		return m.GapH
	case KeyTitle:
		return m.LargeH
	case "vigour_bar", "mood_bar", "hunger_bar", "rest_bar", "fun_bar", "need_bar":
		return m.BarH
	}
	return m.BodyH
}

// Measure is the height of a node when avail tall is offered: the sum of a
// column's or block's children, the tallest of a strip's or line's. A node
// with Fill grows to avail when avail is larger.
func Measure(n config.HUDNode, m Metrics, avail float32) float32 {
	return MeasureWith(n, m, avail, nil)
}

// MeasureWith is Measure where empty reports a widget with no value to show:
// it takes no height, and a container of only such widgets takes none.
func MeasureWith(n config.HUDNode, m Metrics, avail float32, empty func(config.HUDNode) bool) float32 {
	h := natural(n, m, empty)
	if n.Fill && IsContainer(n.Kind) && avail > h {
		return avail
	}
	return h
}

func natural(n config.HUDNode, m Metrics, empty func(config.HUDNode) bool) float32 {
	if !IsContainer(n.Kind) {
		if empty != nil && !isBlank(n) && empty(n) {
			return 0
		}
		return WidgetHeight(n.Kind, n.Def, m)
	}
	var sum, tallest float32
	for _, k := range n.Kids {
		h := natural(k, m, empty)
		sum += h
		tallest = max(tallest, h)
	}
	if n.Kind == KindStrip || n.Kind == KindLine {
		return tallest
	}
	return sum
}

// Heights is the height of each child of container n inside avail tall. A
// column or block gives each child its own height and shares what is left
// evenly among the children with Fill; a strip or line gives every child the
// common height.
func Heights(n config.HUDNode, m Metrics, avail float32, empty func(config.HUDNode) bool) []float32 {
	hs := make([]float32, len(n.Kids))
	var sum float32
	fills := 0
	for i, k := range n.Kids {
		hs[i] = natural(k, m, empty)
		sum += hs[i]
		if k.Fill && IsContainer(k.Kind) {
			fills++
		}
	}
	if n.Kind == KindStrip || n.Kind == KindLine {
		h := max(natural(n, m, empty), avail)
		for i := range hs {
			hs[i] = h
		}
		return hs
	}
	if extra := avail - sum; extra > 0 && fills > 0 {
		for i, k := range n.Kids {
			if k.Fill && IsContainer(k.Kind) {
				hs[i] += extra / float32(fills)
			}
		}
	}
	return hs
}

// Count is the number of nodes in the tree, and its depth (a lone node is 1).
func Count(n config.HUDNode) (nodes, depth int) {
	nodes, depth = 1, 1
	for _, k := range n.Kids {
		c, d := Count(k)
		nodes += c
		depth = max(depth, d+1)
	}
	return nodes, depth
}

// Validate lists what is wrong with a stored tree. A tree with no problems
// is safe to measure and draw.
func Validate(tr lang.Tr, n config.HUDNode) []Problem {
	var out []Problem
	if nodes, depth := Count(n); nodes > config.HUDMaxNodes || depth > config.HUDMaxDepth {
		out = append(out, Problem{Path: "root", Text: tr.T("the tree is bigger than a card can hold")})
		return out
	}
	validate(tr, n, "root", &out)
	return out
}

func validate(tr lang.Tr, n config.HUDNode, path string, out *[]Problem) {
	bad := func(text string) { *out = append(*out, Problem{Path: path, Text: text}) }
	switch {
	case IsContainer(n.Kind):
	case IsWidget(n.Kind):
		if len(n.Kids) > 0 {
			bad(lang.Tf(tr, "%s cannot hold anything", n.Kind))
		}
	case n.Kind == "":
		bad(tr.T("an element with no name"))
	default:
		bad(lang.Tf(tr, "unknown element %s", strconv.Quote(n.Kind)))
	}
	if strings.Trim(n.Who, WhoLetters) != "" || hasRepeat(n.Who) {
		bad(tr.T("targets must be some of c s p w, each once"))
	}
	if NeedsDef(n.Kind) {
		if _, ok := LookupDef(tr, n.Kind, n.Def); !ok {
			bad(lang.Tf(tr, "%s needs a known definition, not %s", n.Kind, strconv.Quote(n.Def)))
		}
	}
	if n.BarStyle != "" && (!IsBar(n.Kind) || !ValidBarStyle(n.BarStyle)) {
		bad(lang.Tf(tr, "bar style %s does not fit here", strconv.Quote(n.BarStyle)))
	}
	if n.Fill && !IsContainer(n.Kind) {
		bad(tr.T("only a column, strip or block can fill"))
	}
	for i, k := range n.Kids {
		kp := path + "." + strconv.Itoa(i)
		if IsContainer(n.Kind) && !CanHold(n.Kind, k.Kind) && (IsContainer(k.Kind) || IsWidget(k.Kind)) {
			*out = append(*out, Problem{Path: kp, Text: lang.Tf(tr, "%s cannot sit in a %s", k.Kind, n.Kind)})
		}
		validate(tr, k, kp, out)
	}
}

func hasRepeat(s string) bool {
	for i := 0; i < len(s); i++ {
		if strings.IndexByte(s[i+1:], s[i]) >= 0 {
			return true
		}
	}
	return false
}

// Errors from editing a tree.
var (
	ErrNoSuchNode = errors.New("no such element")
	ErrCannotHold = errors.New("that element cannot hold this one")
	ErrLastTarget = errors.New("an element must show for at least one kind of target")
)

// At is the node at path (child indexes from the root), or nil.
func At(root *config.HUDNode, path []int) *config.HUDNode {
	n := root
	for _, i := range path {
		if n == nil || i < 0 || i >= len(n.Kids) {
			return nil
		}
		n = &n.Kids[i]
	}
	return n
}

// Move shifts the node at path by delta places among its siblings (-1 up, +1
// down) and returns its new path. The root cannot move; a move past either
// end does nothing.
func Move(root *config.HUDNode, path []int, delta int) ([]int, bool) {
	if len(path) == 0 {
		return path, false
	}
	parent := At(root, path[:len(path)-1])
	i := path[len(path)-1]
	j := i + delta
	if parent == nil || i < 0 || i >= len(parent.Kids) || j < 0 || j >= len(parent.Kids) || delta == 0 {
		return path, false
	}
	parent.Kids[i], parent.Kids[j] = parent.Kids[j], parent.Kids[i]
	np := append(append([]int(nil), path[:len(path)-1]...), j)
	return np, true
}

// Remove deletes the node at path. The root cannot be removed.
func Remove(root *config.HUDNode, path []int) bool {
	if len(path) == 0 {
		return false
	}
	parent := At(root, path[:len(path)-1])
	i := path[len(path)-1]
	if parent == nil || i < 0 || i >= len(parent.Kids) {
		return false
	}
	parent.Kids = append(parent.Kids[:i:i], parent.Kids[i+1:]...)
	return true
}

// NewNode is a fresh node for a catalog key: a container empty, a widget
// with the first definition of its kind when it needs one.
func NewNode(key string) config.HUDNode {
	n := config.HUDNode{Kind: key}
	if NeedsDef(key) {
		if ds := Defs(lang.English{}, key); len(ds) > 0 {
			n.Def = ds[0].Key
		}
	}
	return n
}

// Add puts node n into the container at parent, at index at (past the end
// appends), and returns its path.
func Add(root *config.HUDNode, parent []int, at int, n config.HUDNode) ([]int, error) {
	p := At(root, parent)
	if p == nil {
		return nil, ErrNoSuchNode
	}
	if !CanHold(p.Kind, n.Kind) {
		return nil, ErrCannotHold
	}
	if at < 0 || at > len(p.Kids) {
		at = len(p.Kids)
	}
	p.Kids = append(p.Kids, config.HUDNode{})
	copy(p.Kids[at+1:], p.Kids[at:])
	p.Kids[at] = n
	return append(append([]int(nil), parent...), at), nil
}

// WhoSet is the letters a node shows for, in stored order; an empty Who is
// all four.
func WhoSet(n config.HUDNode) string {
	if n.Who == "" {
		return WhoLetters
	}
	var b strings.Builder
	for i := 0; i < len(WhoLetters); i++ {
		if strings.IndexByte(n.Who, WhoLetters[i]) >= 0 {
			b.WriteByte(WhoLetters[i])
		}
	}
	return b.String()
}

// SetWho turns one target on or off for a node. It refuses to leave the node
// with no target (ErrLastTarget); all four on is stored as "".
func SetWho(n *config.HUDNode, w Who, on bool) error {
	set := WhoSet(*n)
	has := strings.IndexByte(set, w.Letter()) >= 0
	switch {
	case on && !has:
		set += string(w.Letter())
	case !on && has:
		set = strings.Replace(set, string(w.Letter()), "", 1)
	}
	if set == "" {
		return ErrLastTarget
	}
	probe := config.HUDNode{Who: set}
	var b strings.Builder
	for i := 0; i < len(WhoLetters); i++ {
		if strings.IndexByte(probe.Who, WhoLetters[i]) >= 0 {
			b.WriteByte(WhoLetters[i])
		}
	}
	if b.String() == WhoLetters {
		n.Who = ""
	} else {
		n.Who = b.String()
	}
	return nil
}
