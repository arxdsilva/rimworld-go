package inspector

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
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
	gearWantW, gearWantH         = 400, 360
	logWantW, logWantH           = 400, 280
	bioWantW, bioWantH           = 480, 404
	healthWantW, healthWantH     = 480, 360
	socialWantW, socialWantH     = 480, 340
	prisonerWantW, prisonerWantH = 480, 300
)

// pawnCommands are colonist id's command buttons, in declaration order
// (hotkeys follow it): Draft, Clear orders, Equip nearby, Rest when hurt (a
// living colonist) and, for a free colonist, Capture and Arrest. A dead or
// downed colonist's commands are disabled with their status as the reason
// (Rest when hurt is a setting, so it stays usable).
func (a Pawn) PawnCommands(id entity.EntityID) []Button {
	p, ok := a.pawnView(id)
	if !ok {
		return nil
	}
	draft := Button{
		ID: ids.PawnDraft, Label: "Draft", Icon: "draft", Hotkey: input.Draft, Toggle: true, On: p.Drafted,
		Desc: "Take direct control. A drafted colonist drops their work and obeys orders: right-click to move or attack. " +
			"Undraft to send them back to their schedule.",
	}
	clear := Button{
		ID: ids.PawnClearOrders, Label: "Clear orders", Icon: "clear_orders",
		Desc: "Cancel the current order and everything queued behind it.",
	}
	switch {
	case p.Status != "alive":
		draft.Disabled, draft.Reason = true, p.Status
		clear.Disabled, clear.Reason = true, p.Status
	case len(p.QueuedOrders) == 0:
		clear.Disabled, clear.Reason = true, "no queued orders"
	}
	out := []Button{draft, clear, a.equipCommand(p)}
	out = append(out, a.combatCommands(p)...) // cmds_combat.go: a drafted colonist's fire, hold, grenade
	if c, ok := a.bedRestCommand(p); ok {
		out = append(out, c)
	}
	out = append(out, a.Tabs().prisonerCommands(p)...) // prisoner_ui.go: Capture / Arrest
	if p.Weapon != "" && p.Weapon != "fists" {         // nothing to drop when unarmed
		out = append(out, dropWeaponCommand(p))
	}
	out = append(out, smokepopCommand(p)...)
	out = append(out, threatCommand(p)...)
	return out
}

// DropWeaponID is the Gear tab's Drop weapon button (journeys press it).
const DropWeaponID = "cmd.pawn.dropweapon"

// dropWeaponCommand is Drop weapon: the colonist puts the weapon down where
// they stand (DropWeapon), to be picked up again by anyone.
func dropWeaponCommand(p game.PawnView) Button {
	c := Button{ID: DropWeaponID, Label: "Drop weapon", Icon: "equip",
		Desc: "Drop weapon\nPut the wielded weapon down where this colonist stands.",
		Cmds: []command.Command{{Kind: command.DropWeapon, Pawn: uint64(p.ID)}}}
	switch {
	case p.Status != "alive":
		c.Disabled, c.Reason = true, p.Status
	case p.Weapon == "" || p.Weapon == "fists":
		c.Disabled, c.Reason = true, "no weapon in hand"
	}
	return c
}

// bedRestCommand is the colonist's "Rest when hurt" toggle (PLAY-020): on,
// a colonist in pain, wounded or sick lies in bed to heal. Only for a living
// colonist: not a prisoner, a raider or a corpse. The click flips the setting
// as it stands then.
func (a Pawn) bedRestCommand(p game.PawnView) (Button, bool) {
	if p.Prisoner || p.Hostile || p.Embodied || p.Status == "dead" {
		return Button{}, false
	}
	id := p.ID
	return Button{
		ID: "cmd.pawn.bedrest", Label: "Rest when hurt", Icon: "bed_rest", Toggle: true, On: p.RestWhenHurt,
		Desc: "Lie in bed to heal when in pain, wounded or sick.",
		Run: func() {
			if pv, ok := a.pawnView(id); ok {
				a.shell.Enqueue(command.Command{Kind: command.SetPawnBedRest, Target: uint64(id), Enabled: !pv.RestWhenHurt})
			}
		},
	}, true
}

