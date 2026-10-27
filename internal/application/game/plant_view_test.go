package game

import (
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/world/plant"
)

// plantViewGame is a small lit map with three plants and the view cache
// on: a crop under the open sky, a wild shrub beside it, and a tree.
func plantViewGame(t *testing.T) (*Game, [3]PlantView) {
	t.Helper()
	g := mildLight(t)
	g.SetViewCache(true)
	g.setRoof(20*40+20, 0)
	var ids [3]PlantView
	for i, s := range []struct {
		sp   plant.Species
		x, y int
	}{{plant.Potato, 20, 20}, {plant.Shrub, 22, 20}, {plant.Tree, 24, 20}} {
		id, err := g.spawnPlant(s.sp, s.x, s.y, 10)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = PlantView{ID: id, X: s.x, Y: s.y}
	}
	return g, ids
}

// TestPlantViewsKeepIdentityOnQuietTick: with no plant written, no zone
// edited and no crop wilting, the plant list is handed out again as it is
// (the fast path), across the clock moving on, dawn included.
func TestPlantViewsKeepIdentityOnQuietTick(t *testing.T) {
	g, _ := plantViewGame(t)
	a := g.plantViews()
	ver := g.views.plants.ver
	for _, at := range [][2]int{{11, 11}, {11, 18}, {11, 23}, {12, 3}, {12, 6}} {
		jumpTo(g, at[0], at[1])
		if b := g.plantViews(); !sameSlice(a, b) {
			t.Fatalf("day %d %d:00: the plant list changed identity", at[0], at[1])
		}
	}
	if g.views.plants.ver != ver {
		t.Fatal("the memo was rebuilt without a plant write")
	}
	// a plant write that changes no view field (a growth tick below the
	// next percent) rebuilds and still hands out the old slice
	id := a[2].ID
	p, _ := g.plants.Get(id)
	p.Growth++
	g.setPlant(id, p)
	if b := g.plantViews(); !sameSlice(a, b) || !reflect.DeepEqual(a, b) {
		t.Fatal("an invisible plant change moved the plant list")
	}
	// a visible one moves it, and the old slice is left as it was
	before := append([]PlantView(nil), a...)
	p.Cut = true
	g.setPlant(id, p)
	b := g.plantViews()
	if sameSlice(a, b) || !plantOf(b, id).Cut {
		t.Fatal("a cut mark kept the plant list")
	}
	if !reflect.DeepEqual(a, before) {
		t.Fatal("the old plant list was rewritten")
	}
}

// TestPlantViewsRefreshOnZoneEdit: painting a grow zone under a plant
// gives its view the zone's crop; a zone edit that changes no plant's
// view keeps the list.
func TestPlantViewsRefreshOnZoneEdit(t *testing.T) {
	g, ids := plantViewGame(t)
	a := g.plantViews()
	zid := g.paintZone(zone.KindGrow, zone.Zone{Crop: uint8(plant.Potato)}, 20, 20, 20, 20)
	if zid == 0 {
		t.Fatal("no grow zone painted")
	}
	b := g.plantViews()
	if sameSlice(a, b) || plantOf(b, ids[0].ID).Zone != "potato" {
		t.Fatalf("the plant under a new grow zone: %+v", plantOf(b, ids[0].ID))
	}
	g.zones.Update(zid, func(z *zone.Zone) { z.Name = "Spuds" })
	if c := g.plantViews(); !sameSlice(b, c) {
		t.Fatal("renaming a zone moved the plant list")
	}
	g.zones.Remove(zid)
	if c := g.plantViews(); sameSlice(b, c) || plantOf(c, ids[0].ID).Zone != "" {
		t.Fatal("removing the zone kept the plant's zone")
	}
}

// TestPlantViewsRefreshOnWiltFlip: freezing air over an outdoor crop
// wilts its view with no plant written; a wild shrub in the same air is
// not a crop and never wilts.
func TestPlantViewsRefreshOnWiltFlip(t *testing.T) {
	g, ids := plantViewGame(t)
	a := g.plantViews()
	if plantOf(a, ids[0].ID).Wilted {
		t.Fatal("a crop in mild air wilted")
	}
	setAir(g, 20, 20, -150)
	b := g.plantViews()
	if sameSlice(a, b) || !plantOf(b, ids[0].ID).Wilted {
		t.Fatal("freezing air did not wilt the crop's view")
	}
	setAir(g, 22, 20, -150)
	if c := g.plantViews(); !sameSlice(b, c) {
		t.Fatal("freezing air over a wild shrub moved the plant list")
	}
	setAir(g, 20, 20, 150)
	if c := g.plantViews(); sameSlice(b, c) || plantOf(c, ids[0].ID).Wilted {
		t.Fatal("mild air again did not lift the wilt")
	}
}

// TestPlantViewsSelfCheckCatchesRawSet: a plant written past setPlant
// (no plantVer bump) is caught by the self-check on the fast path (which
// samples one call in selfCheckEvery under -race: it is called until
// checked).
func TestPlantViewsSelfCheckCatchesRawSet(t *testing.T) {
	g, ids := plantViewGame(t)
	g.plantViews()
	p, _ := g.plants.Get(ids[1].ID)
	p.Cut = true
	g.plants.Set(ids[1].ID, p) // raw: the memo is not told
	defer func() {
		r := recover()
		if s, _ := r.(string); !strings.Contains(s, "without a plantVer bump") {
			t.Fatalf("recovered %v, want the plantVer self-check panic", r)
		}
	}()
	for range 100 * selfCheckEvery {
		g.plantViews()
	}
	t.Fatal("a raw plant write passed the self-check")
}
