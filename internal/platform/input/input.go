// Package input is the client's key-binding registry: named actions, the
// context each belongs to, their default bindings, conflict checks and the
// resolver that turns one frame of key state into the actions that fired.
// It is pure Go (no Ebitengine), so the rules are tested without a window.
// Key names are Ebitengine's Key.String() names ("A", "Digit1", "Comma",
// "F10", "NumpadMultiply"), with the modifiers folded into "Shift",
// "Ctrl" and "Alt".
package input

import (
	"sort"
	"strings"
)

// Action names one thing the player can do with a key.
type Action string

// Mod is a set of held modifier keys.
type Mod uint8

// Modifier bits.
const (
	ModShift Mod = 1 << iota
	ModCtrl
	ModAlt
)

// Modifier key names.
const (
	KeyShift = "Shift"
	KeyCtrl  = "Ctrl"
	KeyAlt   = "Alt"
)

// modKeys maps a modifier key name to its bit.
var modKeys = map[string]Mod{KeyShift: ModShift, KeyCtrl: ModCtrl, KeyAlt: ModAlt}

// IsModifier reports whether name is a modifier key.
func IsModifier(name string) bool { _, ok := modKeys[name]; return ok }

// Binding is one key chord: a key plus the modifiers held with it. A
// Modifier action's binding is the modifier itself (Key "Shift", no Mods).
type Binding struct {
	Key  string
	Mods Mod
}

// String is the binding as stored in settings: "Shift+R", "Ctrl+Shift+F1".
func (b Binding) String() string {
	var parts []string
	if b.Mods&ModCtrl != 0 {
		parts = append(parts, KeyCtrl)
	}
	if b.Mods&ModShift != 0 {
		parts = append(parts, KeyShift)
	}
	if b.Mods&ModAlt != 0 {
		parts = append(parts, KeyAlt)
	}
	return strings.Join(append(parts, b.Key), "+")
}

// Label is the binding as shown to the player: "Shift+R", ",", "Numpad *".
func (b Binding) Label() string {
	s := b.String()
	return strings.TrimSuffix(s, b.Key) + KeyLabel(b.Key)
}

// ParseBinding reads a stored binding ("Shift+R"); false when it names no
// key or an unknown modifier.
func ParseBinding(s string) (Binding, bool) {
	parts := strings.Split(strings.TrimSpace(s), "+")
	var b Binding
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return Binding{}, false
		}
		if i == len(parts)-1 {
			b.Key = p
			break
		}
		m, ok := modKeys[p]
		if !ok {
			return Binding{}, false
		}
		b.Mods |= m
	}
	if IsModifier(b.Key) && b.Mods != 0 {
		return Binding{}, false // "Ctrl+Shift" is not a chord
	}
	return b, b.Key != ""
}

var keyLabels = map[string]string{
	"Comma": ",", "Period": ".", "Slash": "/", "Backquote": "`", "Quote": "'",
	"Equal": "=", "Minus": "-", "Semicolon": ";", "Backslash": "\\",
	"BracketLeft": "[", "BracketRight": "]", "Space": "Space",
	"NumpadMultiply": "Numpad *", "NumpadAdd": "Numpad +", "NumpadSubtract": "Numpad -",
	"NumpadDivide": "Numpad /", "NumpadEnter": "Numpad Enter",
	"ArrowLeft": "Left", "ArrowRight": "Right", "ArrowUp": "Up", "ArrowDown": "Down",
	"PageUp": "PgUp", "PageDown": "PgDn", "Escape": "Esc",
}

// KeyLabel is a key name as the player reads it ("Digit1" -> "1").
func KeyLabel(name string) string {
	if l, ok := keyLabels[name]; ok {
		return l
	}
	if d, ok := strings.CutPrefix(name, "Digit"); ok {
		return d
	}
	if d, ok := strings.CutPrefix(name, "Numpad"); ok && len(d) == 1 {
		return "Numpad " + d
	}
	return name
}

// Context is a layer of the input stack. When several are active the
// first one (in the order the client passes them) that binds a pressed key
// takes it, so one key never does two things.
type Context uint8

// Contexts, top of the stack first.
const (
	// CtxText is a focused text field or key capture: it swallows every
	// key (its owner reads the raw keys and typed characters).
	CtxText   Context = iota
	CtxModal          // pause menu, order menu, dialogs, search results
	CtxTool           // Move mode, an Architect tool or drag
	CtxPanel          // Work, Schedule, Animals, Research, Options
	CtxMap            // the colony map: selection, orders, overlays
	CtxGlobal         // camera, speed, tabs, screenshots
	CtxDev            // developer mode only
)

var contextNames = [...]string{"text", "modal", "tool", "panel", "map", "global", "dev"}

func (c Context) String() string {
	if int(c) < len(contextNames) {
		return contextNames[c]
	}
	return "?"
}