// equipCommand is Equip nearby: its options are the weapons and garments
// within reach (inspector.go's equipNearby), each enqueuing the game's own
// Equip or Wear order.
func (a Pawn) equipCommand(p game.PawnView) Button {
	id := p.ID
	c := Button{
		ID: "cmd.pawn.equip", Label: "Equip", Icon: "equip",
		Desc: "Equip nearby\nPick up a weapon or put on a garment lying within reach. Right-click, or use the arrow, to choose which.",
		Options: func() []Option {
			pv, ok := a.pawnView(id)
			if !ok {
				return nil
			}
			var out []Option
			for _, o := range a.equipNearby(pv) {
				cmd := o.Cmd
				out = append(out, Option{Label: o.Label, Cmd: &cmd})
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
func (a Pawn) PawnPane(id entity.EntityID) Spec {
	p, ok := a.pawnView(id)
	if !ok {
		return Spec{}
	}
	return Spec{
		Title:    a.shell.PawnLabel(p),
		Subtitle: PawnSubtitle(p),
		Bars:     pawnBars(p),
		Chips:    a.pawnChips(p),
		Lines:    a.PawnLines(p),
		Tabs:     a.pawnTabs(p),
		Close:    ids.InspectorClose,
	}
}

// pawnSubtitle is "Age 31, Convoy mechanic" (a prisoner, ally or visitor
// says so first; a legacy colonist with neither age nor title is just a
// colonist).
func PawnSubtitle(p game.PawnView) string {
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
func pawnBars(p game.PawnView) []Bar {
	return []Bar{
		{Label: "Health", Pct: barPct(p.Health), Color: HealthColor(p.Health)},
		{Label: "Mood", Pct: barPct(p.Mood), Color: MoodColor(p.Mood)},
		{Label: "Food", Pct: barPct(p.Hunger), Color: needColor(p.Hunger)},
		{Label: "Rest", Pct: barPct(p.Rest), Color: needColor(p.Rest)},
	}
}

func barPct(v int) int { return max(0, min(100, v)) }

// pawnChips are the allowed-area chip (when the game reports areas) and,
// for a colonist who keeps one, the schedule slot now (it opens the
// Schedule tab).
func (a Pawn) pawnChips(p game.PawnView) []Chip {
	var out []Chip
	if c, ok := a.shell.AreaChip(p); ok { // area_select.go
		out = append(out, c)
	}
	if !p.Prisoner {
		out = append(out, Chip{
			ID:    ids.TabSchedule,
			Label: "Schedule: " + p.Activity.String(),
			Tip:   "Schedule\nWhat " + a.shell.PawnLabel(p) + " is scheduled to do this hour. Click to open the Schedule tab.",
		})
	}
	return out
}

// pawnStatusLines are the conditions worth a line of their own, most
// pressing first: not alive, drafted, a mental break, exposure, infection
// or an untended wound, resting in bed to heal, and being held up by
// someone in the way.
func PawnStatusLines(p game.PawnView, name func(entity.EntityID) string) []string {
	var out []string
	if p.Status != "alive" {
		ln := TitleCase(p.Status)
		if p.Reason != "" {
			ln += " (" + p.Reason + ")"
		}
		out = append(out, ln)
	}
	if p.Drafted {
		out = append(out, "Drafted")
	}
	if p.CarriedBy != 0 {
		out = append(out, "Carried by "+carryName(name, p.CarriedBy))
	}
	if p.CarryingPawn != 0 {
		out = append(out, "Carrying "+carryName(name, p.CarryingPawn))
	}
	if ln := mentalBreakLine(p, name); ln != "" {
		out = append(out, ln)
	}
	if ln := guestLine(p); ln != "" {
		out = append(out, ln)
	}
	if ln := bedStateLine(p); ln != "" {
		out = append(out, ln)
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
	if ln := WaitingForLine(p.BlockedBy, name); ln != "" {
		out = append(out, TitleCase(ln))
	}
	return out
}

// carryName is the carried or carrying pawn's name, "someone" when the
// read model cannot resolve it.
func carryName(name func(entity.EntityID) string, id entity.EntityID) string {
	if n := name(id); n != "" {
		return n
	}
	return "someone"
}

// pawnLines are the pane's scrolling lines: the status, the job, what is
// put off or queued, the weapon and clothes, then where the colonist is.
func (a Pawn) PawnLines(p game.PawnView) []string {
	out := PawnStatusLines(p, a.CreatureName)
	if ln := InspirationLine(p.Inspiration); ln != "" {
		out = append(out, ln)
	}
	job := p.Job
	if job == "" {
		job = "idle"
	}
	out = append(out, "Job: "+job)
	if ln := PostponedLine(p.Postponed); ln != "" {
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
	out = append(out, fmt.Sprintf("Tile: (%d, %d)  %s", p.X, p.Y, a.shell.Temp(p.Temp)))
	if a.sim() != nil {
		if rt := world.RoomLabelAt(a.shell.World(), p.X, p.Y); rt != "" {
			out = append(out, "Room: "+rt)
		}
	}
	if ln := DirtyBootsLine(p); ln != "" {
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
func (a Pawn) pawnTabs(p game.PawnView) []Tab {
	id := p.ID
	bio := Tab{ID: "inspector.bio", Label: "Bio", WantW: bioWantW, WantH: bioWantH,
		Declare: func(r gfx.Rect) { a.Tabs().bioTab(id, r) }}
	if !HasBio(p) {
		bio.Disabled, bio.Reason = true, "No biography for this colonist"
	}
	gear := Tab{ID: "inspector.gear", Label: "Gear", WantW: gearWantW, WantH: gearWantH,
		Declare: func(r gfx.Rect) { a.Tabs().gearTabBody(id, r) }}
	if a.Tabs().isPrisoner(id) {
		gear = Tab{ID: PrisonerTabID, Label: "Prisoner", WantW: prisonerWantW, WantH: prisonerWantH,
			Declare: func(r gfx.Rect) { a.Tabs().prisonerTabBody(id, r) }}
	}
	return []Tab{
		{ID: "inspector.needs", Label: "Needs", WantW: needsWantW, WantH: needsWantH,
			Draw: func(screen *ebiten.Image, r gfx.Rect) { a.drawNeedsTab(screen, id, r) }},
		bio,
		{ID: "inspector.health", Label: "Health", WantW: healthWantW, WantH: healthWantH,
			Declare: func(r gfx.Rect) { a.Tabs().kitTabBody(id, "health", r) }},
		{ID: "inspector.social", Label: "Social", WantW: socialWantW, WantH: socialWantH,
			Declare: func(r gfx.Rect) { a.Tabs().kitTabBody(id, "social", r) }},
		gear,
		{ID: "inspector.log", Label: "Log", WantW: logWantW, WantH: logWantH,
			Draw: func(screen *ebiten.Image, r gfx.Rect) { a.drawPawnLog(screen, id, r) }},
	}
}
