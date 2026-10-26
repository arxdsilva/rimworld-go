package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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
func (a *App) animalView(id entity.EntityID) (game.AnimalView, bool) {
	if a.renderer == nil {
		return game.AnimalView{}, false
	}
	w := a.renderer.world()
	for i := range w.Animals {
		if w.Animals[i].ID == id {
			return w.Animals[i], true
		}
	}
	if a.hasAnimal && a.selAnimal == id {
		a.hasAnimal = false // it died or left
	}
	return game.AnimalView{}, false
}

// itemView is the loose stack with id, if it is still on the ground.
func (a *App) itemView(id entity.EntityID) (game.ItemView, bool) {
	if a.renderer == nil {
		return game.ItemView{}, false
	}
	w := a.renderer.world()
	for i := range w.Items {
		if w.Items[i].ID == id && !w.Items[i].Carried {
			return w.Items[i], true
		}
	}
	if a.hasItem && a.selItem == id {
		a.hasItem = false // picked up, used or rotted away
	}
	return game.ItemView{}, false
}

// plantView is the plant with id, if it is still standing.
func (a *App) plantView(id entity.EntityID) (game.PlantView, bool) {
	if a.renderer == nil {
		return game.PlantView{}, false
	}
	w := a.renderer.world()
	for i := range w.Plants {
		if w.Plants[i].ID == id {
			return w.Plants[i], true
		}
	}
	if a.hasPlant && a.selPlant == id {
		a.hasPlant = false // cut, harvested or eaten
	}
	return game.PlantView{}, false
}

// thingBarColor is a status bar's colour for a percent: good from 70, a
// warning from 30, danger below.
func thingBarColor(pct int) color.RGBA {
	switch {
	case pct < 30:
		return uiDanger
	case pct < 70:
		return uiWarning
	}
	return uiGood
}

