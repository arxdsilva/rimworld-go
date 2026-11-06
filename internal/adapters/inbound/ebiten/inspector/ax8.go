package inspector

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// AX-8: the escape ship in the building pane (MAIN_UI_SCREENS §37, the
// endgame ship): which ship a part belongs to, the reactor's countdown,
// a casket's sleeper and the launch report, with the Start reactor
// (behind a warning about the raids it draws), Launch ship and Wake
// sleeper buttons. Everything is read from BuildingView.AX and
// WorldView.AX; every click is a typed command.

// Ship button and dialog ids (journeys press them).
const (
	StartReactorBtnID  = "cmd.bldg.start_reactor"
	LaunchShipBtnID    = "cmd.bldg.launch_ship"
	WakeSleeperBtnID   = "cmd.bldg.wake_sleeper"
	ReactorWarnModalID = "ship.reactor.warn"
	ReactorWarnOKID    = "ship.reactor.start"
	LaunchModalID      = "ship.launch.confirm"
	LaunchOKID         = "ship.launch.go"
)

// reactorWarning is the explicit warning before the countdown (MAIN_UI
// §37: reactor activation warns of the defence it starts).
// IsShipPart reports one of the escape ship's six part kinds.
func IsShipPart(kind string) bool {
	switch kind {
	case "ship structural beam", "ship stasis casket", "ship computer core", "ship reactor", "ship engine", "ship sensor cluster":
		return true
	}
	return false
}

// ShipCountdown is a countdown in ticks as the panes and alerts say it
// ("12 days 4 hours").
func ShipCountdown(ticks int) string {
	d, h := ticks/clock.TicksPerDay, (ticks%clock.TicksPerDay)/clock.TicksPerHour
	switch {
	case d > 0 && h > 0:
		return fmt.Sprintf("%d %s %d %s", d, plural(d, "day", "days"), h, plural(h, "hour", "hours"))
	case d > 0:
		return fmt.Sprintf("%d %s", d, plural(d, "day", "days"))
	}
	return fmt.Sprintf("%d %s", max(h, 1), plural(max(h, 1), "hour", "hours"))
}

// ShipLines are a ship part's pane lines: the reactor's state, a
// casket's sleeper, and the launch report when the part belongs to the
// ship the report speaks of.
func ShipLines(b game.BuildingView, w game.WorldView) []string {
	if !IsShipPart(b.Kind) {
		return nil
	}
	var out []string
	switch b.Kind {
	case "ship reactor":
		switch c := b.AX.Countdown; {
		case c < 0:
			out = append(out, "Reactor asleep: starting it begins a 15-day waking that draws raids")
		case c > 0:
			out = append(out, "Reactor waking: "+ShipCountdown(c)+" left")
		default:
			out = append(out, "Reactor awake: ready to lift the ship")
		}
	case "ship stasis casket":
		if b.AX.Sleeper != "" {
			out = append(out, "Sleeper: "+b.AX.Sleeper)
		} else {
			out = append(out, "Empty: select a colonist and right-click the casket to board")
		}
	}
	if b.AX.ShipID == 0 || b.AX.ShipID != w.AX.ShipID {
		return append(out, "Not joined to the main ship (a casket joins only through a beam)")
	}
	if len(w.AX.LaunchIssues) == 0 {
		return append(out, "Launch report: ready for launch")
	}
	out = append(out, "Launch report:")
	for _, s := range w.AX.LaunchIssues {
		out = append(out, "  "+s)
	}
	return out
}