// alwaysOn reports a context whose keys sit under every other one: its
// bindings conflict with any context's.
func (c Context) alwaysOn() bool { return c == CtxGlobal || c == CtxDev }

// Kind is how an action reads its key.
type Kind uint8

// Kinds.
const (
	Press    Kind = iota // fires once when the key goes down
	Hold                 // active while the key is held (modifiers ignored)
	Toggle               // a press that flips something on and off
	Modifier             // a modifier held with a mouse action (Shift: queue)
)

// Def describes one action.
type Def struct {
	ID       Action
	Label    string
	Category string
	Context  Context
	Defaults []Binding
	Kind     Kind
	// Reserved is why the action does nothing yet ("not implemented yet");
	// it still fires so the client can say so.
	Reserved string
	// Fixed actions (the modifiers) are listed but not rebindable.
	Fixed bool
}

// Rebindable reports whether the player may change the action's keys.
func (d Def) Rebindable() bool { return !d.Fixed }

var defIndex = func() map[Action]int {
	m := make(map[Action]int, len(defs))
	for i, d := range defs {
		m[d.ID] = i
	}
	return m
}()

// Defs is every action in display order (grouped by category).
func Defs() []Def { return append([]Def(nil), defs...) }

// Lookup finds an action's definition.
func Lookup(a Action) (Def, bool) {
	i, ok := defIndex[a]
	if !ok {
		return Def{}, false
	}
	return defs[i], true
}

// Bindings are the keys bound to each action. A missing action is
// unbound.
type Bindings map[Action][]Binding

// Defaults are the bindings of a fresh install.
func Defaults() Bindings {
	b := make(Bindings, len(defs))
	for _, d := range defs {
		b[d.ID] = append([]Binding(nil), d.Defaults...)
	}
	return b
}

// Clone copies the bindings.
func (b Bindings) Clone() Bindings {
	out := make(Bindings, len(b))
	for a, ks := range b {
		out[a] = append([]Binding(nil), ks...)
	}
	return out
}

// Of is the keys bound to a.
func (b Bindings) Of(a Action) []Binding { return b[a] }

// Label is the first binding of a as shown to the player ("" when unbound).
func (b Bindings) Label(a Action) string {
	if ks := b[a]; len(ks) > 0 {
		return ks[0].Label()
	}
	return ""
}

// overlaps reports two actions' bindings would both claim the same key.
func overlaps(da, db Def, ka, kb Binding) bool {
	if ka.Key != kb.Key {
		return false
	}
	if da.Kind == Hold || db.Kind == Hold {
		return true // holds ignore modifiers
	}
	return ka.Mods == kb.Mods
}

// sharesLayer reports two contexts that can both see one key press.
func sharesLayer(a, b Context) bool {
	return a == b || a.alwaysOn() || b.alwaysOn()
}

// Conflict is two actions bound to the same key in overlapping contexts.
type Conflict struct {
	A, B Action
	Key  Binding
}

// Conflicts lists every pair of actions whose bindings collide: in the
// same context, or one of them global (or dev) against any context.
// Modifier actions share their keys by design and are never reported.
func Conflicts(b Bindings) []Conflict {
	var out []Conflict
	for i, da := range defs {
		if da.Kind == Modifier {
			continue
		}
		for _, db := range defs[i+1:] {
			if db.Kind == Modifier || !sharesLayer(da.Context, db.Context) {
				continue
			}
			for _, ka := range b[da.ID] {
				for _, kb := range b[db.ID] {
					if overlaps(da, db, ka, kb) {
						out = append(out, Conflict{A: da.ID, B: db.ID, Key: ka})
					}
				}
			}
		}
	}
	return out
}

// ConflictsWith lists the actions that would collide with a if it were
// bound to k (a itself excluded).
func (b Bindings) ConflictsWith(a Action, k Binding) []Action {
	da, ok := Lookup(a)
	if !ok || da.Kind == Modifier {
		return nil
	}
	var out []Action
	for _, db := range defs {
		if db.ID == a || db.Kind == Modifier || !sharesLayer(da.Context, db.Context) {
			continue
		}
		for _, kb := range b[db.ID] {
			if overlaps(da, db, k, kb) {
				out = append(out, db.ID)
				break
			}
		}
	}
	return out
}

// Set returns a copy with a's slot i bound to k (appending when i is past
// the end); a zero k clears the slot.
func (b Bindings) Set(a Action, i int, k Binding) Bindings {
	out := b.Clone()
	ks := out[a]
	switch {
	case k.Key == "" && i < len(ks):
		ks = append(ks[:i], ks[i+1:]...)
	case k.Key == "":
	case i < len(ks):
		ks[i] = k
	default:
		ks = append(ks, k)
	}
	out[a] = dedupe(ks)
	return out
}

