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

// rightColumn is the floating card's state column.
func rightColumn() config.HUDNode {
	return box(KindColumn, widgetFor("area_pick", "cp"), widgetFor("hour_pick", "c"), widgetFor("diet_pick", "cp"),
		widget("doing"), widgetFor("next_up", "c"), widgetFor("in_hand", "cs"), widgetFor("keeper", "p"),
		widgetFor("hauling", "cs"))
}

func fill(n config.HUDNode) config.HUDNode { n.Fill = true; return n }

// compactRight is the docked card's state column: the selectors, what the
// creature does and holds, its master and its captivity. Each row is for the
// targets that can fill it, so no target gets more than six.
func compactRight() config.HUDNode {
	return box(KindColumn, widgetFor("area_pick", "cp"), widgetFor("hour_pick", "c"), widgetFor("diet_pick", "cp"),
		widget("doing"), widgetFor("next_up", "c"), widgetFor("in_hand", "cs"), widgetFor("keeper", "p"),
		widgetFor("captive", "s"))
}

// skillColumns are the eleven skills and the work speed as two columns of six.
func skillColumns() (a, b config.HUDNode) {
	return boxFor(KindColumn, "c", generic("skill", "shooting"), generic("skill", "construction"),
			generic("skill", "cooking"), generic("skill", "animals"), generic("skill", "medicine"),
			generic("skill", "intellectual")),
		boxFor(KindColumn, "c", generic("skill", "melee"), generic("skill", "mining"), generic("skill", "plants"),
			generic("skill", "crafting"), generic("skill", "social"), generic("stat", "work_speed"))
}

// DefaultDocked is the built-in arrangement for the inspect pane (the pane
// header already carries the name): two lines of who and how, a rule, and one
// strip of columns (needs, state, skills, training). Pruned for any target it is
// eight rows and a rule tall, 132 pixels at the default text.
func DefaultDocked() config.HUDNode {
	skillsA, skillsB := skillColumns()
	return fill(box(KindColumn,
		box(KindLine, widget("standing"), widget("identity")),
		box(KindLine, widget("body_state"), widgetFor("mind_state", "cs")),
		widget("rule"),
		box(KindStrip, barsColumn(), compactRight(), skillsA, skillsB, trainRows())))
}

// roomyDocked is the docked tree with every row for every target, but for the
// state column: its rows each fit one kind of target, and without their filters
// they would pass the pane's height.
func roomyDocked() config.HUDNode {
	n := everyTarget(DefaultDocked())
	strip := &n.Kids[len(n.Kids)-1]
	strip.Kids[1] = compactRight()
	return n
}

// DefaultFloating is the built-in arrangement for the floating card: the name
// on top, the docked card's rows in a taller order, hauling, traits, records and
// notes.
func DefaultFloating() config.HUDNode {
	return fill(box(KindColumn,
		widget("title"),
		widget("standing"), widget("identity"), widget("body_state"), widgetFor("mind_state", "cs"),
		widget("rule"),
		box(KindStrip, barsColumn(), rightColumn()),
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
