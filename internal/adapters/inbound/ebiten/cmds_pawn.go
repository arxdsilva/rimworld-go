package ebiten

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// A colonist's commands and pane (PLAY-018: PAWN-INSPECT owns them).
//
// The pane (400x188, lower-left) shows the name, age and title, four
// compact bars, two chips (the allowed area, the schedule slot now) and a
// scrolling list of lines; the old inspector's Needs, Bio, Health, Social,
// Gear, Log and Prisoner tabs open as panels above it, and the old action
// bar is the command buttons right of it. The old Character tab is the
// pane itself.

// The expanded panels' wanted sizes. The layout clamps the width to
// 400..560 and the height to the room under the colonist bar, and a body
// gets the panel inset by paneInset, so every body also copes with less.
const (
	needsWantW, needsWantH       = 400, 260
	gearWantW, gearWantH         = 400, 220
	logWantW, logWantH           = 400, 280
	bioWantW, bioWantH           = 480, 404
	healthWantW, healthWantH     = 480, 360
	socialWantW, socialWantH     = 480, 340
	prisonerWantW, prisonerWantH = 480, 300
)

// paneAreaChipID is the allowed-area chip's id (it opens the area picker).
const paneAreaChipID = "inspector.area"

// pawnCommands are colonist id's command buttons, in declaration order
// (hotkeys follow it): Draft, Clear orders, Equip nearby, Rest when hurt (a
// living colonist) and, for a free colonist, Capture and Arrest. A dead or
// downed colonist's commands are disabled with their status as the reason
// (Rest when hurt is a setting, so it stays usable).
func (a *App) pawnCommands(id entity.EntityID) []cmdButton {
	p, ok := a.pawnView(id)
	if !ok {
		return nil
	}
	draft := cmdButton{
		ID: "pawn.draft", Label: "Draft", Icon: "draft", Hotkey: input.Draft, Toggle: true, On: p.Drafted,
		Desc: "Take direct control. A drafted colonist drops their work and obeys orders: right-click to move or attack. " +
			"Undraft to send them back to their schedule.",
	}
	clear := cmdButton{
		ID: "pawn.clearorders", Label: "Clear orders", Icon: "clear_orders",
		Desc: "Cancel the current order and everything queued behind it.",
	}
	switch {
	case p.Status != "alive":
		draft.Disabled, draft.Reason = true, p.Status
		clear.Disabled, clear.Reason = true, p.Status
	case len(p.QueuedOrders) == 0:
		clear.Disabled, clear.Reason = true, "no queued orders"
	}
	out := []cmdButton{draft, clear, a.equipCommand(p)}
	if c, ok := bedRestCommand(p); ok {
		out = append(out, c)
	}
	return append(out, a.prisonerCommands(p)...) // prisoner_ui.go: Capture / Arrest
}

// bedRestCommand is the colonist's "Rest when hurt" toggle (PLAY-020): on,
// a colonist in pain, wounded or sick lies in bed to heal. Only for a living
// colonist: not a prisoner, a raider or a corpse. The click flips the setting
// as it stands then.
func bedRestCommand(p game.PawnView) (cmdButton, bool) {
	if p.Prisoner || p.Hostile || p.Embodied || p.Status == "dead" {
		return cmdButton{}, false
	}
	id := p.ID
	return cmdButton{
		ID: "cmd.pawn.bedrest", Label: "Rest when hurt", Icon: "bed_rest", Toggle: true, On: p.RestWhenHurt,
		Desc: "Lie in bed to heal when in pain, wounded or sick.",
		Run: func(a *App) {
			if pv, ok := a.pawnView(id); ok {
				a.renderer.enqueue(command.Command{Kind: command.SetPawnBedRest, Target: uint64(id), Enabled: !pv.RestWhenHurt})
			}
		},
	}, true
}

// equipCommand is Equip nearby: its options are the weapons and garments
// within reach (inspector.go's equipNearby), each enqueuing the game's own
// Equip or Wear order.
func (a *App) equipCommand(p game.PawnView) cmdButton {
	id := p.ID
	c := cmdButton{
		ID: "cmd.pawn.equip", Label: "Equip", Icon: "equip",
		Desc: "Equip nearby\nPick up a weapon or put on a garment lying within reach. Right-click, or use the arrow, to choose which.",
		Options: func(a *App) []cmdOption {
			pv, ok := a.pawnView(id)
			if !ok {
				return nil
			}
			var out []cmdOption
			for _, o := range a.equipNearby(pv) {
				cmd := o.Cmd
				out = append(out, cmdOption{Label: o.Label, Cmd: &cmd})
			}
			return out
		},
	}
	switch {
	case p.Status != "alive":
		c.Disabled, c.Reason = true, p.Status
	case len(a.equipNearby(p)) == 0:
		c.Disabled, c.Reason = true, "nothing to equip in reach"
	}
	return c
}