// shipCommands are a ship part's buttons (free functions: Building is
// at its method limit): Start reactor, Wake sleeper
// and Launch ship.
func shipCommands(a Building, b game.BuildingView) []Button {
	if !IsShipPart(b.Kind) {
		return nil
	}
	var out []Button
	if b.Kind == "ship reactor" {
		s := Button{ID: StartReactorBtnID, Label: a.shell.T("Start reactor"), Icon: "ship_reactor",
			Desc: a.shell.T("Begin the reactor's 15-day waking. Raiders and machines will come until it is awake."),
			Run:  func() { warnReactor(a, b) }}
		if b.AX.Countdown >= 0 {
			s.Disabled, s.Reason = true, "already started"
		}
		out = append(out, s)
	}
	if b.Kind == "ship stasis casket" && b.AX.Sleeper != "" {
		out = append(out, Button{ID: WakeSleeperBtnID, Label: a.shell.T("Wake sleeper"), Icon: "ship_casket",
			Desc: b.AX.Sleeper + a.shell.T(" climbs out of the casket and walks again."),
			Cmds: []command.Command{{Kind: command.LeaveShip, Target: uint64(b.ID)}}})
	}
	l := Button{ID: LaunchShipBtnID, Label: a.shell.T("Launch ship"), Icon: "ship_engine",
		Desc: a.shell.T("Lift the ship off the planet with everyone asleep in its caskets. This ends the colony's story."),
		Run:  func() { confirmLaunch(a, b) }}
	w := a.shell.World()
	if b.AX.ShipID == w.AX.ShipID && len(w.AX.LaunchIssues) > 0 {
		l.Disabled, l.Reason = true, w.AX.LaunchIssues[0]
		l.Desc += "\n\nStill missing:\n" + strings.Join(w.AX.LaunchIssues, "\n")
	}
	return append(out, l)
}

// confirmLaunch asks before the ship leaves: who goes, and that the
// story ends (MAIN_UI §37: launch confirmation).
func confirmLaunch(a Building, b game.BuildingView) {
	var aboard []string
	for _, v := range a.shell.World().Buildings {
		if v.AX.ShipID == b.AX.ShipID && v.AX.Sleeper != "" {
			aboard = append(aboard, v.AX.Sleeper)
		}
	}
	text := lang.Tf(a.shell, "The ship will leave with %s asleep aboard. Everyone else stays behind, and this colony's story ends.\n\nLaunch the ship now?", strings.Join(aboard, ", "))
	shipConfirm(a, LaunchModalID, a.shell.T("Launch the ship?"), text, LaunchOKID, a.shell.T("Launch"), command.Command{Kind: command.LaunchShip, Target: uint64(b.ID)})
}

// warnReactor asks before the reactor starts (the raids it draws).
func warnReactor(a Building, b game.BuildingView) {
	shipConfirm(a, ReactorWarnModalID, a.shell.T("Start the ship reactor?"),
		a.shell.T("Waking the reactor takes 15 days, and its call carries far: raiders and machines will come for it, again and again, until it is fully awake. If the reactor is destroyed, the waking is lost and must begin again.\n\nStart the reactor now?"),
		ReactorWarnOKID, a.shell.T("Start reactor"),
		command.Command{Kind: command.StartReactor, Target: uint64(b.ID)})
}

// shipConfirm pushes a yes/no dialog that sends c on yes.
func shipConfirm(a Building, id, title, text, okID, okLabel string, c command.Command) {
	lines := gfx.WrapPx(text, gfx.Body, 420-2*uikit.ModalPad)
	h := float32(uikit.ModalTitleH+uikit.ModalFooterH+2*uikit.ModalPad) + float32(len(lines))*float32(gfx.LineH(gfx.Body)) + 8
	a.kit().Push(uikit.Modal{
		ID: id, Title: title, W: 420, H: h,
		Draw: func(body gfx.Rect) {
			y := body.Y
			for _, ln := range lines {
				a.kit().Label(body.X, y, ln, gfx.Body)
				y += float32(gfx.LineH(gfx.Body))
			}
		},
		Buttons: []uikit.ModalButton{
			{ID: okID, Label: okLabel, Style: uikit.BtnPrimary, Closes: true, OnClick: func() { a.shell.Enqueue(c) }},
			{ID: "cancel", Label: a.shell.T("Cancel"), Closes: true},
		},
	})
}
