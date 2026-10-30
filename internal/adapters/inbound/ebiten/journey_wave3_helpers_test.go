package ebiten

// journey_wave3_helpers_test.go: fixtures for the wave 3 journeys (CBT-3
// weapons and gear, CBT-4 AI, CBT-5 explosions and combat UI). Dev hooks and
// read-model fields the parallel agents land are looked up at run time so a
// missing one fails the journey with a message instead of breaking the build.

import (
	"reflect"
	"sort"
	"strings"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// w3Call calls the first *game.Game method named in names whose parameters
// accept args (numbers convert, item kinds and ids pass through); fatal,
// listing the game methods holding a hint, when none does.
func w3Call(j *journey, names []string, hints []string, args ...any) []reflect.Value {
	j.t.Helper()
	gv := reflect.ValueOf(j.g)
	for _, n := range names {
		m := gv.MethodByName(n)
		if !m.IsValid() || m.Type().NumIn() != len(args) && !m.Type().IsVariadic() {
			continue
		}
		var in []reflect.Value
		ok := true
		for i, a := range args {
			pt := m.Type().In(min(i, m.Type().NumIn()-1))
			if m.Type().IsVariadic() && i >= m.Type().NumIn()-1 {
				pt = pt.Elem()
			}
			av := reflect.ValueOf(a)
			if !av.Type().ConvertibleTo(pt) {
				ok = false
				break
			}
			in = append(in, av.Convert(pt))
		}
		if ok {
			return m.Call(in)
		}
	}
	var have []string
	gt := gv.Type()
	for i := 0; i < gt.NumMethod(); i++ {
		for _, h := range hints {
			if strings.Contains(strings.ToLower(gt.Method(i).Name), h) {
				have = append(have, gt.Method(i).Name)
				break
			}
		}
	}
	sort.Strings(have)
	j.t.Fatalf("no *game.Game method among %q accepts %d args; methods matching %q: %q", names, len(args), hints, have)
	return nil
}

// w3Num finds the first numeric (or bool, as 0/1) field of v whose name
// holds every substring in subs (lower case), and returns its value, its
// name and whether there is one.
func w3Num(v any, subs ...string) (float64, string, bool) {
	rv := reflect.Indirect(reflect.ValueOf(v))
	for i := 0; i < rv.NumField(); i++ {
		name := rv.Type().Field(i).Name
		low := strings.ToLower(name)
		ok := true
		for _, s := range subs {
			ok = ok && strings.Contains(low, s)
		}
		if !ok {
			continue
		}
		f := rv.Field(i)
		switch {
		case f.CanInt():
			return float64(f.Int()), name, true
		case f.CanUint():
			return float64(f.Uint()), name, true
		case f.CanFloat():
			return f.Float(), name, true
		case f.Kind() == reflect.Bool:
			if f.Bool() {
				return 1, name, true
			}
			return 0, name, true
		}
	}
	return 0, "", false
}

// w3Shield is colonist id's shield charge read by reflection off its pawn
// view (a numeric field named *Shield*).
func w3Shield(j *journey, id uint64) float64 {
	j.t.Helper()
	p := j.pawn(id)
	v, _, ok := w3Num(p, "shield")
	if !ok {
		j.t.Fatalf("PawnView has no numeric *Shield* field yet (the shield charge read model); worn %v", p.Apparel)
	}
	return v
}

// w3GunshotOn reports a gunshot or bullet wound on id's health view.
func w3GunshotOn(j *journey, id entity.EntityID) bool {
	hv, ok := j.g.Health(id)
	if !ok {
		return false
	}
	for _, p := range hv.Parts {
		txt := strings.ToLower(p.Label + " " + p.Status + " " + p.TendTip)
		if strings.Contains(txt, "gunshot") || strings.Contains(txt, "bullet") {
			return true
		}
	}
	return false
}

// w3Wear forces colonist pawn to put on the belt kind k lying at (x, y)
// (spawned there) and waits until the pawn wears it.
func w3Wear(j *journey, pawn uint64, k item.Kind, x, y int) {
	j.t.Helper()
	if _, err := j.g.SpawnStack(k, 1, x, y); err != nil {
		j.t.Fatal(err)
	}
	j.g.Enqueue(command.Command{Kind: command.Prioritize, Pawn: pawn, Order: command.OrderEquip, X: x, Y: y})
	j.g.ApplyPending()
	j.key("Digit3", 0)
	for i := 0; i < 2000 && !w3Wears(j, pawn, k.String()); i++ {
		careFeed(j)
		j.update(1)
	}
	j.key("Space", 0)
	if !w3Wears(j, pawn, k.String()) {
		j.t.Fatalf("colonist %d never put on the %s: %+v", pawn, k, j.pawn(pawn).Apparel)
	}
}

// w3Wears reports that colonist pawn wears something named name.
func w3Wears(j *journey, pawn uint64, name string) bool {
	for _, a := range j.pawn(pawn).Apparel {
		if strings.EqualFold(a, name) {
			return true
		}
	}
	return false
}

// w3Throw drafts colonist idx, presses Throw grenade and clicks (x, y).
func w3Throw(j *journey, idx int, x, y int) {
	j.t.Helper()
	id := weDraft(j, idx)
	weThrowAt(j, id, x, y)
}

// weThrowAt presses Throw grenade for the drafted, selected colonist id and
// clicks (x, y).
func weThrowAt(j *journey, id uint64, x, y int) {
	j.t.Helper()
	j.clickID("cmd.pawn.grenade")
	j.lookAt(x, y)
	j.update(1)
	j.settle()
	j.clickTile(x, y, 0, 0)
	if c, ok := j.lastCmd(command.ThrowGrenade); !ok || c.Pawn != id {
		j.t.Fatalf("ThrowGrenade %+v (sent %v), want colonist %d", c, ok, id)
	}
}

// w3Bursts counts the bursts of that blast kind (case blind).
func w3Bursts(j *journey, blast string) (n int) {
	for _, b := range j.world().Bursts {
		if strings.EqualFold(b.Blast, blast) {
			n++
		}
	}
	return n
}

// w3Hostiles are the raiders still standing.
func w3Hostiles(j *journey) (out []uint64) {
	for _, p := range weRaiders(j) {
		if p.Status == "alive" {
			out = append(out, uint64(p.ID))
		}
	}
	return out
}

// w3Auto calls the first *game.Game method named in names, filling its
// parameters by type: an entity.EntityID gets id, another integer n, a
// float n, a string str, anything else its zero value (a variadic tail is
// left empty). Fatal, listing the game methods matching hints, when none
// of the names exists.
func w3Auto(j *journey, names, hints []string, id entity.EntityID, n int, str string) []reflect.Value {
	j.t.Helper()
	gv := reflect.ValueOf(j.g)
	for _, name := range names {
		m := gv.MethodByName(name)
		if !m.IsValid() {
			continue
		}
		mt := m.Type()
		var in []reflect.Value
		for i := 0; i < mt.NumIn(); i++ {
			if mt.IsVariadic() && i == mt.NumIn()-1 {
				break
			}
			pt := mt.In(i)
			a := reflect.New(pt).Elem()
			switch {
			case pt == reflect.TypeOf(entity.EntityID(0)):
				a.SetUint(uint64(id))
			case pt.Kind() >= reflect.Int && pt.Kind() <= reflect.Int64:
				a.SetInt(int64(n))
			case pt.Kind() >= reflect.Uint && pt.Kind() <= reflect.Uint64:
				a.SetUint(uint64(n))
			case pt.Kind() == reflect.String:
				a.SetString(str)
			}
			in = append(in, a)
		}
		return m.Call(in)
	}
	var have []string
	gt := gv.Type()
	for i := 0; i < gt.NumMethod(); i++ {
		for _, h := range hints {
			if strings.Contains(strings.ToLower(gt.Method(i).Name), h) {
				have = append(have, gt.Method(i).Name)
				break
			}
		}
	}
	sort.Strings(have)
	j.t.Fatalf("no *game.Game dev hook among %q (CC/CD/CE may add one); methods matching %q: %q", names, hints, have)
	return nil
}

// isZero reports the zero value of any comparable struct.
func isZero(v any) bool { return reflect.ValueOf(v).IsZero() }