// pawnPane is colonist id's pane: name, age and title, status bars, the
// area and schedule chips, lines, and the Needs/Bio/Health/Social/Gear/Log
// tabs (Gear gives way to Prisoner for a prisoner). Title is "" for a
// colonist the snapshot does not have.
func (a *App) pawnPane(id entity.EntityID) paneSpec {
	p, ok := a.pawnView(id)
	if !ok {
		return paneSpec{}
	}
	return paneSpec{
		Title:    pawnLabel(p),
		Subtitle: pawnSubtitle(p),
		Bars:     pawnBars(p),
		Chips:    a.pawnChips(p),
		Lines:    a.pawnLines(p),
		Tabs:     a.pawnTabs(p),
		Close:    "inspector.close",
	}
}

// pawnSubtitle is "Age 31, Convoy mechanic" (a prisoner, ally or visitor
// says so first; a legacy colonist with neither age nor title is just a
// colonist).
func pawnSubtitle(p game.PawnView) string {
	var parts []string
	switch {
	case p.Prisoner:
		parts = append(parts, "Prisoner")
	case p.Ally:
		parts = append(parts, "Ally")
	case p.Visitor:
		parts = append(parts, "Visitor")
	}
	if p.AgeYears > 0 {
		parts = append(parts, fmt.Sprintf("Age %d", p.AgeYears))
	}
	if p.Title != "" && p.Title != "—" {
		parts = append(parts, p.Title)
	}
	if len(parts) == 0 {
		return "Colonist"
	}
	return strings.Join(parts, ", ")
}

// pawnBars are the pane's compact bars: health, mood, food and rest.
func pawnBars(p game.PawnView) []paneBar {
	return []paneBar{
		{Label: "Health", Pct: barPct(p.Health), Color: healthColor(p.Health)},
		{Label: "Mood", Pct: barPct(p.Mood), Color: moodColor(p.Mood)},
		{Label: "Food", Pct: barPct(p.Hunger), Color: needColor(p.Hunger)},
		{Label: "Rest", Pct: barPct(p.Rest), Color: needColor(p.Rest)},
	}
}

func barPct(v int) int { return max(0, min(100, v)) }

// pawnChips are the allowed-area chip (when the game reports areas) and,
// for a colonist who keeps one, the schedule slot now (it opens the
// Schedule tab).
func (a *App) pawnChips(p game.PawnView) []paneChip {
	var out []paneChip
	if c, ok := a.areaChip(p); ok { // area_select.go
		out = append(out, c)
	}
	if !p.Prisoner {
		out = append(out, paneChip{
			ID:    "tab.schedule",
			Label: "Schedule: " + p.Activity.String(),
			Tip:   "Schedule\nWhat " + pawnLabel(p) + " is scheduled to do this hour. Click to open the Schedule tab.",
		})
	}
	return out
}

// pawnStatusLines are the conditions worth a line of their own, most
// pressing first: not alive, drafted, a mental break, exposure, infection
// or an untended wound, resting in bed to heal, and being held up by
// someone in the way.
func pawnStatusLines(p game.PawnView, name func(entity.EntityID) string) []string {
	var out []string
	if p.Status != "alive" {
		ln := titleCase(p.Status)
		if p.Reason != "" {
			ln += " (" + p.Reason + ")"
		}
		out = append(out, ln)
	}
	if p.Drafted {
		out = append(out, "Drafted")
	}
	if p.MentalBreak != "" {
		out = append(out, "MENTAL BREAK: "+p.MentalBreak)
	}
	if p.Hypothermia > 0 {
		out = append(out, fmt.Sprintf("Hypothermia %d%%", p.Hypothermia/10))
	}
	if p.Heatstroke > 0 {
		out = append(out, fmt.Sprintf("Heatstroke %d%%", p.Heatstroke/10))
	}
	if p.Bleeding && p.Status != "dead" { // MED-01
		out = append(out, fmt.Sprintf("Bleeding (death in %dh)", p.BleedDeathHours))
	}
	switch {
	case p.Infection > 0 && p.InfectionLosing: // MED-02: the race against immunity
		out = append(out, fmt.Sprintf("Infection: losing (%d%%)", p.Infection))
	case p.Infection > 0:
		out = append(out, fmt.Sprintf("Infection: winning (%d%%)", p.Infection))
	case p.Wounded:
		out = append(out, "Wound untended")
	}
	if p.Tended && (p.Infection > 0 || p.Health < 100) {
		out = append(out, "Wounds tended")
	}
	if ln := waitingForLine(p.BlockedBy, name); ln != "" {
		out = append(out, titleCase(ln))
	}
	return out
}

