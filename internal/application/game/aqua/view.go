package aqua

import (
	"slices"
	"strconv"

	rules "rimworld-go/internal/domain/building/aqua"
	"rimworld-go/internal/domain/sim/entity"
)

// Option is one stock in the inspector's picker: ID is the Count of
// command.SetTankStock, Reason why no one here can raise it yet ("": someone
// can). The stock can still be chosen for a handler to come.
type Option struct {
	ID            int
	Label, Reason string
}

// View is a tank's read model (game.TankView). Trouble is the player's words
// for what is wrong: "No power", "Too cold", "Too hot", "No feed", "" when
// fine.
type View struct {
	Stock                         string
	StockID                       int
	Options                       []Option
	Stocked, Ripe                 bool
	GrowthPct, HealthPct, FeedPct int
	Trouble                       string
}

// Equal reports two views that read the same, Options by element.
func (v *View) Equal(o *View) bool {
	if v == nil || o == nil {
		return v == o
	}
	return v.Stock == o.Stock && v.StockID == o.StockID && slices.Equal(v.Options, o.Options) && v.Stocked == o.Stocked &&
		v.Ripe == o.Ripe && v.GrowthPct == o.GrowthPct && v.HealthPct == o.HealthPct && v.FeedPct == o.FeedPct && v.Trouble == o.Trouble
}

// View is fish tank id's read model; it changes nothing.
func (ts *Tanks) View(id entity.EntityID) *View {
	x, y, ok := ts.h.Tank(id)
	if !ok {
		return nil
	}
	var t rules.Tank
	if e := ts.entry(id); e != nil {
		t = e.Tank
	}
	feed := ts.h.Feed(id)
	c := ts.conditions(id, x, y)
	c.Fed = c.Fed || !t.Stocked // an empty tank needs feed only once it has fish
	cause := t.Species.Trouble(c)
	if !t.Species.Valid() && c.Power {
		cause = rules.Fine // no stock chosen: only the power can be wrong
	}
	return &View{Stock: t.Species.String(), StockID: int(t.Species), Options: ts.options(), Stocked: t.Stocked, Ripe: t.Ripe(),
		GrowthPct: t.GrowthPct(), HealthPct: t.HealthPct(), FeedPct: min(100, feed*100/rules.FeedFull), Trouble: cause.String()}
}

// options is the picker's rows.
func (ts *Tanks) options() []Option {
	best := ts.h.BestAnimals()
	all := rules.All()
	out := make([]Option, len(all))
	for i, s := range all {
		d := rules.DefOf(s)
		out[i] = Option{ID: int(s), Label: d.Label}
		if best < d.MinSkill {
			out[i].Reason = "Needs Animals " + strconv.Itoa(d.MinSkill)
		}
	}
	return out
}
