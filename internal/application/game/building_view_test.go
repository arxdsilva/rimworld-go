package game

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
)

// fieldsAllCompared fails unless eq tells base apart from a copy with any
// one field changed (so a field added to the view and forgotten in eq
// fails here), and holds for a deep copy with fresh slices and pointers.
func fieldsAllCompared[T any](t *testing.T, base T, clone func(T) T, eq func(a, b *T) bool) {
	t.Helper()
	c := clone(base)
	if !eq(&base, &c) {
		t.Fatal("a deep copy reads different")
	}
	rt := reflect.TypeOf(base)
	for i := range rt.NumField() {
		b := clone(base)
		f := reflect.ValueOf(&b).Elem().Field(i)
		switch f.Kind() {
		case reflect.String:
			f.SetString(f.String() + "x")
		case reflect.Bool:
			f.SetBool(!f.Bool())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			f.SetInt(f.Int() + 1)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			f.SetUint(f.Uint() + 1)
		case reflect.Slice:
			f.Set(reflect.Append(f, reflect.New(f.Type().Elem()).Elem()))
		case reflect.Pointer:
			if f.IsNil() {
				f.Set(reflect.New(f.Type().Elem()))
			} else {
				f.Set(reflect.Zero(f.Type()))
			}
		default:
			t.Fatalf("field %s: kind %s not handled", rt.Field(i).Name, f.Kind())
		}
		if eq(&base, &b) {
			t.Errorf("field %s is not compared", rt.Field(i).Name)
		}
	}
}

// TestBuildingViewEqual: equalBuildingView compares every field, Bills by
// element and MovingTo by the cell it points at.
func TestBuildingViewEqual(t *testing.T) {
	base := BuildingView{ID: 3, Kind: "campfire", Bills: []BillView{{Recipe: "meal", Target: 10}}, MovingTo: &[2]int{4, 5},
		Linked: []string{"tool cabinet"}}
	clone := func(v BuildingView) BuildingView {
		v.Bills = append([]BillView(nil), v.Bills...)
		v.Linked = append([]string(nil), v.Linked...)
		if v.MovingTo != nil {
			at := *v.MovingTo
			v.MovingTo = &at
		}
		return v
	}
	fieldsAllCompared(t, base, clone, equalBuildingView)
	b := clone(base)
	b.Bills[0].Done++
	if equalBuildingView(&base, &b) {
		t.Fatal("a bill's progress is not compared")
	}
	b = clone(base)
	b.MovingTo[0]++
	if equalBuildingView(&base, &b) {
		t.Fatal("the move target's cell is not compared")
	}
}

// TestFrameViewEqual: equalFrameView compares every field, Haulers by
// element.
func TestFrameViewEqual(t *testing.T) {
	base := FrameView{ID: 3, Kind: "wall", Haulers: []string{"Ada (4 wood)"}}
	clone := func(v FrameView) FrameView {
		v.Haulers = append([]string(nil), v.Haulers...)
		return v
	}
	fieldsAllCompared(t, base, clone, equalFrameView)
}

// viewGame is a bare 40x40 map with the view cache on.
func viewGame(t *testing.T) *Game {
	t.Helper()
	g := New(Config{Seed: 1, Width: 40, Height: 40})
	bareGround(g)
	g.SetViewCache(true)
	return g
}

// TestBuildingViewsKeepIdentity: the building list is handed out again
// while it reads the same; a change hands out a fresh list that keeps an
// unchanged bench's Bills slice, and the old list is left as it was.
func TestBuildingViewsKeepIdentity(t *testing.T) {
	g := viewGame(t)
	fire := buildNow(t, g, construction.Campfire, item.Wood, construction.South, 10, 10)
	wall := buildNow(t, g, construction.Wall, item.Wood, construction.South, 20, 10)
	a := g.buildingViews()
	if len(a) != 2 || len(a[0].Bills) == 0 {
		t.Fatalf("buildings %+v", a)
	}
	if b := g.buildingViews(); !sameSlice(a, b) {
		t.Fatal("an unchanged building list changed identity")
	}
	before := append([]BuildingView(nil), a...)
	w, _ := g.bldgs.Get(wall)
	w.Deconstruct = true
	g.bldgs.Set(wall, w)
	b := g.buildingViews()
	if sameSlice(a, b) || !b[1].Deconstruct {
		t.Fatal("a deconstruct mark kept the building list")
	}
	if b[0].ID != fire || !sameSlice(a[0].Bills, b[0].Bills) {
		t.Fatal("the unchanged campfire's bills were rebuilt")
	}
	if !reflect.DeepEqual(a, before) {
		t.Fatal("the old building list was rewritten")
	}
	g.craft.Bills.Of(fire)[0].Target++
	c := g.buildingViews()
	if sameSlice(b, c) || sameSlice(b[0].Bills, c[0].Bills) || c[0].Bills[0].Target != b[0].Bills[0].Target+1 {
		t.Fatal("a bill change kept the list or the bills")
	}
	// the fresh build agrees with the memo
	g.views.on = false
	if fresh := g.buildingViews(); !reflect.DeepEqual(fresh, c) {
		t.Fatalf("memoised buildings differ from a fresh build:\n%+v\n%+v", c, fresh)
	}
}

