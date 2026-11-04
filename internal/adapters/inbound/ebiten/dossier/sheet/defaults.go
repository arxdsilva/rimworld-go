package sheet

import "rimworld-go/internal/platform/config"

func widget(key string) config.HUDNode         { return config.HUDNode{Kind: key} }
func widgetFor(key, who string) config.HUDNode { return config.HUDNode{Kind: key, Who: who} }
func generic(key, def string) config.HUDNode   { return config.HUDNode{Kind: key, Def: def} }
func box(kind string, kids ...config.HUDNode) config.HUDNode {
	return config.HUDNode{Kind: kind, Kids: kids}
}
func boxFor(kind, who string, kids ...config.HUDNode) config.HUDNode {
	return config.HUDNode{Kind: kind, Who: who, Kids: kids}
}

// skillBlock is the eleven skills as five rows of two and a last row that
// pairs the odd skill with the work speed it feeds, so nothing sits beside a
// blank.
func skillBlock() config.HUDNode {
	pair := func(a, b string) config.HUDNode {
		return box(KindLine, generic("skill", a), generic("skill", b))
	}
	return boxFor(KindBlock, "c",
		pair("shooting", "melee"), pair("construction", "mining"), pair("cooking", "plants"),
		pair("animals", "crafting"), pair("medicine", "social"),
		box(KindLine, generic("skill", "intellectual"), generic("stat", "work_speed")))
}

func trainRows() config.HUDNode {
	return boxFor(KindColumn, "p", generic("train", "obedience"), generic("train", "guard"), generic("train", "release"))
}

func barsColumn() config.HUDNode {
	return box(KindColumn, widget("vigour_bar"), widgetFor("mood_bar", "cs"), widget("hunger_bar"),
		widget("rest_bar"), widgetFor("fun_bar", "c"))
}

func rightColumn(withHauling bool) config.HUDNode {
	kids := []config.HUDNode{widgetFor("area_pick", "cp"), widgetFor("hour_pick", "c"), widgetFor("diet_pick", "cp"),
		widget("doing"), widgetFor("next_up", "c"), widgetFor("in_hand", "cs"), widgetFor("keeper", "p")}
	if withHauling {
		kids = append(kids, widgetFor("hauling", "cs"))
	}
	return box(KindColumn, kids...)
}

func fill(n config.HUDNode) config.HUDNode { n.Fill = true; return n }

// DefaultDocked is the built-in arrangement for the inspect pane (the pane
// header already carries the name). Pruned for a target it stays within 300
// pixels at the default text size.
func DefaultDocked() config.HUDNode {
	return fill(box(KindColumn,
		widget("standing"), widget("identity"), widget("body_state"), widgetFor("mind_state", "cs"),
		widget("rule"),
		box(KindStrip, barsColumn(), rightColumn(false)),
		widgetFor("captive", "s"),
		widget("rule"),
		skillBlock(), trainRows()))
}

// DefaultFloating is the built-in arrangement for the floating card: the
// docked rows with the name on top, hauling, traits, records and notes.
func DefaultFloating() config.HUDNode {
	return fill(box(KindColumn,
		widget("title"),
		widget("standing"), widget("identity"), widget("body_state"), widgetFor("mind_state", "cs"),
		widget("rule"),
		box(KindStrip, barsColumn(), rightColumn(true)),
		widgetFor("captive", "s"),
		widget("rule"),
		skillBlock(), trainRows(),
		widgetFor("traits", "cs"),
		boxFor(KindLine, "c", generic("tally", "kills"), generic("tally", "damage")),
		widget("notes")))
}

// Default is the built-in tree for the pane or the card, cut to one target.
func Default(docked bool, w Who) config.HUDNode {
	if docked {
		return Prune(DefaultDocked(), w)
	}
	return Prune(DefaultFloating(), w)
}

// walk applies fn to every node, children first, rebuilding the tree.
func walk(n config.HUDNode, fn func(config.HUDNode) config.HUDNode) config.HUDNode {
	if len(n.Kids) > 0 {
		kids := make([]config.HUDNode, len(n.Kids))
		for i, k := range n.Kids {
			kids[i] = walk(k, fn)
		}
		n.Kids = kids
	}
	return fn(n)
}

// everyTarget removes every target filter: each row shows for every kind of
// target and collapses when it has nothing to say.
func everyTarget(n config.HUDNode) config.HUDNode {
	return walk(n, func(x config.HUDNode) config.HUDNode { x.Who = ""; return x })
}

// narrowed gives the named widgets new target filters.
func narrowed(n config.HUDNode, who map[string]string) config.HUDNode {
	return walk(n, func(x config.HUDNode) config.HUDNode {
		if w, ok := who[x.Kind]; ok {
			x.Who = w
		}
		return x
	})
}

// withTraits adds a traits row after the first rule of a column root.
func withTraits(n config.HUDNode) config.HUDNode {
	if n.Kind != KindColumn {
		return n
	}
	for i, k := range n.Kids {
		if k.Kind == KeyRule {
			kids := append([]config.HUDNode(nil), n.Kids[:i]...)
			kids = append(kids, widgetFor("traits", "cs"))
			n.Kids = append(kids, n.Kids[i:]...)
			return n
		}
	}
	return n
}
