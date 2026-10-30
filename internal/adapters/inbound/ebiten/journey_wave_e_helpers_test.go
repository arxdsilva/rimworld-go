package ebiten

// journey_wave_e_helpers_test.go: fixtures for the wave E journeys (PRD-4
// storage filters, shelves and fine meals; DEF-4 drop pods, fire control,
// grenades and mechanoids). Dev hooks and read-model fields the parallel
// agents land are looked up at run time so a missing one fails the journey
// with a message instead of breaking the package build.

import (
	"reflect"
	"sort"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// weHook calls the first *game.Game method named in names (an exported dev
// hook), building its arguments from the parameter types: a variadic
// item.Kind kit gets kit, an int-like parameter whose type name contains
// "Arrival" gets arrival, every other number gets 0. It fatals listing the
// Raid*/Spawn* methods the game has when none matches.
func weHook(j *journey, names []string, kit []item.Kind, arrival int, extra ...any) []reflect.Value {
	j.t.Helper()
	gv := reflect.ValueOf(j.g)
	for _, n := range names {
		m := gv.MethodByName(n)
		if !m.IsValid() {
			continue
		}
		mt := m.Type()
		var args []reflect.Value
		xi := 0
		for i := 0; i < mt.NumIn(); i++ {
			pt := mt.In(i)
			if mt.IsVariadic() && i == mt.NumIn()-1 {
				for _, k := range kit {
					args = append(args, reflect.ValueOf(k))
				}
				continue
			}
			arg := reflect.New(pt).Elem()
			switch {
			case xi < len(extra) && reflect.TypeOf(extra[xi]).AssignableTo(pt):
				arg.Set(reflect.ValueOf(extra[xi]))
				xi++
			case strings.Contains(pt.Name(), "Arrival") && pt.Kind() >= reflect.Int && pt.Kind() <= reflect.Uint64:
				if pt.Kind() <= reflect.Int64 {
					arg.SetInt(int64(arrival))
				} else {
					arg.SetUint(uint64(arrival))
				}
			case pt.Kind() == reflect.Slice && pt.Elem() == reflect.TypeOf(item.Kind(0)):
				arg.Set(reflect.ValueOf(kit))
			}
			args = append(args, arg)
		}
		return m.Call(args)
	}
	var have []string
	gt := gv.Type()
	for i := 0; i < gt.NumMethod(); i++ {
		n := gt.Method(i).Name
		if strings.Contains(n, "Raid") || strings.Contains(n, "Mech") || strings.Contains(n, "Pod") {
			have = append(have, n)
		}
	}
	sort.Strings(have)
	j.t.Fatalf("no dev hook among %q on *game.Game (R1/R2 may add one); related methods: %q", names, have)
	return nil
}

// weDropPodRaid sends a drop-pod raid carrying kit through whichever hook
// the raid package exposes.
func weDropPodRaid(j *journey, kit ...item.Kind) {
	j.t.Helper()
	weHook(j, []string{"RaidDropPods", "RaidArmedPods", "RaidPods", "RaidArmedVia", "RaidVia", "RaidArrival", "RaidDrop", "RaidArmedBy"}, kit, 1)
}

// weHostileArrival reports a hostile arrival on the world view.
func weHostileArrival(j *journey) (landed, any bool) {
	for _, a := range j.world().Arrivals {
		if a.Hostile {
			any = true
			landed = landed || a.Landed
		}
	}
	return landed, any
}

// weRaiders are the hostile pawns on the map.
func weRaiders(j *journey) []game.PawnView {
	var out []game.PawnView
	for _, p := range j.g.Snapshot() {
		if p.Hostile {
			out = append(out, p)
		}
	}
	return out
}

// weShotsBy counts the projectiles in flight owned by id.
func weShotsBy(j *journey, id uint64) int {
	n := 0
	for _, s := range j.world().Shots {
		if uint64(s.Owner) == id {
			n++
		}
	}
	return n
}

// weButton is the id of the on-screen control holding sub, fatal otherwise
// with the cmd.* ids that are on screen.
func weButton(j *journey, what string, subs ...string) string {
	j.t.Helper()
	if id := wbFind(j, subs...); id != "" {
		return id
	}
	var cmds []string
	for _, id := range wbIDs(j) {
		if strings.HasPrefix(id, "cmd.") || strings.Contains(id, "zone") || strings.Contains(id, "filter") {
			cmds = append(cmds, id)
		}
	}
	j.t.Fatalf("%s: no control with ids containing %q; cmd/zone ids on screen: %q", what, subs, cmds)
	return ""
}

// weStrings reads a []string field by reflection.
func weStrings(j *journey, v any, name string) []string {
	j.t.Helper()
	f := wcField(j, v, name)
	s, ok := f.Interface().([]string)
	if !ok {
		j.t.Fatalf("%T.%s is %s, not []string", v, name, f.Type())
	}
	return s
}

// weHas reports whether any entry of list contains sub (lower case).
func weHas(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(strings.ToLower(s), sub) {
			return true
		}
	}
	return false
}

// weItemsAt counts the uncarried stacks of any kind at (x, y).
func weItemsAt(j *journey, x, y int) (n int) {
	for _, it := range j.world().Items {
		if it.X == x && it.Y == y && !it.Carried {
			n++
		}
	}
	return n
}

// weDraft selects the i-th colonist and drafts it with R.
func weDraft(j *journey, i int) uint64 {
	j.t.Helper()
	id := ordColonists(j)[i]
	ordPick(j, i)
	j.key("R", 0)
	if !j.pawn(id).Drafted {
		j.t.Fatalf("R did not draft colonist %d", id)
	}
	return id
}

// weID is entity.EntityID of a uint64.
func weID(id uint64) entity.EntityID { return entity.EntityID(id) }

// reflectStruct is the struct value behind v.
func reflectStruct(v any) reflect.Value { return reflect.Indirect(reflect.ValueOf(v)) }

// wcIntOf is an integer field as int.
func wcIntOf(j *journey, v any, name string) int { j.t.Helper(); return int(wcNum(j, v, name)) }

// mustView fatals when the view func finds nothing.
func mustView(t interface{ Fatal(...any) }, view func() (any, bool)) any {
	v, ok := view()
	if !ok {
		t.Fatal("the mech vanished")
	}
	return v
}
