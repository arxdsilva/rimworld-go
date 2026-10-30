package inspector

import (
	"fmt"
	"reflect"
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// The bill settings dialog, reorder and copy/paste (PRD-1, UI checklist
// §14.1). Every control sends a typed ConfigureBill / SetBillIngredient /
// ReorderBill command; the dialog keeps a local copy of the settings it
// is editing so a second click before the next snapshot builds on the
// first. The settings and the per-bill worker line come from the optional
// BillSettingsReader the game implements; without it the dialog starts
// from the defaults and the worker line stays empty.

// Store modes (mirrors recipe.StoreMode; the adapter only sends numbers).
const (
	StoreBest     uint8 = 0 // the best stockpile
	StoreFloor    uint8 = 1 // drop on the floor
	StoreSpecific uint8 = 2 // a specific stockpile
)

const (
	BillCfgModalID = "bill.cfg"
	BillPasteBtn   = "bill.paste"

	billSkillMax   = 20
	billRadiusMax  = 250
	billUnpauseMax = BillMaxCount
	billCfgRowH    = 26
	billCfgChipH   = 22
)

// BillIngredient is one ingredient kind a bill may use.
type BillIngredient struct {
	Item    uint8  // the item kind (command.Stuff)
	Label   string // its name
	Allowed bool
}

// BillSettings is a bill's configuration as the read model reports it.
type BillSettings struct {
	Store              uint8
	Zone               uint64 // the stockpile for StoreSpecific
	Source             uint64 // the storage the bench takes ingredients from (0: anywhere); sent as SetStorageSource
	Radius             int    // ingredient search radius in tiles; 0 unlimited
	SkillMin, SkillMax uint8
	Worker             uint64 // only this colonist; 0 anyone
	UnpauseAt          int    // "until you have": resume below it; 0 the default
	Ingredients        []BillIngredient
	Stuff              uint8 // the pinned material's item kind (PRD-2); 0 any
}

// Bills is the bill dialog, reorder and copy/paste: a view of the Pane
// (kept apart from Building to stay under the method budget).
type Bills struct{ *Pane }

// Bills is the pane's bills component.
func (a *Pane) Bills() Bills { return Bills{a} }

// billSettings are bill bl's settings as the read model reports them.
func billSettings(bl game.BillView) BillSettings {
	s := BillSettings{Store: bl.Store, Zone: bl.StoreZone, Radius: bl.Radius, SkillMin: bl.SkillMin, SkillMax: bl.SkillMax,
		Worker: bl.Worker, UnpauseAt: bl.UnpauseAt, Stuff: bl.StuffKind}
	for k := range bl.Ingredients.Len() {
		in := bl.Ingredients.At(k)
		s.Ingredients = append(s.Ingredients, BillIngredient{Item: in.Kind, Label: in.Label, Allowed: in.Allowed})
	}
	return s
}

// BillClip is a copied bill (the UI session's clipboard).
type BillClip struct {
	Recipe   string
	ModeID   uint8
	Target   int
	Settings BillSettings
}

// BillClipboard is the session's copied bill, nil when empty.
type BillClipboard struct{ Clip *BillClip }

// config command: the whole settings of bill i of b.
func billConfigCmd(b game.BuildingView, i int, s BillSettings) command.Command {
	return command.Command{Kind: command.ConfigureBill, X: b.X, Y: b.Y, Index: i,
		Store: s.Store, Zone: s.Zone, Radius: s.Radius, SkillMin: s.SkillMin, SkillMax: s.SkillMax,
		Worker: s.Worker, UnpauseAt: s.UnpauseAt, Stuff: s.Stuff}
}

// billMaterials are the ingredients of a stuffed recipe's bill, the
// materials it may be made of: every ingredient a material and at least
// two to pick from (nil otherwise, and the dialog shows no picker).
func billMaterials(ings []BillIngredient) []BillIngredient {
	if len(ings) < 2 {
		return nil
	}
	for _, in := range ings {
		if item.Kind(in.Item).StuffCategory() == 0 {
			return nil
		}
	}
	return ings
}

func billIngredientCmd(b game.BuildingView, i int, item uint8, allowed bool) command.Command {
	return command.Command{Kind: command.SetBillIngredient, X: b.X, Y: b.Y, Index: i, Stuff: item, Enabled: allowed}
}

// billStoreLabels name the store modes, in StoreBest.. order.
var billStoreLabels = [...]string{"Best stockpile", "Drop on floor", "Specific stockpile"}

// zoneLabel names stockpile z.
func zoneLabel(z game.ZoneView) string {
	if z.Name != "" {
		return z.Name
	}
	switch z.Kind {
	case game.ZoneShelf:
		return "Shelf " + strconv.Itoa(int(z.ID))
	case game.ZoneDump:
		return "Dumping stockpile " + strconv.Itoa(int(z.ID))
	}
	return "Stockpile " + strconv.Itoa(int(z.ID))
}

// billSource reads BillView.Source (the storage zone or group the bench
// takes ingredients from, 0: anywhere) when the view has it.
func billSource(bl game.BillView) uint64 {
	f := reflect.ValueOf(bl).FieldByName("Source")
	switch f.Kind() {
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return f.Uint()
	case reflect.Int, reflect.Int64:
		return uint64(f.Int())
	}
	return 0
}

func radiusText(r int) string {
	if r <= 0 {
		return "unlimited"
	}
	return fmt.Sprintf("%d tiles", r)
}

// openBillConfig opens the settings dialog of bill i on bench b.
func (a Bills) openBillConfig(b game.BuildingView, i int) {
	if i < 0 || i >= len(b.Bills) {
		return
	}
	a.openBillConfigWith(b, i, billSettings(b.Bills[i]))
}

// openBillConfigWith opens bill i's dialog over settings cfg.
func (a Bills) openBillConfigWith(b game.BuildingView, i int, cfg BillSettings) {
	mode := b.Bills[i].ModeID
	cfg.Source = billSource(b.Bills[i])
	a.kit().Push(uikit.Modal{
		ID: BillCfgModalID, Title: "Bill: " + b.Bills[i].Label, W: 460, H: 560,
		Draw:    func(body gfx.Rect) { a.billConfigBody(body, b, i, mode, &cfg) },
		Buttons: []uikit.ModalButton{{ID: "bill.cfg.done", Label: "Done", Style: uikit.BtnPrimary, Closes: true}},
	})
}

// billConfigBody declares the dialog's controls in body.
func (a Bills) billConfigBody(body gfx.Rect, b game.BuildingView, i int, mode uint8, cfg *BillSettings) {
	k := a.kit()
	send := func() { a.shell.Enqueue(billConfigCmd(b, i, *cfg)) }
	var zones []game.ZoneView
	for _, z := range a.shell.World().Zones {
		if z.Kind == game.ZoneStockpile || z.Kind == game.ZoneDump || z.Kind == game.ZoneShelf {
			zones = append(zones, z)
		}
	}
	var colonists []game.PawnView
	for _, p := range a.shell.Pawns() {
		if p.Status != "dead" {
			colonists = append(colonists, p)
		}
	}
	mats := billMaterials(cfg.Ingredients)
	rowsH := a.billConfigHeight(len(zones), len(colonists), len(cfg.Ingredients), cfg.Store == StoreSpecific, mode == BillModeUntil)
	if len(mats) > 0 {
		rowsH += float32((len(mats)+4)/4)*(billCfgChipH+3) + billCfgRowH + 8
	}
	area := k.ScrollArea("bill.cfg.rows", body, rowsH, uikit.ScrollOpts{Step: billCfgRowH * 2})
	x, w := area.View.X, area.View.W-uikit.ScrollBarW
	y := area.View.Y - area.Off
	label := func(s string) {
		k.Label(x, y+3, s, gfx.Label)
		y += billCfgRowH - 2
	}
	// chips lays labelled buttons out left to right, wrapping.
	chips := func(n int, id func(j int) string, text func(j int) string, on func(j int) bool, click func(j int)) {
		cx := x
		for j := range n {
			cw := min(w, float32(len(text(j)))*7+20)
			if cx+cw > x+w && cx > x {
				cx, y = x, y+billCfgChipH+3
			}
			if k.Button(id(j), gfx.Rect{X: cx, Y: y, W: cw, H: billCfgChipH}, text(j), uikit.BtnOpts{Selected: on(j)}) {
				click(j)
			}
			cx += cw + 4
		}
		y += billCfgChipH + 8
	}
	// stepper: "-" value "+".
	stepper := func(id, caption, value string, minus, plus func()) {
		k.Label(x, y+3, caption, gfx.Body)
		bx := x + w - 150
		if k.Button(id+".minus", gfx.Rect{X: bx, Y: y, W: 22, H: billCfgChipH}, "-", uikit.BtnOpts{}) {
			minus()
		}
		k.Label(bx+28, y+3, value, gfx.Body)
		if k.Button(id+".plus", gfx.Rect{X: bx + 126, Y: y, W: 22, H: billCfgChipH}, "+", uikit.BtnOpts{}) {
			plus()
		}
		y += billCfgRowH
	}
	step := Building{a.Pane}.qtyStep()

	label("Store in")
	chips(len(billStoreLabels), func(j int) string { return "bill.cfg.store." + strconv.Itoa(j) },
		func(j int) string { return billStoreLabels[j] }, func(j int) bool { return cfg.Store == uint8(j) },
		func(j int) { cfg.Store = uint8(j); send() })
	if cfg.Store == StoreSpecific {
		if len(zones) == 0 {
			k.Label(x, y, "No stockpiles yet.", uikit.DimText())
			y += billCfgRowH
		}
		chips(len(zones), func(j int) string { return "bill.cfg.zone." + strconv.Itoa(int(zones[j].ID)) },
			func(j int) string { return zoneLabel(zones[j]) }, func(j int) bool { return cfg.Zone == uint64(zones[j].ID) },
			func(j int) { cfg.Zone = uint64(zones[j].ID); send() })
	}

	label("Ingredients")
	stepper("bill.cfg.radius", "Search radius", radiusText(cfg.Radius),
		func() { cfg.Radius = max(0, cfg.Radius-step); send() },
		func() { cfg.Radius = min(billRadiusMax, cfg.Radius+step); send() })
	if k.Button("bill.cfg.radius.unlimited", gfx.Rect{X: x, Y: y, W: 100, H: billCfgChipH}, "Unlimited", uikit.BtnOpts{Selected: cfg.Radius <= 0}) {
		cfg.Radius = 0
		send()
	}
	y += billCfgRowH
	label("Take from")
	src := cfg.Source
	srcName := "Anywhere"
	for _, z := range zones {
		if uint64(z.ID) == src {
			srcName = zoneLabel(z)
		}
	}
	if k.Button("bill.cfg.source", gfx.Rect{X: x, Y: y, W: min(w, 240), H: billCfgChipH}, srcName+" v", uikit.BtnOpts{Tip: "Take ingredients only from this storage."}) {
		entries := []game.OrderOption{{Label: "Anywhere"}}
		for _, z := range zones {
			entries = append(entries, game.OrderOption{Label: zoneLabel(z)})
		}
		mx, my := a.shell.Cursor()
		a.shell.Pick(float32(mx), float32(my), false, entries, func(j int) {
			var zid uint64
			if j > 0 && j <= len(zones) {
				zid = uint64(zones[j-1].ID)
			}
			cfg.Source = zid
			a.shell.Enqueue(command.Command{Kind: command.SetStorageSource, X: b.X, Y: b.Y, Index: i, Zone: zid})
		})
	}
	y += billCfgRowH
	if len(cfg.Ingredients) > 0 {
		chips(len(cfg.Ingredients), func(j int) string { return "bill.cfg.ing." + strconv.Itoa(int(cfg.Ingredients[j].Item)) },
			func(j int) string { return cfg.Ingredients[j].Label }, func(j int) bool { return cfg.Ingredients[j].Allowed },
			func(j int) {
				cfg.Ingredients[j].Allowed = !cfg.Ingredients[j].Allowed
				a.shell.Enqueue(billIngredientCmd(b, i, cfg.Ingredients[j].Item, cfg.Ingredients[j].Allowed))
			})
	}

	if len(mats) > 0 { // PRD-2: pin the material, or let any allowed one do
		label("Material")
		chips(len(mats)+1, func(j int) string {
			if j == 0 {
				return "bill.cfg.stuff.0"
			}
			return "bill.cfg.stuff." + strconv.Itoa(int(mats[j-1].Item))
		}, func(j int) string {
			if j == 0 {
				return "Any"
			}
			return Capitalize(mats[j-1].Label)
		}, func(j int) bool {
			if j == 0 {
				return cfg.Stuff == 0
			}
			return cfg.Stuff == mats[j-1].Item
		}, func(j int) {
			cfg.Stuff = 0
			if j > 0 {
				cfg.Stuff = mats[j-1].Item
			}
			send()
		})
	}

	label("Skill range")
	stepper("bill.cfg.smin", "At least", strconv.Itoa(int(cfg.SkillMin)),
		func() { cfg.SkillMin = uint8(max(0, int(cfg.SkillMin)-1)); send() },
		func() {
			cfg.SkillMin = uint8(min(billSkillMax, int(cfg.SkillMin)+1))
			cfg.SkillMax = max(cfg.SkillMax, cfg.SkillMin)
			send()
		})
	stepper("bill.cfg.smax", "At most", strconv.Itoa(int(cfg.SkillMax)),
		func() {
			cfg.SkillMax = uint8(max(0, int(cfg.SkillMax)-1))
			cfg.SkillMin = min(cfg.SkillMin, cfg.SkillMax)
			send()
		},
		func() { cfg.SkillMax = uint8(min(billSkillMax, int(cfg.SkillMax)+1)); send() })

	label("Worker")
	chips(len(colonists)+1, func(j int) string {
		if j == 0 {
			return "bill.cfg.worker.0"
		}
		return "bill.cfg.worker." + strconv.FormatUint(uint64(colonists[j-1].ID), 10)
	}, func(j int) string {
		if j == 0 {
			return "Anyone"
		}
		return a.shell.PawnLabel(colonists[j-1])
	}, func(j int) bool {
		if j == 0 {
			return cfg.Worker == 0
		}
		return cfg.Worker == uint64(colonists[j-1].ID)
	}, func(j int) {
		cfg.Worker = 0
		if j > 0 {
			cfg.Worker = uint64(colonists[j-1].ID)
		}
		send()
	})

	if mode == BillModeUntil {
		label("Until you have")
		stepper("bill.cfg.unpause", "Resume below", unpauseText(cfg.UnpauseAt),
			func() { cfg.UnpauseAt = max(0, cfg.UnpauseAt-step); send() },
			func() { cfg.UnpauseAt = min(billUnpauseMax, cfg.UnpauseAt+step); send() })
	}
}

func unpauseText(n int) string {
	if n <= 0 {
		return "default"
	}
	return strconv.Itoa(n)
}

// billConfigHeight is the dialog's content height for the given counts.
func (a Bills) billConfigHeight(zones, colonists, ingredients int, specific, until bool) float32 {
	chipRows := func(n int) float32 { return float32((n+3)/4) * (billCfgChipH + 3) } // ~4 chips a row
	h := float32(3*billCfgRowH + billCfgChipH + 8 + 2*billCfgRowH + 2*billCfgRowH)   // labels, store, radius, skill
	if specific {
		h += chipRows(zones) + billCfgRowH
	}
	h += chipRows(ingredients) + chipRows(colonists+1) + 8
	h += billCfgRowH // Take from
	if until {
		h += 2 * billCfgRowH
	}
	return h + 40
}

// ---- reorder, copy and paste ----------------------------------------------

// BillExtra is the Bills tab's per-bill worker line.
type BillExtra struct {
	Worker      string // who is working it ("" nobody)
	ProgressPct int    // <0: none
	NoWorker    string // why nobody takes it
	Gathering   bool   // the ingredients are still being fetched
}

// Line is the text appended to the bill's row.
func (e BillExtra) Line() string {
	switch {
	case e.Worker != "" && e.Gathering:
		return "  [" + e.Worker + " gathering]"
	case e.Worker != "" && e.ProgressPct > 0:
		return fmt.Sprintf("  [%s %d%%]", e.Worker, e.ProgressPct)
	case e.Worker != "":
		return "  [" + e.Worker + "]"
	case e.NoWorker != "":
		return "  (no worker: " + e.NoWorker + ")"
	}
	return ""
}

func billExtra(bl game.BillView) BillExtra {
	return BillExtra{Worker: bl.CurrentWorker, ProgressPct: bl.Progress, Gathering: bl.Gathering, NoWorker: bl.NoWorker}
}

// copyBill puts bill i of b on the clipboard.
func (a Bills) copyBill(b game.BuildingView, i int) {
	bl := b.Bills[i]
	a.billClip.Clip = &BillClip{Recipe: bl.Recipe, ModeID: bl.ModeID, Target: bl.Target, Settings: billSettings(bl)}
}

// pasteBlock is why the clipboard cannot be pasted on b ("" it can).
func (a Bills) pasteBlock(b game.BuildingView) string {
	c := a.billClip.Clip
	if c == nil {
		return "Copy a bill first."
	}
	for _, r := range (Building{a.Pane}).RecipesFor(b.Kind) {
		if r.Name == c.Recipe {
			return ""
		}
	}
	return "This bench cannot make that."
}

// pasteBill appends the copied bill to b: AddBill, ConfigureBill and one
// SetBillIngredient per forbidden ingredient.
func (a Bills) pasteBill(b game.BuildingView) {
	c := a.billClip.Clip
	if a.pasteBlock(b) != "" {
		return
	}
	var id uint8
	for _, r := range (Building{a.Pane}).RecipesFor(b.Kind) {
		if r.Name == c.Recipe {
			id = r.ID
		}
	}
	idx := len(b.Bills)
	a.shell.Enqueue(command.Command{Kind: command.AddBill, X: b.X, Y: b.Y, Recipe: id, Mode: c.ModeID, Count: max(1, c.Target)})
	a.shell.Enqueue(billConfigCmd(b, idx, c.Settings))
	for _, ing := range c.Settings.Ingredients {
		if !ing.Allowed {
			a.shell.Enqueue(billIngredientCmd(b, idx, ing.Item, false))
		}
	}
}

// billVerb routes bill.cfg|up|dn|copy.<n>; it reports the id was one.
func (a Bills) billVerb(verb string, n int, b game.BuildingView) bool {
	switch verb {
	case "cfg":
		a.openBillConfig(b, n)
	case "up":
		a.shell.Enqueue(command.Command{Kind: command.ReorderBill, X: b.X, Y: b.Y, Index: n, Count: -1})
	case "dn":
		a.shell.Enqueue(command.Command{Kind: command.ReorderBill, X: b.X, Y: b.Y, Index: n, Count: 1})
	case "copy":
		a.copyBill(b, n)
	default:
		return false
	}
	return true
}
