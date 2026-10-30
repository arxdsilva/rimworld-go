package inspector

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// Animals', items' and plants' commands and panes (PLAY-018:
// THING-INSPECT owns them). Each kind describes itself twice: a paneSpec
// (title, bars, lines, tabs) for the inspect pane and a list of command
// buttons for the grid. Legacy command ids (animal.*, item.*, plant.*)
// keep their handlers in the *_inspector.go files; new ones are cmd.*
// with Run, Cmds or Options.

// noColonistReason is why a verb is disabled when no colonist is listed.
const noColonistReason = "no colonist can do that"

// animalView is the animal with id from the world, if it is still there; a
// vanished one also clears its selection.
func (a *Pane) animalView(id entity.EntityID) (game.AnimalView, bool) {
	if a.sim() == nil {
		return game.AnimalView{}, false
	}
	w := a.shell.World()
	for i := range w.Animals {
		if w.Animals[i].ID == id {
			return w.Animals[i], true
		}
	}
	if a.sel.HasAnimal && a.sel.SelAnimal == id {
		a.sel.HasAnimal = false // it died or left
	}
	return game.AnimalView{}, false
}

// itemView is the loose stack with id, if it is still on the ground.
func (a *Pane) itemView(id entity.EntityID) (game.ItemView, bool) {
	if a.sim() == nil {
		return game.ItemView{}, false
	}
	w := a.shell.World()
	for i := range w.Items {
		if w.Items[i].ID == id && !w.Items[i].Carried {
			return w.Items[i], true
		}
	}
	if a.sel.HasItem && a.sel.SelItem == id {
		a.sel.HasItem = false // picked up, used or rotted away
	}
	return game.ItemView{}, false
}

// plantView is the plant with id, if it is still standing.
func (a *Pane) plantView(id entity.EntityID) (game.PlantView, bool) {
	if a.sim() == nil {
		return game.PlantView{}, false
	}
	w := a.shell.World()
	for i := range w.Plants {
		if w.Plants[i].ID == id {
			return w.Plants[i], true
		}
	}
	if a.sel.HasPlant && a.sel.SelPlant == id {
		a.sel.HasPlant = false // cut, harvested or eaten
	}
	return game.PlantView{}, false
}

// thingBarColor is a status bar's colour for a percent: good from 70, a
// warning from 30, danger below.
func thingBarColor(pct int) color.RGBA {
	switch {
	case pct < 30:
		return uikit.Danger
	case pct < 70:
		return uikit.Warning
	}
	return uikit.Good
}

// colonistOptions turns the colonists able (or unable, with a reason) to do
// an order into a command's options, in the order given (nearest first). A
// disabled option carries its reason and no command.
func ColonistOptions(acts []ColonistAction) []Option {
	out := make([]Option, 0, len(acts))
	for i := range acts {
		c := acts[i]
		o := Option{Label: c.Name, Disabled: c.Disabled, Reason: c.Reason}
		if !c.Disabled {
			cmd := c.Cmd
			o.Cmd = &cmd
		}
		out = append(out, o)
	}
	return out
}

// colonistVerbCmd is a command whose options are the colonists who could do an
// order at (x, y): a click picks the nearest able one, the options list
// them all. It is disabled, with the first reason, when none can.
func (a Thing) colonistVerbCmd(id, label, icon, desc string, x, y int, match func(game.OrderOption) bool) Button {
	acts := a.Pawn().ColonistActions(x, y, match)
	c := Button{ID: id, Label: label, Icon: icon, Desc: desc,
		Options: func() []Option { return ColonistOptions(acts) }}
	switch {
	case len(acts) == 0:
		c.Disabled, c.Reason = true, noColonistReason
	case acts[0].Disabled: // enabled ones sort first: nobody can
		c.Disabled, c.Reason = true, acts[0].Reason
		if c.Reason == "" {
			c.Reason = noColonistReason
		}
	}
	return c
}