// colonistOptions turns the colonists able (or unable, with a reason) to do
// an order into a command's options, in the order given (nearest first). A
// disabled option carries its reason and no command.
func colonistOptions(acts []colonistAction) []cmdOption {
	out := make([]cmdOption, 0, len(acts))
	for i := range acts {
		c := acts[i]
		o := cmdOption{Label: c.Name, Disabled: c.Disabled, Reason: c.Reason}
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
func (a *App) colonistVerbCmd(id, label, icon, desc string, x, y int, match func(game.OrderOption) bool) cmdButton {
	acts := a.colonistActions(x, y, match)
	c := cmdButton{ID: id, Label: label, Icon: icon, Desc: desc,
		Options: func(*App) []cmdOption { return colonistOptions(acts) }}
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
		parts = append(parts, capitalize(animalLabel(an)))
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
func (a *App) animalPaneState(an game.AnimalView) string {
	switch {
	case an.Manhunter:
		return "MANHUNTER: hunting colonists!"
	case an.Hostile:
		return "attacking a colonist!"
	}
	if st := a.animalStateLine(an); st != "" {
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
func (a *App) animalPaneLines(an game.AnimalView) []string {
	var out []string
	add := func(s string) {
		if s != "" {
			out = append(out, s)
		}
	}
	add(a.animalPaneState(an))
	for _, r := range animalHealthRows(an) { // animal_inspector.go (MED-09)
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
	if animalLifeLine(an) != "" {
		add(fmt.Sprintf("%d days old", an.AgeDays))
	}
	add(animalPregLine(an))
	add(animalDietLine(an))
	add(animalHuntLine(an))
	add(animalPenLine(an))
	for _, r := range animalBondRows(an) {
		add(r)
	}
	if an.Product != "" {
		add(fmt.Sprintf("%s: %d%%", capitalize(an.Product), an.ProductPct))
	}
	if !an.Tame && animalHasNeeds(an) {
		if row, ok := a.wildRow(an.Species); ok {
			add(fmt.Sprintf("Tame chance %d%%", row.TameChance))
		}
	}
	add(animalBlurb(an.Species))
	return out
}

// animalPane is the inspect pane of the animal with id.
func (a *App) animalPane(id entity.EntityID) paneSpec {
	an, ok := a.animalView(id)
	if !ok {
		return paneSpec{}
	}
	if a.animalLogFor != an.ID { // another animal: its log starts at the top
		if u := a.uiSt(); u.scroll != nil {
			u.scroll[animalLogID] = 0
		}
		a.animalLogFor = an.ID
	}
	sp := paneSpec{Title: animalPaneTitle(an), Subtitle: animalSubtitle(an), Close: "animal.close",
		Lines: a.animalPaneLines(an)}
	sp.Bars = []paneBar{{Label: "Health", Pct: an.Health, Color: thingBarColor(an.Health)}}
	if animalHasNeeds(an) {
		sp.Bars = append(sp.Bars,
			paneBar{Label: "Food", Pct: an.Food, Color: uiAccent},
			paneBar{Label: "Rest", Pct: an.Rest, Color: uiLetterInfo})
	}
	if an.Tame && !an.Penned {
		sp.Chips = []paneChip{{ID: "animal.area", Label: "Area: " + areaRefLabel(a.renderer.game.Areas(), an.AreaRef), Tip: assignAreaTip}}
	}
	sp.Tabs = a.animalTabs(an)
	return sp
}

// animalPaneTitle is a tame animal's name, else its species.
func animalPaneTitle(an game.AnimalView) string {
	if an.Tame && an.Name != "" {
		return an.Name
	}
	return capitalize(animalLabel(an))
}

// animalCommands are the animal's command buttons: the hunt (or slaughter)
// mark first, then taming or, for a tame one, release, master and training.
func (a *App) animalCommands(id entity.EntityID) []cmdButton {
	an, ok := a.animalView(id)
	if !ok {
		return nil
	}
	out := []cmdButton{animalHuntCmd(an)}
	if an.Tame {
		out = append(out, cmdButton{ID: "animal.release", Label: "Release", Icon: "release",
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
func animalHuntCmd(an game.AnimalView) cmdButton {
	c := cmdButton{ID: "animal.hunt", Label: "Hunt", Icon: "order_hunt", Toggle: true, On: an.Designated,
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
func (a *App) animalTameCmd(an game.AnimalView) cmdButton {
	c := cmdButton{ID: "animal.tame", Label: "Tame", Icon: "order_tame", Toggle: true, On: an.TameMark,
		Desc: "Mark it for taming. A colonist keeps trying until it trusts the colony."}
	row, known := a.wildRow(an.Species)
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
func (a *App) animalMasterOrders(an game.AnimalView) []game.OrderOption {
	var opts []game.OrderOption
	for _, p := range a.renderer.snapshot() {
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
func (a *App) animalMasterCmd(an game.AnimalView) cmdButton {
	orders := a.animalMasterOrders(an)
	opts := make([]cmdOption, len(orders))
	for i := range orders {
		cmd := orders[i].Cmd
		opts[i] = cmdOption{Label: orders[i].Label, Cmd: &cmd}
	}
	desc := "Pick who handles it. Its master is followed when drafted."
	if an.Master != "" {
		desc += " Now: " + an.Master + "."
	}
	return cmdButton{ID: "cmd.animal.master", Label: "Master", Desc: desc,
		Options: func(*App) []cmdOption { return opts },
		Run: func(a *App) { // listed above the cursor, like the options menu
			mx, my := a.currentCursor()
			a.openPick(float32(mx), float32(my), true, orders, func(i int) { a.runOption(opts[i]) })
		}}
}

// animalTrainCmd is one training skill's toggle ("Obedience 2/3").
func animalTrainCmd(t game.AnimalTrainingView) cmdButton {
	desc := fmt.Sprintf("Handlers train it in %s while this is on. %d of %d steps", strings.ToLower(t.Label), t.Steps, t.Needed)
	if t.Learned {
		desc += ", learned"
	}
	return cmdButton{ID: "animal.train." + strconv.Itoa(t.Skill), Label: trainLabel(t), Toggle: true, On: t.On, Desc: desc + "."}
}

// animalTabs are the animal's pane tabs: its Log and, for a tame one that
// can learn, Training.
func (a *App) animalTabs(an game.AnimalView) []paneTab {
	tabs := []paneTab{{ID: "inspector.log", Label: "Log", WantW: animalTabW, WantH: animalLogTabH,
		Declare: func(r rect) { a.declareAnimalLog(an, r) },
		Draw:    func(screen *ebiten.Image, r rect) { a.drawAnimalLogTab(screen, an, r) }}}
	if an.Tame && len(animalTrainable(an)) > 0 {
		tabs = append(tabs, paneTab{ID: "inspector.training", Label: "Training", WantW: animalTabW,
			WantH: animalTrainingTabH(an),
			Draw:  func(screen *ebiten.Image, r rect) { a.drawAnimalTrainingTab(screen, an, r) }})
	}
	return tabs
}

// ---- items ----

// itemPane is the inspect pane of the loose stack with id.
func (a *App) itemPane(id entity.EntityID) paneSpec {
	it, ok := a.itemView(id)
	if !ok {
		return paneSpec{}
	}
	lines := itemLines(it, inStockpile(a.renderer.world(), it.X, it.Y))
	if rot := itemRotLine(it); rot != "" { // right under the freshness line
		lines = append(lines[:1:1], append([]string{rot}, lines[1:]...)...)
	}
	return paneSpec{Title: itemTitle(it), Subtitle: itemCategory(it), Lines: lines, Close: "item.close"}
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
func (a *App) itemCommands(id entity.EntityID) []cmdButton {
	it, ok := a.itemView(id)
	if !ok {
		return nil
	}
	forbid := cmdButton{ID: "item.forbid", Label: "Forbid", Icon: "order_forbid", Toggle: true, On: it.Forbidden,
		Hotkey: input.Forbid, Desc: "Colonists leave it alone: no hauling, eating or using."}
	if it.Forbidden {
		forbid.Label, forbid.Icon = "Allow", "order_allow"
		forbid.Desc = "Let colonists haul, eat and use it again."
	}
	out := []cmdButton{forbid}
	if it.Unclaimed {
		mark := cmdButton{ID: "item.haulmark", Label: "Mark for hauling", Icon: "order_haul", Toggle: true, On: it.HaulMarked,
			Desc: "Colonists will haul it to a stockpile."}
		switch {
		case it.HaulMarked:
			mark.Label, mark.Desc = "Unmark", "Colonists leave it where it lies."
		case it.Forbidden: // colonists never haul a forbidden stack
			mark.Disabled, mark.Reason = true, "forbidden: allow it first"
		}
		out = append(out, mark)
	}
	for _, v := range itemVerbs(it, inStockpile(a.renderer.world(), it.X, it.Y)) {
		out = append(out, a.colonistVerbCmd(v.id, v.label, v.icon, v.desc, it.X, it.Y, v.match))
	}
	return out
}

// ---- plants ----

// plantSubtitle is the category and growth stage: "Crop · sprouting".
func plantSubtitle(p game.PlantView) string {
	return plantCategory(p) + " · " + plantStageWord(p)
}

// plantPane is the inspect pane of the plant with id.
func (a *App) plantPane(id entity.EntityID) paneSpec {
	p, ok := a.plantView(id)
	if !ok {
		return paneSpec{}
	}
	col := uiGood
	if plantCaptionWarns(p) {
		col = uiWarning // a roof, a lamp or a fire is needed
	}
	return paneSpec{Title: plantTitle(p), Subtitle: plantSubtitle(p), Close: "plant.close",
		Bars:  []paneBar{{Label: "Growth", Pct: p.Growth, Color: col}},
		Lines: plantPaneLines(p)}
}

// plantCommands are the plant's command buttons: Harvest (crops), Cut or
// Chop, and "now by" a colonist of your pick.
func (a *App) plantCommands(id entity.EntityID) []cmdButton {
	p, ok := a.plantView(id)
	if !ok {
		return nil
	}
	var out []cmdButton
	if !isTree(p) {
		h := cmdButton{ID: "plant.harvest", Label: "Harvest", Icon: "order_harvest", Toggle: true, On: p.Designated,
			Desc: "Mark it for harvest. A colonist picks it when it is ripe."}
		switch {
		case p.Designated:
			h.Label, h.Desc = "Cancel harvest", "Take the mark off: nobody will pick it."
		case !p.Harvestable:
			h.Disabled, h.Reason = true, "not ready"
		}
		out = append(out, h)
	}
	cut := cmdButton{ID: "plant.cut", Label: "Cut", Icon: "order_cut", Toggle: true, On: p.Cut,
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
