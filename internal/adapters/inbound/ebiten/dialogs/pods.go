package dialogs

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// The Load transport pods dialog (MAIN_UI_SCREENS §35 travel slice, AX-9):
// pick the colonists, tame animals and goods a pod group carries, watch
// the mass against its lift (150 kg a pod, wiki), Load. It emits LoadPods
// (Target = the pod, Name = "id" entries for colonists and animals and
// "stack:qty" entries for goods); the colonists walk in and haulers bring
// the rest.

const (
	LoadPodsID   = "colony.pods.load"
	loadPodsW    = 760
	loadPodsH    = 540
	LoadNoPicks  = "Pick someone or something to load."
	LoadTooHeavy = "Too heavy for the pods."
)

// podLoad is the dialog's draft.
type podLoad struct {
	pod    entity.EntityID
	pawns  map[entity.EntityID]bool
	beasts map[entity.EntityID]bool
	goods  map[string]int // item kind name -> amount
}

// goodsRow is one kind of goods on the map: its kind, how much lies loose
// and the stacks holding it (ascending).
type goodsRow struct {
	kind   string
	qty    int
	stacks []game.ItemView
}

// OpenLoadPods opens the dialog for pod's group.
func (dl *Dealings) OpenLoadPods(pod entity.EntityID) {
	f := &podLoad{pod: pod, pawns: map[entity.EntityID]bool{}, beasts: map[entity.EntityID]bool{}, goods: map[string]int{}}
	dl.kit().Push(uikit.Modal{
		ID: LoadPodsID, Title: "Load transport pods", W: loadPodsW, H: loadPodsH,
		Draw: func(body gfx.Rect) { dl.loadPodsBody(f, body) },
		Buttons: []uikit.ModalButton{
			{ID: "pods.load", Label: "Load", Style: uikit.BtnPrimary, Closes: true,
				Disabled: func() string { return dl.loadReason(f) }, OnClick: func() { dl.sendLoad(f) }},
			{ID: "pods.cancel", Label: "Cancel", Closes: true},
		},
	})
}

// podView is the pod's building view (ok false: gone).
func (dl *Dealings) podView(pod entity.EntityID) (game.BuildingView, bool) {
	for _, b := range dl.sh.World().Buildings {
		if b.ID == pod {
			return b, true
		}
	}
	return game.BuildingView{}, false
}

// goodsRows are the loose goods on the map by kind (no corpses), in kind
// name order.
func (dl *Dealings) goodsRows() []goodsRow {
	by := map[string]*goodsRow{}
	for _, it := range dl.sh.World().Items {
		k, ok := item.ParseKind(it.Kind)
		if !ok || it.Carried || it.Forbidden || k.IsCorpse() || it.Qty <= 0 {
			continue
		}
		r := by[it.Kind]
		if r == nil {
			r = &goodsRow{kind: it.Kind}
			by[it.Kind] = r
		}
		r.qty += it.Qty
		r.stacks = append(r.stacks, it)
	}
	var out []goodsRow
	for _, r := range by {
		slices.SortFunc(r.stacks, func(a, b game.ItemView) int { return int(a.ID) - int(b.ID) })
		out = append(out, *r)
	}
	slices.SortFunc(out, func(a, b goodsRow) int { return strings.Compare(a.kind, b.kind) })
	return out
}

// GoodsKg is n of kind name's mass in kilograms (rounded up).
func GoodsKg(kind string, n int) int {
	k, ok := item.ParseKind(kind)
	if !ok {
		return 0
	}
	return (n*k.MassGrams() + 999) / 1000
}

// AnimalKg is an animal's mass aboard by its species key.
func AnimalKg(species string) int {
	s, ok := animal.ParseSpecies(species)
	if !ok {
		return 0
	}
	return caravan.AnimalMassKg(animal.RosterOf(s).Size)
}

// pickedKg is the mass of everything picked.
func (dl *Dealings) pickedKg(f *podLoad) int {
	kg := 0
	for _, on := range f.pawns {
		if on {
			kg += caravan.HumanMassKg
		}
	}
	for _, a := range dl.sh.World().Animals {
		if f.beasts[a.ID] {
			kg += AnimalKg(a.Species)
		}
	}
	for k, n := range f.goods {
		kg += GoodsKg(k, n)
	}
	return kg
}

// loadReason is why Load is disabled ("" it is not).
func (dl *Dealings) loadReason(f *podLoad) string {
	b, ok := dl.podView(f.pod)
	switch {
	case !ok:
		return "The pods are gone."
	case dl.loadList(f) == "":
		return LoadNoPicks
	case b.AX.LoadKg+dl.pickedKg(f) > b.AX.CapKg:
		return LoadTooHeavy
	}
	return ""
}