// ---- animals ----

// animalSubtitle is the pane's second line: the species when the title is
// the animal's name, then sex, age stage and tame or wild.
func animalSubtitle(an game.AnimalView) string {
	var parts []string
	if an.Tame && an.Name != "" {
		parts = append(parts, Capitalize(AnimalLabel(an)))
	}
	if an.AgeDays != 0 || an.Stage != "" || an.Scale != 0 {
		sex := "Male"
		if an.Female {
			sex = "Female"
		}
		stage := an.Stage
		if stage == "" {
			stage = "adult"
			if !an.Adult {
				stage = "young"
			}
		}
		parts = append(parts, sex, stage)
	}
	if an.Tame {
		return strings.Join(append(parts, "tame"), " | ")
	}
	return strings.Join(append(parts, "wild"), " | ")
}

// animalPaneState is the pane's state line: what it is doing, or the alarm
// when it is enraged or attacking.
func (a Animal) animalPaneState(an game.AnimalView) string {
	switch {
	case an.Manhunter:
		return "MANHUNTER: hunting colonists!"
	case an.Hostile:
		return "attacking a colonist!"
	}
	if st := a.AnimalStateLine(an); st != "" {
		return st
	}
	return an.Mode
}

// animalPenLine is the pen of a penned tame animal ("Pen 1", "Roaming",
// "Being roped"); "" for any other (a roaming tame animal has the area chip
// instead).
func animalPenLine(an game.AnimalView) string {
	switch {
	case !an.Tame || !an.Penned:
		return ""
	case an.Pen == "":
		return "Pen: Roaming"
	}
	return "Pen: " + an.Pen
}

// animalPaneLines are the pane's body lines, the ones that matter first.
func (a Animal) AnimalPaneLines(an game.AnimalView) []string {
	var out []string
	add := func(s string) {
		if s != "" {
			out = append(out, s)
		}
	}
	add(a.animalPaneState(an))
	for _, r := range AnimalHealthRows(an) { // animal_inspector.go (MED-09)
		add(r)
	}
	if an.Designated {
		if an.Tame {
			add("Marked for slaughter")
		} else {
			add("Marked for hunting")
		}
	}
	if an.TameMark {
		add("Marked for taming")
	}
	if AnimalLifeLine(an) != "" {
		add(fmt.Sprintf("%d days old", an.AgeDays))
	}
	add(AnimalPregLine(an))
	add(AnimalDietLine(an))
	add(AnimalHuntLine(an))
	add(AnimalFeedLine(an)) // FEED-001
	add(animalPenLine(an))
	for _, r := range AnimalBondRows(an) {
		add(r)
	}
	if an.Product != "" {
		add(fmt.Sprintf("%s: %d%%", Capitalize(an.Product), an.ProductPct))
	}
	if !an.Tame && AnimalHasNeeds(an) {
		if row, ok := a.shell.WildRow(an.Species); ok {
			add(fmt.Sprintf("Tame chance %d%%", row.TameChance))
		}
	}
	add(a.shell.AnimalBlurb(an.Species))
	return out
}

// animalPane is the inspect pane of the animal with id.
func (a Animal) AnimalPane(id entity.EntityID) Spec {
	an, ok := a.animalView(id)
	if !ok {
		return Spec{}
	}
	if a.animalLogFor != an.ID { // another animal: its log starts at the top
		if u := a.kit(); u.Scroll != nil {
			u.Scroll[AnimalLogID] = 0
		}
		a.animalLogFor = an.ID
	}
	sp := Spec{Title: animalPaneTitle(an), Subtitle: animalSubtitle(an), Close: "animal.close",
		Lines: a.AnimalPaneLines(an)}
	sp.Bars = []Bar{{Label: "Health", Pct: an.Health, Color: thingBarColor(an.Health)}}
	if AnimalHasNeeds(an) {
		sp.Bars = append(sp.Bars,
			Bar{Label: "Food", Pct: an.Food, Color: uikit.Accent},
			Bar{Label: "Rest", Pct: an.Rest, Color: uikit.LetterInfo})
	}
	if an.Tame && !an.Penned {
		label, tip := a.shell.AreaLabel(an.AreaRef)
		sp.Chips = []Chip{{ID: "animal.area", Label: "Area: " + label, Tip: tip}}
	}
	sp.Tabs = a.animalTabs(an)
	return sp
}

