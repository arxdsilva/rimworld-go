package craft

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

func TestDryingAdvance(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.build(1, construction.DryingRack, 4, 4)
	total := item.Meat.DryTicks()
	c.Drying.Set(1, Batch{Kind: item.Meat, Qty: 6, Left: total})
	c.Drying.Set(9, Batch{Kind: item.Meat, Qty: 1, Left: total}) // a rack that is gone
	if p := c.Drying.Batch(1).Percent(); p != 0 {
		t.Fatalf("start %d%%", p)
	}
	f.temp = 1000 // hot: full speed
	c.Drying.Advance(total / 2)
	if p := c.Drying.Batch(1).Percent(); p <= 0 || p >= 100 {
		t.Fatalf("half way %d%%", p)
	}
	f.ids = append(f.ids, 9) // the gone rack is still listed: its batch goes
	c.Drying.Advance(total)
	if c.Drying.Batch(1).Qty != 0 || c.Drying.Batch(9).Qty != 0 {
		t.Fatal("finished batch still on the rack")
	}
	if len(f.drops) != 1 || !f.drops[0].Dried || f.drops[0].Qty != 6 || len(f.msgs) != 1 {
		t.Fatalf("drops %+v msgs %v", f.drops, f.msgs)
	}
	if (Batch{}).Percent() != 100 {
		t.Fatal("an empty batch is done")
	}
}

func TestDryingEmpty(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	c.Drying.Set(1, Batch{Kind: item.Berries, Qty: 4, Rot: 7, Left: 10})
	c.Drying.Empty(1, 2, 3)
	c.Drying.Empty(2, 2, 3) // an empty rack drops nothing
	if len(f.drops) != 1 || f.drops[0].Dried || f.drops[0].Rot != 7 || f.drops[0].X != 2 {
		t.Fatalf("drops %+v", f.drops)
	}
}
