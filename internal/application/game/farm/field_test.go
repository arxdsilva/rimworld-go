package farm

import (
	"testing"

	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// field is a fake Field: plants by id, rooms by column (x < wall is room 1).
type field struct {
	ps   map[entity.EntityID]plant.Plant
	ids  []entity.EntityID
	wall int
}

func (f *field) PlantIDs() []entity.EntityID { return f.ids }
func (f *field) Plant(id entity.EntityID) (plant.Plant, bool) {
	p, ok := f.ps[id]
	return p, ok
}
func (f *field) SetPlant(id entity.EntityID, p plant.Plant) { f.ps[id] = p }
func (f *field) PlantAt(x, y int) entity.EntityID {
	for _, id := range f.ids {
		if p := f.ps[id]; p.X == x && p.Y == y {
			return id
		}
	}
	return 0
}
func (f *field) RoomAt(x, _ int) int {
	if x < f.wall {
		return 1
	}
	return 2
}

func riceField(w, h, wall int) *field {
	f := &field{ps: map[entity.EntityID]plant.Plant{}, wall: wall}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			id := entity.EntityID(1 + y*w + x)
			f.ids = append(f.ids, id)
			f.ps[id] = plant.Plant{Species: plant.Rice, X: x, Y: y, Zone: true}
		}
	}
	return f
}

// TestStrikeKeepsToTheRoom: the strike infects only the origin's species in
// its room; nothing to strike, no strike.
func TestStrikeKeepsToTheRoom(t *testing.T) {
	t.Parallel()
	f := riceField(12, 6, 6)
	o, n, ok := Strike(f, 1000)
	if !ok || n == 0 {
		t.Fatalf("no strike: %v %d", ok, n)
	}
	room := f.RoomAt(o.X, o.Y)
	for _, id := range f.ids {
		if p := f.ps[id]; p.Blight > 0 && f.RoomAt(p.X, p.Y) != room {
			t.Fatalf("struck across the wall at %d,%d", p.X, p.Y)
		}
	}
	if _, _, ok := Strike(&field{ps: map[entity.EntityID]plant.Plant{}}, 5); ok {
		t.Fatal("struck an empty field")
	}
}

// TestSpreadStaysWithinFourTiles: a plant past 28 % passes blight to a
// healthy neighbour within 4 tiles; one under 28 % never does.
func TestSpreadStaysWithinFourTiles(t *testing.T) {
	t.Parallel()
	f := riceField(15, 15, 99)
	src := entity.EntityID(1 + 7*15 + 7)
	p := f.ps[src]
	p.Blight = plant.BlightSpread - 1
	f.ps[src] = p
	for now := uint64(0); now < 5000; now++ {
		Spread(f, src, p, now)
	}
	for _, id := range f.ids {
		if id != src && f.ps[id].Blight > 0 {
			t.Fatal("spread under 28 %")
		}
	}
	p.Blight = plant.BlightSpread
	for now := uint64(0); now < 5000; now++ {
		Spread(f, src, p, now)
	}
	n := 0
	for _, id := range f.ids {
		q := f.ps[id]
		if id == src || q.Blight == 0 {
			continue
		}
		n++
		if dx, dy := q.X-7, q.Y-7; dx*dx+dy*dy > plant.BlightRadius*plant.BlightRadius {
			t.Fatalf("spread to %d,%d", q.X, q.Y)
		}
	}
	if n == 0 {
		t.Fatal("never spread")
	}
}