// TestZoneViewsShared: the zone list is handed out again until the zone
// map changes.
func TestZoneViewsShared(t *testing.T) {
	g := viewGame(t)
	zid := g.paintZone(zone.KindStockpile, zone.Zone{Food: true}, 2, 2, 4, 4)
	a := g.zoneViews()
	if len(a) != 1 {
		t.Fatalf("zones %+v", a)
	}
	if b := g.zoneViews(); !sameSlice(a, b) {
		t.Fatal("an unchanged zone list changed identity")
	}
	g.zones.Update(zid, func(z *zone.Zone) { z.Name = "Pantry" })
	b := g.zoneViews()
	if sameSlice(a, b) || b[0].Name != "Pantry" || a[0].Name != "" {
		t.Fatalf("renamed zone: old %+v, new %+v", a, b)
	}
}

// TestItemViewsKeepIdentity: the item list and the shared totals (the
// FoodByKind map, the Stored rows) are handed out again while they read
// the same, and rebuilt only when what they count moves.
func TestItemViewsKeepIdentity(t *testing.T) {
	g := viewGame(t)
	g.paintZone(zone.KindStockpile, zone.Zone{Food: true, Wood: true}, 2, 2, 4, 4)
	berries, _ := g.SpawnStack(item.Berries, 10, 3, 3)
	wood, _ := g.SpawnStack(item.Wood, 20, 20, 20)
	world := func() WorldView {
		var w WorldView
		g.fillItemViews(&w)
		return w
	}
	a := world()
	if len(a.Items) != 2 || a.Food != 10 || a.Wood != 20 || a.FoodByKind["berries"] != 10 || len(a.Stored) != 1 {
		t.Fatalf("items %+v food %d wood %d by kind %v stored %+v", a.Items, a.Food, a.Wood, a.FoodByKind, a.Stored)
	}
	b := world()
	if !sameSlice(a.Items, b.Items) || !sameMap(a.FoodByKind, b.FoodByKind) || !sameSlice(a.Stored, b.Stored) {
		t.Fatal("unchanged items changed identity")
	}
	st, _ := g.stacks.Get(wood) // loose wood outside the stockpile: no food, nothing stored
	st.Qty = 25
	g.putStack(wood, st)
	c := world()
	if sameSlice(b.Items, c.Items) || c.Wood != 25 {
		t.Fatal("a wood stack change kept the item list")
	}
	if !sameMap(b.FoodByKind, c.FoodByKind) || !sameSlice(b.Stored, c.Stored) {
		t.Fatal("a wood stack change rebuilt the food or stored totals")
	}
	st, _ = g.stacks.Get(berries)
	st.Qty = 4
	g.putStack(berries, st)
	d := world()
	if sameMap(c.FoodByKind, d.FoodByKind) || sameSlice(c.Stored, d.Stored) || d.FoodByKind["berries"] != 4 || c.FoodByKind["berries"] != 10 {
		t.Fatal("a food stack change kept the food totals, or rewrote the old ones")
	}
	g.views.on = false
	if fresh := world(); !reflect.DeepEqual(fresh, d) {
		t.Fatalf("memoised items differ from a fresh build:\n%+v\n%+v", d, fresh)
	}
}

// sameMap reports two maps that are the same map.
func sameMap[K comparable, V any](a, b map[K]V) bool {
	return reflect.ValueOf(a).UnsafePointer() == reflect.ValueOf(b).UnsafePointer()
}
