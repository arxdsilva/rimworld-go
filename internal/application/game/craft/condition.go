package craft

import (
	"fmt"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Condition is what time does to things (PRD-2): food rots by the local
// temperature, made things left under the open sky deteriorate
// (item.DeteriorationRate) and worn apparel wears out. Both passes walk
// the item list on the climate cadence, never the map.
type Condition struct {
	h    Host
	gone []entity.EntityID // the pass's removals, reused
	lost []loss            // what rotted or fell apart, by kind, in order
}

// loss is the quantity of one kind a pass removed, and where the first
// went.
type loss struct {
	kind item.Kind
	qty  int
	x, y int
}

// Rot advances spoilage by the local temperature over interval ticks.
// Spoiled food is destroyed; a rotten corpse lingers unbutcherable until
// it has rotted as long again, then decays away, except a human corpse,
// which becomes the deceased's bones (SIM-017). Stacks being carried or
// reserved for a job keep rotting but are only removed once they are
// free.
func (c *Condition) Rot(interval int) {
	h := c.h
	c.gone, c.lost = c.gone[:0], c.lost[:0]
	for _, id := range h.ItemIDs() {
		st, ok := h.Stack(id)
		if !ok || st.Shelf() == 0 {
			continue
		}
		x, y := st.X, st.Y
		if st.Carrier != 0 {
			if px, py, ok := h.Pos(st.Carrier); ok {
				x, y = px, py
			}
		}
		st.Rot += interval * item.RotRate(h.TemperatureAt(x, y)) / 1000
		if st.Kind == item.CorpseHuman && st.Rot >= 2*st.Shelf() {
			st.Kind, st.Rot = item.Bones, 0 // same Of: whose bones they are
		}
		h.SetStack(id, st)
		gone := st.Rotten() && !st.Kind.IsCorpse() || st.Kind.IsCorpse() && st.Rot >= 2*st.Shelf()
		if gone && st.Carrier == 0 && c.free(id) {
			c.remove(id, st.Kind, st.Qty, x, y)
		}
	}
	c.flush(func(l loss) string {
		if l.kind.IsCorpse() {
			return fmt.Sprintf("A rotten %s decayed away", l.kind)
		}
		return fmt.Sprintf("%d %s rotted away", l.qty, l.kind)
	})
}

// Weather runs the hourly condition pass at tick now (nothing between
// hours): made things lying under the open sky lose hit points, and
// each colonist's worn apparel has its daily wear roll in its own hour
// (staggered by id). A thing at no hit points falls apart.
func (c *Condition) Weather(now uint64) {
	if now%clock.TicksPerHour != 0 {
		return
	}
	hour := now / clock.TicksPerHour
	c.deteriorate(hour)
	c.wearOutfits(hour)
}

// deteriorate takes this hour's share of each exposed made thing's daily
// deterioration. The share is the integer step of a running total
// phased by the stack's id, so a rate below one hit point an hour still
// costs its whole amount over the day, with no remainder kept.
func (c *Condition) deteriorate(hour uint64) {
	h := c.h
	c.gone, c.lost = c.gone[:0], c.lost[:0]
	rain := h.Raining()
	for _, id := range h.ItemIDs() {
		st, ok := h.Stack(id)
		if !ok || !item.HasQuality(st.Kind) || st.Carrier != 0 || !h.Exposed(st.X, st.Y) {
			continue
		}
		r := uint64(item.DeteriorationRate(st, rain, h.Wet(st.X, st.Y)))
		if r == 0 {
			continue
		}
		t := hour + uint64(id)
		n := int((t+1)*r/24000 - t*r/24000) // milli-HP per day over 24 hours
		if n == 0 {
			continue
		}
		st.Wear += n
		h.SetStack(id, st)
		if st.WornOut() && c.free(id) {
			c.remove(id, st.Kind, st.Qty, st.X, st.Y)
		}
	}
	c.flush(func(l loss) string { return fmt.Sprintf("A %s left outside fell apart", l.kind) })
}

// wearOutfits rolls the daily wear of the garments of each living
// colonist whose hour it is: each piece loses a hit point on
// item.WornWearChance permille of days and is gone at none.
func (c *Condition) wearOutfits(hour uint64) {
	h := c.h
	for _, id := range h.PawnIDs() {
		if (uint64(id)+hour)%clock.HoursPerDay != 0 {
			continue
		}
		o := h.Outfit(id)
		if o == (Outfit{}) {
			continue
		}
		name := h.Name(id)
		if name == "" {
			continue // the dead and the gone wear nothing out
		}
		changed := false
		for s := range o.On {
			p := &o.On[s]
			if p.Kind == 0 || h.Roll(1000) >= item.WornWearChance {
				continue
			}
			p.Wear++
			changed = true
			if p.HP() > 0 {
				continue
			}
			x, y, _ := h.Pos(id)
			h.Message(x, y, fmt.Sprintf("%s's %s wore out.", name, p.Kind))
			*p = Piece{}
			o.SetForced(item.Slot(s), false)
		}
		if changed {
			h.SetOutfit(id, o)
		}
	}
}

// free reports a stack nobody holds reserved.
func (c *Condition) free(id entity.EntityID) bool {
	_, taken := c.h.Claims().EntityOwner(id)
	return !taken
}

// remove queues stack id (qty of kind at x, y) for deletion after the
// pass, counting it into the kind's loss.
func (c *Condition) remove(id entity.EntityID, k item.Kind, qty, x, y int) {
	c.gone = append(c.gone, id)
	for i := range c.lost {
		if c.lost[i].kind == k {
			c.lost[i].qty += qty
			return
		}
	}
	c.lost = append(c.lost, loss{kind: k, qty: qty, x: x, y: y})
}

// flush deletes the queued stacks in order and posts one message per
// kind lost, worded by text.
func (c *Condition) flush(text func(loss) string) {
	for _, id := range c.gone {
		c.h.DeleteItem(id)
	}
	for _, l := range c.lost {
		c.h.Message(l.x, l.y, text(l))
	}
	c.gone, c.lost = c.gone[:0], c.lost[:0]
}

// FreshDays is how many days a perishable stack keeps at full rot speed
// (room temperature); -1 for food that never spoils.
func FreshDays(st item.Stack) float64 {
	if st.Shelf() == 0 {
		return -1
	}
	return float64(st.FreshTicks()) / float64(clock.TicksPerDay)
}