// animalPaneTitle is a tame animal's name, else its species.
func animalPaneTitle(an game.AnimalView) string {
	if an.Tame && an.Name != "" {
		return an.Name
	}
	return Capitalize(AnimalLabel(an))
}

// animalCommands are the animal's command buttons: the hunt (or slaughter)
// mark first, then taming or, for a tame one, release, master and training.
func (a Animal) AnimalCommands(id entity.EntityID) []Button {
	an, ok := a.animalView(id)
	if !ok {
		return nil
	}
	out := []Button{animalHuntCmd(an)}
	if an.Tame {
		out = append(out, Button{ID: "animal.release", Label: "Release", Icon: "release",
			Desc: "Turn it loose. It goes wild again, and a bonded colonist will mind."})
		out = append(out, a.animalMasterCmd(an))
		out = append(out, a.animalCareCmd(an)) // animal_inspector.go (MED-09)
		for _, t := range animalTrainable(an) {
			out = append(out, animalTrainCmd(t))
		}
		return out
	}
	if !an.Manhunter {
		out = append(out, a.animalTameCmd(an))
	}
	return out
}

// animalHuntCmd is Hunt (wild), Slaughter (tame) or their cancel.
func animalHuntCmd(an game.AnimalView) Button {
	c := Button{ID: "animal.hunt", Label: "Hunt", Icon: "order_hunt", Toggle: true, On: an.Designated,
		Desc: "Mark it for hunting. A colonist kills it and the meat is hauled home."}
	switch {
	case an.Designated:
		c.Label, c.Desc = "Cancel hunt", "Take the mark off: nobody will kill it."
	case an.Tame:
		c.Label, c.Icon = "Slaughter", "slaughter"
		c.Desc = "Mark it for slaughter. A colonist kills it for its meat. Release it instead to set it free."
	case an.TameMark:
		c.Disabled, c.Reason = true, "marked for taming"
	}
	return c
}

// animalTameCmd is Tame or Cancel tame; a species that is never tamed, or
// one already marked for hunting, says why it is off.
func (a Animal) animalTameCmd(an game.AnimalView) Button {
	c := Button{ID: "animal.tame", Label: "Tame", Icon: "order_tame", Toggle: true, On: an.TameMark,
		Desc: "Mark it for taming. A colonist keeps trying until it trusts the colony."}
	row, known := a.shell.WildRow(an.Species)
	if known && row.TameChance > 0 {
		c.Desc = fmt.Sprintf("Mark it for taming. A colonist keeps trying until it trusts the colony (about %d%% per attempt).", row.TameChance)
	}
	switch {
	case an.TameMark:
		c.Label, c.Desc = "Cancel tame", "Take the mark off: nobody will try to tame it."
	case known && row.TameChance == 0:
		c.Disabled, c.Reason = true, "cannot be tamed"
	case an.Designated:
		c.Disabled, c.Reason = true, "marked for hunting"
	}
	return c
}

// animalMasterOrders lists the colonists (and "No master") as master
// choices; the current master is marked.
func (a Animal) animalMasterOrders(an game.AnimalView) []game.OrderOption {
	var opts []game.OrderOption
	for _, p := range a.shell.Pawns() {
		o := game.OrderOption{Label: p.Name, Cmd: command.Command{Kind: command.SetAnimalMaster, Target: uint64(an.ID), Pawn: uint64(p.ID)}}
		if p.ID == an.MasterID {
			o.Label += " (master)"
		}
		opts = append(opts, o)
	}
	return append(opts, game.OrderOption{Label: "No master", Cmd: command.Command{Kind: command.SetAnimalMaster, Target: uint64(an.ID)}})
}