// pawnLines are the pane's scrolling lines: the status, the job, what is
// put off or queued, the weapon and clothes, then where the colonist is.
func (a *App) pawnLines(p game.PawnView) []string {
	out := pawnStatusLines(p, a.creatureName)
	job := p.Job
	if job == "" {
		job = "idle"
	}
	out = append(out, "Job: "+job)
	if ln := postponedLine(p.Postponed); ln != "" {
		out = append(out, ln)
	}
	if n := len(p.QueuedOrders); n > 0 {
		out = append(out, fmt.Sprintf("Queued orders %d/%d", n, game.MaxQueuedOrders))
		for i, q := range p.QueuedOrders {
			out = append(out, fmt.Sprintf("%d. %s", i+1, q.Label))
		}
	}
	if p.Weapon != "" {
		out = append(out, "Weapon: "+p.Weapon)
	}
	out = append(out, "Wearing: "+wearingSummary(p))
	out = append(out, fmt.Sprintf("Tile: (%d, %d)  %s", p.X, p.Y, fmtTemp(p.Temp)))
	if a.renderer != nil {
		if rt := roomLabelAt(a.renderer.world(), p.X, p.Y); rt != "" {
			out = append(out, "Room: "+rt)
		}
	}
	if ln := dirtyBootsLine(p); ln != "" {
		out = append(out, ln)
	}
	if p.Temperament != "" {
		out = append(out, "Temperament: "+p.Temperament)
	}
	return out
}

// wearingSummary is the garments over everyday clothes, as one line.
func wearingSummary(p game.PawnView) string {
	s := "everyday clothes"
	if len(p.Apparel) > 0 {
		s = strings.Join(p.Apparel, ", ")
	}
	if p.ChangingInto != "" {
		s += " (changing into " + p.ChangingInto + ")"
	}
	return s
}

// pawnTabs are the tabs that open above the pane. Bio is disabled for a
// colonist without a biography; a prisoner's Gear gives way to Prisoner.
// Needs, Gear and Log draw the legacy way; Bio, Health, Social and Prisoner
// declare widget-kit bodies.
func (a *App) pawnTabs(p game.PawnView) []paneTab {
	id := p.ID
	bio := paneTab{ID: "inspector.bio", Label: "Bio", WantW: bioWantW, WantH: bioWantH,
		Declare: func(r rect) { a.bioTab(id, r) }}
	if !hasBio(p) {
		bio.Disabled, bio.Reason = true, "No biography for this colonist"
	}
	gear := paneTab{ID: "inspector.gear", Label: "Gear", WantW: gearWantW, WantH: gearWantH,
		Draw: func(screen *ebiten.Image, r rect) { a.drawGearTab(screen, id, r) }}
	if a.isPrisoner(id) {
		gear = paneTab{ID: prisonerTabID, Label: "Prisoner", WantW: prisonerWantW, WantH: prisonerWantH,
			Declare: func(r rect) { a.prisonerTabBody(id, r) }}
	}
	return []paneTab{
		{ID: "inspector.needs", Label: "Needs", WantW: needsWantW, WantH: needsWantH,
			Draw: func(screen *ebiten.Image, r rect) { a.drawNeedsTab(screen, id, r) }},
		bio,
		{ID: "inspector.health", Label: "Health", WantW: healthWantW, WantH: healthWantH,
			Declare: func(r rect) { a.kitTabBody(id, "health", r) }},
		{ID: "inspector.social", Label: "Social", WantW: socialWantW, WantH: socialWantH,
			Declare: func(r rect) { a.kitTabBody(id, "social", r) }},
		gear,
		{ID: "inspector.log", Label: "Log", WantW: logWantW, WantH: logWantH,
			Draw: func(screen *ebiten.Image, r rect) { a.drawPawnLog(screen, id, r) }},
	}
}
