// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modding

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"rimworld-go/internal/application/pluginapi"
)

const manifestFile = "plugin.json"

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{1,47}$`)

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// dotted reports whether s is min..max digit groups separated by dots.
func dotted(s string, min, max int) bool {
	parts := strings.Split(s, ".")
	if len(parts) < min || len(parts) > max {
		return false
	}
	for _, p := range parts {
		if !isDigits(p) {
			return false
		}
	}
	return true
}

// CheckManifest reports what is wrong with one manifest (PLG-001): a
// missing or malformed id, the reserved id "core", a missing name or
// version, and malformed entries in requires/after/before/conflicts.
func CheckManifest(m Manifest) []Problem {
	var out []Problem
	idOK := idPattern.MatchString(m.ID)
	plugin := ""
	if idOK {
		plugin = m.ID
	}
	add := func(field, msg string) {
		out = append(out, Problem{Plugin: plugin, File: manifestFile, Field: field, Msg: msg})
	}
	switch {
	case !idOK:
		add("id", "id must be 2–48 lowercase letters, digits or _")
	case m.ID == Core:
		add("id", `id "core" is reserved for the game itself`)
	}
	switch {
	case m.Name == "":
		add("name", "name is required")
	case utf8.RuneCountInString(m.Name) > 80:
		add("name", "name must be at most 80 characters")
	}
	switch {
	case m.Version == "":
		add("version", "version is required")
	case !dotted(m.Version, 1, 3):
		add("version", "version must be numbers separated by dots, like 1.0 or 1.2.3")
	}
	for i, g := range m.Game {
		if !dotted(g, 2, 2) {
			add(fmt.Sprintf("game[%d]", i), fmt.Sprintf("game version %q must look like 1.0 (major.minor)", g))
		}
	}
	checkIDList := func(field string, list []string, allowCore bool) {
		seen := map[string]bool{}
		for i, e := range list {
			f := fmt.Sprintf("%s[%d]", field, i)
			switch {
			case !idPattern.MatchString(e):
				add(f, fmt.Sprintf("%q is not a valid plugin id", e))
			case e == Core && !allowCore:
				add(f, fmt.Sprintf("%s cannot name core: the game itself always loads first and cannot be a conflict", field))
			case idOK && e == m.ID:
				add(f, fmt.Sprintf("%s cannot name the plugin itself", field))
			case seen[e]:
				add(f, fmt.Sprintf("%q is listed twice", e))
			}
			seen[e] = true
		}
	}
	checkIDList("requires", m.Requires, true)
	checkIDList("after", m.After, true)
	checkIDList("before", m.Before, false)
	checkIDList("conflicts", m.Conflicts, false)
	if m.URL != "" && !strings.HasPrefix(m.URL, "https://") && !strings.HasPrefix(m.URL, "http://") {
		add("url", "url must start with https:// or http://")
	}
	out = append(out, checkAPI(m, plugin)...)
	return out
}

// checkAPI checks the plugin API version and capability names (PLG-api).
func checkAPI(m Manifest, plugin string) []Problem {
	var out []Problem
	add := func(field, msg string, warn bool) {
		out = append(out, Problem{Plugin: plugin, File: manifestFile, Field: field, Msg: msg, Warn: warn})
	}
	switch {
	case m.API == 0:
		add("api", "declares no plugin API version; treated as 1", true)
	case m.API > pluginapi.Version:
		add("api", fmt.Sprintf("needs plugin API %d; this game has %d", m.API, pluginapi.Version), false)
	case m.API < pluginapi.MinVersion:
		add("api", fmt.Sprintf("plugin API %d is not supported; this game runs %d to %d", m.API, pluginapi.MinVersion, pluginapi.Version), false)
	}
	_, unknown, dup := pluginapi.Parse(m.Capabilities)
	for _, n := range unknown {
		add("capabilities", fmt.Sprintf("unknown capability %q", n), false)
	}
	for _, n := range dup {
		add("capabilities", fmt.Sprintf("capability %q is listed twice", n), true)
	}
	return out
}

// Edge kinds, strongest first (a pair listed twice keeps the strongest).
const (
	edgeRequires = iota + 1
	edgeAfter
	edgeBefore
)

// Problem rule sequence, the tie-break after the plugin id.
const (
	seqDup = iota
	seqMissing
	seqManifest
	seqRequires
	seqConflict
	seqLoop
	seqVersion
)

type seqProblem struct {
	Problem
	seq int
}

type resolver struct {
	byID  map[string]Manifest
	dup   map[string]bool
	alive map[string]bool
	probs []seqProblem
}

func (r *resolver) add(seq int, p Problem) {
	if p.File == "" {
		p.File = manifestFile
	}
	r.probs = append(r.probs, seqProblem{p, seq})
}

func (r *resolver) aliveIDs() []string {
	ids := make([]string, 0, len(r.alive))
	for id := range r.alive {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// cascadeRequires fails every plugin whose requirement is not loading,
// until nothing changes.
func (r *resolver) cascadeRequires(enabled map[string]bool) {
	for removed := true; removed; {
		removed = false
		for _, id := range r.aliveIDs() {
			bad := false
			for i, req := range r.byID[id].Requires {
				if req == Core || r.alive[req] {
					continue
				}
				bad = true
				why := "which cannot load"
				if _, ok := r.byID[req]; !ok {
					why = "which is not installed"
				} else if !enabled[req] {
					why = "which is not enabled"
				}
				r.add(seqRequires, Problem{Plugin: id, Field: fmt.Sprintf("requires[%d]", i),
					Msg: fmt.Sprintf("requires %s, %s", req, why)})
			}
			if bad {
				delete(r.alive, id)
				removed = true
			}
		}
	}
}

// failConflicts fails both sides of every conflict between loading
// plugins; it reports whether anything failed.
func (r *resolver) failConflicts() bool {
	partners := map[string]map[string]bool{}
	link := func(a, b string) {
		if partners[a] == nil {
			partners[a] = map[string]bool{}
		}
		partners[a][b] = true
	}
	for _, id := range r.aliveIDs() {
		for _, c := range r.byID[id].Conflicts {
			if c != id && r.alive[c] {
				link(id, c)
				link(c, id)
			}
		}
	}
	ids := make([]string, 0, len(partners))
	for id := range partners {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		others := make([]string, 0, len(partners[id]))
		for o := range partners[id] {
			others = append(others, o)
		}
		sort.Strings(others)
		for _, o := range others {
			r.add(seqConflict, Problem{Plugin: id, Field: "conflicts",
				Msg: fmt.Sprintf("conflicts with %s; disable one of them", o)})
		}
	}
	for _, id := range ids {
		delete(r.alive, id)
	}
	return len(ids) > 0
}

// majorMinor reduces "1.2.3-dev" (or a release tag "v1.2.3") to "1.2";
// "" when it is not a version.
func majorMinor(v string) string {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 || !isDigits(parts[0]) || !isDigits(parts[1]) {
		return ""
	}
	return parts[0] + "." + parts[1]
}

// Resolve orders the enabled plugins among found (PLG-010, PLG-011).
// gameVersion is "major.minor[.patch]". Core is always Loaded[0].
func Resolve(found []Manifest, enabled []string, gameVersion string) (Order, []Problem) {
	r := &resolver{byID: map[string]Manifest{}, dup: map[string]bool{}, alive: map[string]bool{}}

	sorted := append([]Manifest(nil), found...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	for _, m := range sorted {
		if _, ok := r.byID[m.ID]; ok {
			if !r.dup[m.ID] {
				r.dup[m.ID] = true
				r.add(seqDup, Problem{Plugin: m.ID, Msg: "installed twice; remove one copy"})
			}
			continue
		}
		r.byID[m.ID] = m
	}

	enabledSet := map[string]bool{}
	for _, id := range enabled {
		if id != Core {
			enabledSet[id] = true
		}
	}
	ens := make([]string, 0, len(enabledSet))
	for id := range enabledSet {
		ens = append(ens, id)
	}
	sort.Strings(ens)

	for _, id := range ens {
		m, ok := r.byID[id]
		if !ok {
			r.add(seqMissing, Problem{Plugin: id, Msg: "enabled but not installed"})
			continue
		}
		if r.dup[id] {
			continue
		}
		hard := false
		for _, p := range CheckManifest(m) {
			r.add(seqManifest, p)
			hard = hard || !p.Warn
		}
		if !hard {
			r.alive[id] = true
		}
	}

	for {
		r.cascadeRequires(enabledSet)
		if !r.failConflicts() {
			break
		}
	}

	placed := r.order()

	loaded := map[string]bool{}
	for _, p := range placed {
		loaded[p.ID] = true
	}
	gv := majorMinor(gameVersion)
	for _, p := range placed {
		m := r.byID[p.ID]
		switch {
		case len(m.Game) == 0:
			r.add(seqVersion, Problem{Plugin: p.ID, Field: "game", Warn: true, Msg: "declares no supported game version"})
		case gv != "" && !contains(m.Game, gv):
			r.add(seqVersion, Problem{Plugin: p.ID, Field: "game", Warn: true,
				Msg: fmt.Sprintf("made for game %s; this is %s", strings.Join(m.Game, ", "), gv)})
		}
	}

	ord := Order{Loaded: append([]Placed{{ID: Core, Reason: "the game itself"}}, placed...)}
	for _, id := range ens {
		if !loaded[id] {
			ord.Failed = append(ord.Failed, id)
		}
	}

	sort.SliceStable(r.probs, func(i, j int) bool {
		a, b := r.probs[i], r.probs[j]
		if a.Plugin != b.Plugin {
			return a.Plugin < b.Plugin
		}
		return a.seq < b.seq
	})
	var probs []Problem
	for _, p := range r.probs {
		probs = append(probs, p.Problem)
	}
	return ord, probs
}

func contains(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

// order runs Kahn's algorithm over the loading plugins, smallest id
// first, and fails whatever a load order loop leaves behind.
func (r *resolver) order() []Placed {
	ids := r.aliveIDs()
	pred := map[string]map[string]int{} // x -> predecessor -> edge kind
	corePred := map[string]int{}
	edge := func(x, y string, kind int) {
		if y == Core {
			if k, ok := corePred[x]; !ok || kind < k {
				corePred[x] = kind
			}
			return
		}
		if pred[x] == nil {
			pred[x] = map[string]int{}
		}
		if k, ok := pred[x][y]; !ok || kind < k {
			pred[x][y] = kind
		}
	}
	for _, x := range ids {
		m := r.byID[x]
		for _, y := range m.Requires {
			edge(x, y, edgeRequires)
		}
		for _, y := range m.After {
			if y == Core || r.alive[y] {
				edge(x, y, edgeAfter)
			}
		}
		for _, y := range m.Before {
			if r.alive[y] {
				edge(y, x, edgeBefore)
			}
		}
	}

	pos := map[string]int{Core: 0}
	var placed []Placed
	for len(placed) < len(ids) {
		next := ""
		for _, x := range ids {
			if _, done := pos[x]; done {
				continue
			}
			ready := true
			for y := range pred[x] {
				if _, done := pos[y]; !done {
					ready = false
					break
				}
			}
			if ready {
				next = x
				break
			}
		}
		if next == "" {
			break
		}
		pos[next] = len(placed) + 1
		placed = append(placed, Placed{ID: next, Reason: reason(next, pred[next], corePred, pos)})
	}

	var rest []string
	for _, x := range ids {
		if _, done := pos[x]; !done {
			rest = append(rest, x)
		}
	}
	r.failLoops(rest, pred)
	return placed
}

func reason(x string, preds map[string]int, corePred map[string]int, pos map[string]int) string {
	best, kind := "", 0
	for y, k := range preds {
		if best == "" || pos[y] > pos[best] {
			best, kind = y, k
		}
	}
	if best == "" {
		k, ok := corePred[x]
		if !ok {
			return "alphabetical"
		}
		best, kind = Core, k
	}
	switch kind {
	case edgeRequires:
		return fmt.Sprintf("after %s (requires it)", best)
	case edgeAfter:
		return fmt.Sprintf("after %s (asks to load after it)", best)
	}
	return fmt.Sprintf("after %s (%s asks to load before it)", best, best)
}

// failLoops reports every plugin in rest: members of a loop name it,
// the others name a blocker.
func (r *resolver) failLoops(rest []string, pred map[string]map[string]int) {
	if len(rest) == 0 {
		return
	}
	inRest := map[string]bool{}
	for _, x := range rest {
		inRest[x] = true
	}
	succ := map[string][]string{}
	for _, x := range rest {
		for y := range pred[x] {
			if inRest[y] {
				succ[y] = append(succ[y], x)
			}
		}
	}
	for _, s := range succ {
		sort.Strings(s)
	}
	reach := map[string]map[string]bool{}
	for _, x := range rest {
		seen := map[string]bool{}
		stack := []string{x}
		for len(stack) > 0 {
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, s := range succ[n] {
				if !seen[s] {
					seen[s] = true
					stack = append(stack, s)
				}
			}
		}
		reach[x] = seen
	}

	onLoop := map[string]bool{}
	for _, x := range rest {
		if onLoop[x] || !reach[x][x] {
			continue
		}
		var members []string
		for _, u := range rest {
			if u == x || (reach[x][u] && reach[u][x]) {
				members = append(members, u)
			}
		}
		msg := "load order loop: " + strings.Join(loopFrom(x, succ, members), " → ")
		for _, u := range members {
			onLoop[u] = true
			r.add(seqLoop, Problem{Plugin: u, Msg: msg})
		}
	}
	for _, x := range rest {
		if onLoop[x] {
			continue
		}
		blocker := ""
		for y := range pred[x] {
			if !inRest[y] {
				continue
			}
			if blocker == "" || (onLoop[y] && !onLoop[blocker]) || (onLoop[y] == onLoop[blocker] && y < blocker) {
				blocker = y
			}
		}
		why := "which is in a load order loop"
		if !onLoop[blocker] {
			why = "which cannot load because of a load order loop"
		}
		r.add(seqLoop, Problem{Plugin: x, Msg: fmt.Sprintf("waits for %s, %s", blocker, why)})
	}
	for _, x := range rest {
		delete(r.alive, x)
	}
}

// loopFrom finds the shortest loop from start back to start inside
// members, trying smaller ids first: [start ... start].
func loopFrom(start string, succ map[string][]string, members []string) []string {
	in := map[string]bool{}
	for _, m := range members {
		in[m] = true
	}
	parent := map[string]string{}
	queue := []string{start}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, s := range succ[n] {
			if !in[s] {
				continue
			}
			if s == start {
				path := []string{start}
				var back []string
				for c := n; c != start; c = parent[c] {
					back = append(back, c)
				}
				for i := len(back) - 1; i >= 0; i-- {
					path = append(path, back[i])
				}
				return append(path, start)
			}
			if _, seen := parent[s]; !seen {
				parent[s] = n
				queue = append(queue, s)
			}
		}
	}
	return []string{start, start}
}
