package craft

import (
	"fmt"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Batch is the food laid out on a drying rack. It doesn't rot while it
// dries; Left counts down full-speed drying ticks. Saved (Game.Racks):
// the field names are the gob names.
type Batch struct {
	Kind item.Kind
	Qty  int
	Rot  int // spoilage it arrived with, restored if the rack is torn down
	Left int
}

// Percent is how far the batch has dried (a barrel's wort: fermented),
// 0-100.
func (b Batch) Percent() int {
	total := b.Kind.DryTicks()
	if b.Kind == item.Wort {
		total = FermentTicks
	}
	if total <= 0 {
		return 100
	}
	return 100 - b.Left*100/total
}

// Drying is what each drying rack holds.
type Drying struct {
	h     Host
	racks map[entity.EntityID]Batch
}

// Batch is what rack id holds (Qty 0: nothing).
func (d *Drying) Batch(id entity.EntityID) Batch { return d.racks[id] }

// Set lays batch b on rack id.
func (d *Drying) Set(id entity.EntityID, b Batch) { d.racks[id] = b }

// busy: rack id still dries its last batch.
func (d *Drying) busy(id entity.EntityID) bool { return d.racks[id].Qty > 0 }

// Advance moves every rack's batch on by interval ticks at the air
// temperature at the rack; a finished batch drops off the rack as a
// dried stack on its work spot.
func (d *Drying) Advance(interval int) {
	for _, bid := range append([]entity.EntityID(nil), d.h.BuildingIDs()...) {
		rb := d.racks[bid]
		if rb.Qty <= 0 {
			continue
		}
		b, ok := d.h.Building(bid)
		if !ok {
			delete(d.racks, bid)
			continue
		}
		if isBarrel(b.Kind) {
			d.ferment(bid, b, rb, interval) // ferment.go (PRD-4)
			continue
		}
		rb.Left -= interval * item.DryRate(d.h.TemperatureAt(b.X, b.Y)) / 1000
		if rb.Left > 0 {
			d.racks[bid] = rb
			continue
		}
		delete(d.racks, bid)
		_, spot, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		d.h.DropStackOf(item.Stack{Kind: rb.Kind, Qty: rb.Qty, Dried: true}, spot[0], spot[1])
		d.h.Message(spot[0], spot[1], fmt.Sprintf("%d dried %s ready", rb.Qty, rb.Kind))
	}
}

// Empty puts a torn-down rack's undried batch (a barrel's wort) back on the
// ground at (x, y).
func (d *Drying) Empty(id entity.EntityID, x, y int) {
	rb := d.racks[id]
	delete(d.racks, id)
	if rb.Kind == item.Wort {
		rb.Rot = 0 // a barrel's Rot is its ruin time, not spoilage (ferment.go)
	}
	if rb.Qty > 0 {
		d.h.DropStackOf(item.Stack{Kind: rb.Kind, Qty: rb.Qty, Rot: rb.Rot}, x, y)
	}
}

// Export is the racks for a save (the live map, read only).
func (d *Drying) Export() map[entity.EntityID]Batch { return d.racks }

// Import loads saved racks (nil, an older save: kept as they are).
func (d *Drying) Import(m map[entity.EntityID]Batch) {
	if m != nil {
		d.racks = m
	}
}