// animalMasterCmd picks the animal's master. Its options are the colonists;
// a plain click lists them (picking the first would silently reassign it).
func (a Animal) animalMasterCmd(an game.AnimalView) Button {
	orders := a.animalMasterOrders(an)
	opts := make([]Option, len(orders))
	for i := range orders {
		cmd := orders[i].Cmd
		opts[i] = Option{Label: orders[i].Label, Cmd: &cmd}
	}
	desc := "Pick who handles it. Its master is followed when drafted."
	if an.Master != "" {
		desc += " Now: " + an.Master + "."
	}
	return Button{ID: "cmd.animal.master", Label: "Master", Desc: desc,
		Options: func() []Option { return opts },
		Run: func() { // listed above the cursor, like the options menu
			mx, my := a.shell.Cursor()
			a.shell.Pick(float32(mx), float32(my), true, orders, func(i int) { a.Commands().runOption(opts[i]) })
		}}
}

// animalTrainCmd is one training skill's toggle ("Obedience 2/3").
func animalTrainCmd(t game.AnimalTrainingView) Button {
	desc := fmt.Sprintf("Handlers train it in %s while this is on. %d of %d steps", strings.ToLower(t.Label), t.Steps, t.Needed)
	if t.Learned {
		desc += ", learned"
	}
	return Button{ID: "animal.train." + strconv.Itoa(t.Skill), Label: trainLabel(t), Toggle: true, On: t.On, Desc: desc + "."}
}

// animalTabs are the animal's pane tabs: its Log and, for a tame one that
// can learn, Training.
func (a Animal) animalTabs(an game.AnimalView) []Tab {
	tabs := []Tab{{ID: "inspector.log", Label: "Log", WantW: animalTabW, WantH: animalLogTabH,
		Declare: func(r gfx.Rect) { a.declareAnimalLog(an, r) },
		Draw:    func(screen *ebiten.Image, r gfx.Rect) { a.drawAnimalLogTab(screen, an, r) }}}
	if an.Tame && len(animalTrainable(an)) > 0 {
		tabs = append(tabs, Tab{ID: "inspector.training", Label: "Training", WantW: animalTabW,
			WantH: animalTrainingTabH(an),
			Draw:  func(screen *ebiten.Image, r gfx.Rect) { a.drawAnimalTrainingTab(screen, an, r) }})
	}
	return tabs
}

// ---- items ----

// itemPane is the inspect pane of the loose stack with id.
func (a Thing) ItemPane(id entity.EntityID) Spec {
	it, ok := a.itemView(id)
	if !ok {
		return Spec{}
	}
	lines := ItemLines(it, InStockpile(a.shell.World(), it.X, it.Y))
	if rot := itemRotLine(it); rot != "" { // right under the freshness line
		lines = append(lines[:1:1], append([]string{rot}, lines[1:]...)...)
	}
	title := QualifiedTitle(ItemTitle(it), it.Stuff, it.Quality)
	lines = append(ItemFactLines(it), lines...)
	return Spec{Title: title, Subtitle: ItemCategory(it), Lines: lines, Close: "item.close"}
}

// itemRotLine is how fast a spoiling stack rots where it lies: "" at the
// normal rate and for anything that does not spoil.
func itemRotLine(it game.ItemView) string {
	if it.FreshDays < 0 || it.Rotten || it.Dried || it.Carried {
		return ""
	}
	switch {
	case it.RotRate == 0:
		return "Frozen: it does not rot"
	case it.RotRate == 1000:
		return ""
	}
	return fmt.Sprintf("Rots at %d%% of the normal rate", it.RotRate/10)
}