// Swap binds a's slot i to k and hands a's previous key in that slot (if
// any) to every action that held k, so the two trade keys.
func (b Bindings) Swap(a Action, i int, k Binding) Bindings {
	var old Binding
	if ks := b[a]; i < len(ks) {
		old = ks[i]
	}
	out := b.Set(a, i, k)
	for _, o := range b.ConflictsWith(a, k) {
		ks := out[o]
		for j, kb := range ks {
			if kb.Key == k.Key {
				if old.Key == "" {
					ks = append(ks[:j], ks[j+1:]...)
				} else {
					ks[j] = old
				}
				break
			}
		}
		out[o] = dedupe(ks)
	}
	return out
}

func dedupe(ks []Binding) []Binding {
	out := ks[:0:0]
	seen := map[Binding]bool{}
	for _, k := range ks {
		if k.Key == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

func equal(a, b []Binding) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// FromSettings rebuilds bindings from the stored overrides (action id ->
// binding strings): every action starts at its defaults, unknown or fixed
// actions and unreadable keys are dropped, duplicates removed. An empty
// list unbinds the action.
func FromSettings(m map[string][]string) Bindings {
	b := Defaults()
	for id, strs := range m {
		d, ok := Lookup(Action(id))
		if !ok || d.Fixed {
			continue
		}
		var ks []Binding
		for _, s := range strs {
			if k, ok := ParseBinding(s); ok && !IsModifier(k.Key) {
				ks = append(ks, k)
			}
		}
		b[d.ID] = dedupe(ks)
	}
	return b
}

// ToSettings is the overrides to store: only actions whose keys differ
// from the defaults (an unbound action stores an empty list). Nil when
// everything is default.
func ToSettings(b Bindings) map[string][]string {
	var out map[string][]string
	for _, d := range defs {
		ks := dedupe(b[d.ID])
		if d.Fixed || equal(ks, d.Defaults) {
			continue
		}
		if out == nil {
			out = map[string][]string{}
		}
		strs := make([]string, 0, len(ks))
		for _, k := range ks {
			strs = append(strs, k.String())
		}
		out[string(d.ID)] = strs
	}
	return out
}

// Normalize cleans stored overrides: unknown actions and keys dropped,
// duplicates removed, entries equal to the defaults left out.
func Normalize(m map[string][]string) map[string][]string { return ToSettings(FromSettings(m)) }

// Frame is one frame of key state.
type Frame struct {
	Pressed []string // keys that went down this frame, in a stable order
	Held    []string // keys down now (pressed ones included)
	Mods    Mod      // modifiers held
}

// Result is what fired in one frame.
type Result struct {
	fired map[Action]bool
	held  map[Action]bool
}

// Fired reports a Press or Toggle action fired this frame.
func (r Result) Fired(a Action) bool { return r.fired[a] }

// Held reports a Hold or Modifier action is active.
func (r Result) Held(a Action) bool { return r.held[a] }

// FiredList is the fired actions in definition order.
func (r Result) FiredList() []Action {
	out := make([]Action, 0, len(r.fired))
	for a := range r.fired {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return defIndex[out[i]] < defIndex[out[j]] })
	return out
}

// Resolve turns a frame into actions for the active context stack (top
// first). Each pressed key goes to the first context that binds it (a
// press with exactly the held modifiers, or a hold on that key), so it
// fires at most one action; a text context on top swallows everything.
// Holds are active in every listed context; modifiers whenever held.
func Resolve(f Frame, b Bindings, stack []Context) Result {
	r := Result{fired: map[Action]bool{}, held: map[Action]bool{}}
	if len(stack) == 0 || stack[0] == CtxText {
		return r
	}
	active := map[Context]bool{}
	for _, c := range stack {
		active[c] = true
	}
	held := map[string]bool{}
	for _, k := range f.Held {
		held[k] = true
	}
	for _, d := range defs {
		if !active[d.Context] {
			continue
		}
		for _, k := range b[d.ID] {
			switch {
			case d.Kind == Modifier:
				if m := modKeys[k.Key]; m != 0 && f.Mods&m != 0 {
					r.held[d.ID] = true
				}
			case d.Kind == Hold && held[k.Key]:
				r.held[d.ID] = true
			}
		}
	}
	for _, key := range f.Pressed {
		if IsModifier(key) {
			continue
		}
	stack:
		for _, c := range stack {
			for _, d := range defs {
				if d.Context != c || d.Kind == Modifier {
					continue
				}
				for _, k := range b[d.ID] {
					if k.Key != key {
						continue
					}
					if d.Kind == Hold {
						break stack // the key belongs to a hold here
					}
					if k.Mods == f.Mods {
						r.fired[d.ID] = true
						break stack
					}
				}
			}
		}
	}
	return r
}