// picks are the picked colonists and animals (ascending) and goods.
func (f *podLoad) picks() ([]entity.EntityID, map[string]int) {
	var ids []entity.EntityID
	for id, on := range f.pawns {
		if on {
			ids = append(ids, id)
		}
	}
	for id, on := range f.beasts {
		if on {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids, f.goods
}

// LoadList is the command's Name: the ids, then for each kind of goods
// (in name order) "stack:qty" entries over its stacks until the amount is
// met.
func LoadList(ids []entity.EntityID, goods map[string]int, rows []goodsRow) string {
	var parts []string
	for _, id := range ids {
		parts = append(parts, strconv.FormatUint(uint64(id), 10))
	}
	for _, r := range rows {
		left := goods[r.kind]
		for _, st := range r.stacks {
			if left <= 0 {
				break
			}
			n := min(left, st.Qty)
			parts = append(parts, fmt.Sprintf("%d:%d", st.ID, n))
			left -= n
		}
	}
	return strings.Join(parts, ",")
}

// sendLoad emits LoadPods.
func (dl *Dealings) sendLoad(f *podLoad) {
	if dl.loadReason(f) != "" {
		return
	}
	dl.sh.Enqueue(command.Command{Kind: command.LoadPods, Target: uint64(f.pod), Enabled: true, Name: dl.loadList(f)})
}

// loadList is f's command Name.
func (dl *Dealings) loadList(f *podLoad) string {
	ids, goods := f.picks()
	return LoadList(ids, goods, dl.goodsRows())
}

// loadPodsBody declares the colonist and animal picks, the goods and the
// mass line.
func (dl *Dealings) loadPodsBody(f *podLoad, body gfx.Rect) {
	left, right := body.SplitH(body.W * 2 / 5)
	left.W -= 12
	hs := gfx.WithColor(gfx.Label, uikit.Heading)
	right, foot := right.SplitV(right.H - 48)
	lh, lrest := left.SplitV(26)
	dl.kit().Label(lh.X, lh.Y, "Colonists and animals", hs)
	var rows []func(r gfx.Rect)
	for _, p := range dl.sh.Pawns() {
		p := p
		why := caravanPickReason(p)
		rows = append(rows, func(rr gfx.Rect) {
			on := f.pawns[p.ID] && why == ""
			if dl.kit().Check(fmt.Sprintf("pods.pawn.%d", p.ID), rr, pawnLabel(p)+fmt.Sprintf("  (%d kg)", caravan.HumanMassKg), &on,
				uikit.CheckOpts{Disabled: why != "", Reason: why}) {
				f.pawns[p.ID] = on
			}
		})
	}
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
			if dl.kit().Check(fmt.Sprintf("pods.animal.%d", a.ID), rr, fmt.Sprintf("%s  (%d kg)", name, AnimalKg(a.Species)), &on, uikit.CheckOpts{}) {
				f.beasts[a.ID] = on
			}
		})
	}
	dl.scrollRows("pods.who", lrest, rows)

	gh, grest := right.SplitV(26)
	dl.kit().Label(gh.X, gh.Y, "Goods", hs)
	goods := dl.goodsRows()
	rows = nil
	for _, g := range goods {
		g := g
		rows = append(rows, func(rr gfx.Rect) {
			v := f.goods[g.kind]
			dl.kit().Label(rr.X, rr.Y+4, gfx.Ellipsize(fmt.Sprintf("%s  (%d here)", Capitalize(g.kind), g.qty), gfx.Body, float64(rr.W-260)), gfx.Body)
			uikit.Stepper(dl.kit(), "pods.goods."+strings.ReplaceAll(g.kind, " ", "_"), gfx.Rect{X: rr.Right() - 200, Y: rr.Y, W: 112, H: rr.H}, &v, 0, g.qty)
			if dl.kit().Button("pods.goods."+strings.ReplaceAll(g.kind, " ", "_")+".all", gfx.Rect{X: rr.Right() - 80, Y: rr.Y, W: 72, H: rr.H}, "All", uikit.BtnOpts{}) {
				v = g.qty
			}
			f.goods[g.kind] = v
			if v == 0 {
				delete(f.goods, g.kind)
			}
		})
	}
	if len(goods) == 0 {
		dl.kit().Label(grest.X, grest.Y, "No loose goods on the map.", uikit.DimText())
	}
	dl.scrollRows("pods.goods", grest, rows)

	b, _ := dl.podView(f.pod)
	kg := b.AX.LoadKg + dl.pickedKg(f)
	st := gfx.Body
	if kg > b.AX.CapKg {
		st = gfx.WithColor(gfx.Body, uikit.Danger)
	}
	dl.kit().Label(foot.X, foot.Y+6, fmt.Sprintf("Mass  %d / %d kg  (aboard %d kg)", kg, b.AX.CapKg, b.AX.LoadKg), st)
	dl.kit().Label(foot.X, foot.Y+28, "Colonists walk in themselves; haulers bring the goods and lead the animals.", gfx.WithColor(gfx.Small, uikit.TextDim))
}

// scrollRows declares rows of formRowH in a scroll area over r.
func (dl *Dealings) scrollRows(id string, r gfx.Rect, rows []func(gfx.Rect)) {
	if len(rows) == 0 {
		return
	}
	area := dl.kit().ScrollArea(id, r, float32(len(rows)*formRowH), uikit.ScrollOpts{Step: formRowH * 2})
	y := area.View.Y - area.Off
	for _, row := range rows {
		rr := gfx.Rect{X: area.View.X, Y: y, W: area.View.W - 4, H: formRowH - 4}
		y += formRowH
		if rr.Y >= area.View.Y && rr.Bottom() <= area.View.Bottom() {
			row(rr)
		}
	}
}

// Capitalize is s with its first letter upper-cased.
func Capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