// itemCommands are the stack's command buttons: Forbid, the haul mark of a
// stack found on the map, then the colonist verbs of its category.
func (a Thing) ItemCommands(id entity.EntityID) []Button {
	it, ok := a.itemView(id)
	if !ok {
		return nil
	}
	forbid := Button{ID: "item.forbid", Label: "Forbid", Icon: "order_forbid", Toggle: true, On: it.Forbidden,
		Hotkey: input.Forbid, Desc: "Colonists leave it alone: no hauling, eating or using."}
	if it.Forbidden {
		forbid.Label, forbid.Icon = "Allow", "order_allow"
		forbid.Desc = "Let colonists haul, eat and use it again."
	}
	out := []Button{forbid}
	if it.Unclaimed {
		mark := Button{ID: "item.haulmark", Label: "Mark for hauling", Icon: "order_haul", Toggle: true, On: it.HaulMarked,
			Desc: "Colonists will haul it to a stockpile."}
		switch {
		case it.HaulMarked:
			mark.Label, mark.Desc = "Unmark", "Colonists leave it where it lies."
		case it.Forbidden: // colonists never haul a forbidden stack
			mark.Disabled, mark.Reason = true, "forbidden: allow it first"
		}
		out = append(out, mark)
	}
	for _, v := range ItemVerbs(it, InStockpile(a.shell.World(), it.X, it.Y)) {
		out = append(out, a.colonistVerbCmd(v.ID, v.Label, v.Icon, v.Desc, it.X, it.Y, v.Match))
	}
	return out
}

// ---- plants ----

// plantSubtitle is the category and growth stage: "Crop · sprouting".
func plantSubtitle(p game.PlantView) string {
	return PlantCategory(p) + " · " + PlantStageWord(p)
}

// plantPane is the inspect pane of the plant with id.
func (a Thing) PlantPane(id entity.EntityID) Spec {
	p, ok := a.plantView(id)
	if !ok {
		return Spec{}
	}
	gv, _ := a.sim().PlantGrowth(id) // PLAY-017: why it is or isn't growing now
	col := uikit.Good
	if plantCaptionWarns(gv) {
		col = uikit.Warning // a roof, a lamp or a fire is needed
	}
	return Spec{Title: PlantTitle(p), Subtitle: plantSubtitle(p), Close: "plant.close",
		Bars:  []Bar{{Label: "Growth", Pct: p.Growth, Color: col}},
		Lines: PlantPaneLines(p, gv)}
}

// plantCommands are the plant's command buttons: Harvest (crops), Cut or
// Chop, and "now by" a colonist of your pick.
func (a Thing) PlantCommands(id entity.EntityID) []Button {
	p, ok := a.plantView(id)
	if !ok {
		return nil
	}
	var out []Button
	if !isTree(p) {
		h := Button{ID: "plant.harvest", Label: "Harvest", Icon: "order_harvest", Toggle: true, On: p.Designated,
			Desc: "Mark it for harvest. A colonist picks it when it is ripe."}
		switch {
		case p.Designated:
			h.Label, h.Desc = "Cancel harvest", "Take the mark off: nobody will pick it."
		case !p.Harvestable:
			h.Disabled, h.Reason = true, "not ready"
		}
		out = append(out, h)
	}
	cut := Button{ID: "plant.cut", Label: "Cut", Icon: "order_cut", Toggle: true, On: p.Cut,
		Desc: "Mark it for cutting. A colonist cuts it down."}
	if isTree(p) {
		cut.Label, cut.Desc = "Chop", "Mark it for chopping. A colonist fells it for wood."
	}
	if p.Cut {
		cut.Label, cut.Desc = "Cancel cut", "Take the mark off: nobody will cut it."
	}
	out = append(out, cut)
	return append(out, a.colonistVerbCmd("cmd.plant.now", "Now by...",
		"", "Have a colonist do it right now. A click sends the nearest; right-click picks who.",
		p.X, p.Y, plantNowMatch(p)))
}
