package dialogs

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// The Form caravan dialog's packing column (AX-9, MAIN_UI_SCREENS §34):
// tame pack animals and goods by kind, with the caravan's mass against
// what it carries (wiki: Caravan, body size x 35 kg each) and its pace.
// The picks ride in FormCaravan's Name after the colonists: "id" for an
// animal, "stack:qty" for goods (the game loads them at the hitching
// spot).

// packKg is what an animal of species key carries (wiki: body size x 35).
func packKg(species string) int {
	s, ok := animal.ParseSpecies(species)
	if !ok {
		return 0
	}
	return caravan.PackKg(animal.BodySize(s))
}

// capKg is what f's colonists and animals carry.
func (f *formCaravan) capKg(dl *Dealings) int {
	kg := len(f.members()) * caravan.PersonCarryKg
	for _, a := range dl.sh.World().Animals {
		if f.beasts[a.ID] {
			kg += packKg(a.Species)
		}
	}
	return kg
}

// massKg is the goods' mass.
func (f *formCaravan) massKg() int {
	kg := 0
	for k, n := range f.goods {
		kg += GoodsKg(k, n)
	}
	return kg
}

// caravanLoadLine is the summary's load and pace ("80 / 105 kg · about
// 1.6 tiles a day").
func (dl *Dealings) caravanLoadLine(f *formCaravan) string {
	c, m := f.capKg(dl), f.massKg()
	pace := 1000
	if c > 0 {
		pace = max(250, 1000-750*min(m, c)/c)
	}
	return fmt.Sprintf("%d / %d kg · about %.1f tiles a day", m, c, float64(formTilesPerDay*pace)/1000)
}

// caravanList is the command's Name: the colonists, the animals, then the
// goods' stack entries.
func (dl *Dealings) caravanList(f *formCaravan) string {
	var ids []entity.EntityID
	for id, on := range f.beasts {
		if on {
			ids = append(ids, id)
		}
	}
	parts := []string{MemberList(f.members())}
	if extra := LoadList(sortedIDs(ids), f.goods, dl.goodsRows()); extra != "" {
		parts = append(parts, extra)
	}
	return strings.Join(parts, ",")
}

// caravanPacks declares the pack animals and goods column in r.
func (dl *Dealings) caravanPacks(f *formCaravan, r gfx.Rect) {
	hs := gfx.WithColor(gfx.Label, uikit.Heading)
	h, rest := r.SplitV(26)
	dl.kit().Label(h.X, h.Y, "Pack animals and goods", hs)
	var rows []func(gfx.Rect)
	for _, a := range dl.sh.World().Animals {
		if !a.Tame {
			continue
		}
		a := a
		rows = append(rows, func(rr gfx.Rect) {
			on := f.beasts[a.ID]
			name := a.Name
			if name == "" {
				name = a.Species
			}
			if dl.kit().Check("form.animal."+strconv.FormatUint(uint64(a.ID), 10), rr, fmt.Sprintf("%s  (carries %d kg)", name, packKg(a.Species)), &on, uikit.CheckOpts{}) {
				f.beasts[a.ID] = on
			}
		})
	}
	for _, g := range dl.goodsRows() {
		g := g
		key := "form.goods." + strings.ReplaceAll(g.kind, " ", "_")
		rows = append(rows, func(rr gfx.Rect) {
			v := f.goods[g.kind]
			dl.kit().Label(rr.X, rr.Y+4, gfx.Ellipsize(fmt.Sprintf("%s (%d)", Capitalize(g.kind), g.qty), gfx.Body, float64(rr.W-190)), gfx.Body)
			uikit.Stepper(dl.kit(), key, gfx.Rect{X: rr.Right() - 180, Y: rr.Y, W: 112, H: rr.H}, &v, 0, g.qty)
			if dl.kit().Button(key+".all", gfx.Rect{X: rr.Right() - 62, Y: rr.Y, W: 58, H: rr.H}, "All", uikit.BtnOpts{}) {
				v = g.qty
			}
			if f.goods[g.kind] = v; v == 0 {
				delete(f.goods, g.kind)
			}
		})
	}
	if len(rows) == 0 {
		dl.kit().Label(rest.X, rest.Y, "No tame animals or loose goods.", uikit.DimText())
		return
	}
	dl.scrollRows("form.packs", rest, rows)
}

// sortedIDs is ids, ascending.
func sortedIDs(ids []entity.EntityID) []entity.EntityID { slices.Sort(ids); return ids }
