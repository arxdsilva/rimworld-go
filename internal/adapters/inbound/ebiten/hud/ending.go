package hud

import (
	"fmt"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// The escape ship in the HUD (AX-8): the reactor's countdown and the
// ready ship as alerts (MAIN_UI §32), and the ending screen (MAIN_UI §38,
// victory and campaign summary) that opens by itself, once a session,
// when the ship has left. All of it reads WorldView.AX; the only actions
// are closing the screen to keep watching the colony and the shell's
// return to the main menu.

// Ship alert keys and the ending screen's ids (journeys press them).
const (
	ShipWakingAlertKey = "shipwaking"
	ShipReadyAlertKey  = "shipready"
	EndingModalID      = "ending.ship"
	EndingWatchID      = "ending.watch"
	EndingMenuID       = "ending.menu"
	GoneAlertKey       = "gone"
	GoneModalID        = "ending.gone" // everyone is gone (MAP-14)
	GoneWatchID        = "ending.gone.watch"
	GoneLoadID         = "ending.gone.load"
	GoneMenuID         = "ending.gone.menu"
	GoneWanderersID    = "ending.gone.wanderers" // New wanderers (MAP-19)
	endingW            = 560
	// goneLoadReason is why Load stays off in a commitment colony: the same
	// words as the pause menu's (options.CommitLoadReason; hud may not
	// import options).
	goneLoadReason = "Commitment: one timeline"
)

// endingTitle and endingText are the ship ending's original prose.
const (
	endingTitle = "Beyond the sky"
	endingText  = "The engines thundered and the ship climbed until the sky turned black. Below, the colony grew small: " +
		"its fields, its graves, the walls that held through every raid. Asleep in their caskets, the travellers dream " +
		"of a world where they can begin again. Their story here is complete."
)

// goneTitle and goneText are the everyone-gone screen's original prose.
const (
	goneTitle = "Nobody left"
	goneText  = "The last of the colony is gone: fallen, taken or lost beyond the horizon. The fires have burned down and the " +
		"doors stand open to the wind. Nothing here answers to you any more, but the world carries on and you may watch it."
)

// shipAlerts are the reactor's countdown (with the raids it draws) and a
// ship ready to launch.
func shipAlerts(w game.WorldView) []Alert {
	if w.AX.ShipLaunched || w.AX.ShipID == 0 {
		return nil
	}
	var at [][2]int
	for i := range w.Buildings {
		if b := &w.Buildings[i]; b.AX.ShipID == w.AX.ShipID && b.Kind == "ship reactor" {
			at = append(at, [2]int{b.X, b.Y})
		}
	}
	switch {
	case w.AX.Countdown > 0:
		return []Alert{{Key: ShipWakingAlertKey, Label: "Ship reactor waking: " + countdown(w.AX.Countdown), Targets: at}}
	case len(w.AX.LaunchIssues) == 0:
		return []Alert{{Key: ShipReadyAlertKey, Label: "Ship ready to launch", Targets: at}}
	}
	return nil
}

// GoneAlert is "Nobody left" while everyone is gone (MAP-19), with the wait
// for new wanderers when that option exists in this game. A click reopens the
// everyone-gone screen, so Keep watching never loses the way back.
func GoneAlert(w game.WorldView) (Alert, bool) {
	ax := w.AX
	if !ax.EveryoneGone {
		return Alert{}, false
	}
	label := goneTitle
	switch {
	case ax.WanderersMax == 0 || ax.WanderersLeft == 0: // no such option, or all seven groups used
	case ax.WanderersOK:
		label += ": new wanderers can come"
	default:
		label += ": wanderers in " + countdown(ax.WanderersIn)
	}
	return Alert{Key: GoneAlertKey, Label: label}, true
}

// countdown is ticks as days and hours ("12 days 4 hours").
func countdown(ticks int) string {
	d, h := ticks/clock.TicksPerDay, (ticks%clock.TicksPerDay)/clock.TicksPerHour
	if d > 0 {
		return fmt.Sprintf("%dd %dh", d, h)
	}
	return fmt.Sprintf("%dh", max(h, 1))
}

// declareEnding opens the ending screen once a session when the ship has
// left (DeclareLetters calls it every Update).
func (c Right) declareEnding() {
	w, st := c.sh.World(), c.session()
	if w.AX.ShipLaunched && !st.ending {
		st.ending, st.goneArmed = true, false // this screen also says what the other one would
		c.openEnding(w)
		return
	}
	c.declareGone(w, st)
}

// declareGone opens the everyone-gone screen (MAP-14) when
// WorldView.AX.EveryoneGone turns true, once until it goes false again: Keep
// watching does not make it come back every frame, a colonist who joins later
// re-arms it, and a session that starts with nobody (a save loaded after the
// end, a map with no colonist yet) does not open it.
func (c Right) declareGone(w game.WorldView, st *rightState) {
	if !w.AX.EveryoneGone {
		st.goneArmed = true
		return
	}
	if st.goneArmed {
		st.goneArmed = false
		c.openGone(w)
	}
}

// EndingLines are the ending screen's lines below its prose: who left,
// who stayed, how long the colony stood and what it learned.
func EndingLines(w game.WorldView, pawns []game.PawnView, now clock.Tick) []string {
	out := []string{"Aboard the ship: " + strings.Join(w.AX.Escaped, ", ")}
	var stayed []string
	for _, p := range pawns {
		if !p.Hostile && !p.Prisoner && p.Status != "dead" {
			stayed = append(stayed, p.Name)
		}
	}
	if len(stayed) > 0 {
		out = append(out, "Left behind: "+strings.Join(stayed, ", "))
	}
	return append(out, standingLine(w, now))
}

// standingLine is how long the colony stood and what it learned.
func standingLine(w game.WorldView, now clock.Tick) string {
	done := 0
	for _, r := range w.Research {
		if r.Done {
			done++
		}
	}
	return fmt.Sprintf("The colony stood %d days and finished %d research projects.", int(now)/clock.TicksPerDay, done)
}

// openEnding pushes the ending screen.
func (c Right) openEnding(w game.WorldView) {
	buttons := []uikit.ModalButton{{ID: EndingWatchID, Label: "Keep watching", Style: uikit.BtnPrimary, Closes: true}}
	if d, ok := c.sh.(ports.Dispatcher); ok {
		if w.AX.EveryoneGone { // MAP-22: all aboard: new wanderers may still come (wiki: Ending)
			buttons = c.wanderersButton(buttons, w, d)
		}
		buttons = append(buttons, uikit.ModalButton{ID: EndingMenuID, Label: "Main menu", Closes: true,
			OnClick: func() { d.Do(ports.ReturnToMenu{}) }})
	}
	c.pushEnding(EndingModalID, endingTitle, endingText, EndingLines(w, c.sh.Pawns(), c.sh.Tick()), buttons)
}

// openGone pushes the everyone-gone screen: Keep watching, New wanderers
// (off until the wait is over), Load game (off
// in a commitment colony, as in the pause menu) and Main menu.
func (c Right) openGone(w game.WorldView) {
	buttons := []uikit.ModalButton{{ID: GoneWatchID, Label: "Keep watching", Style: uikit.BtnPrimary, Closes: true}}
	if d, ok := c.sh.(ports.Dispatcher); ok {
		buttons = c.wanderersButton(buttons, w, d)
		buttons = append(buttons, uikit.ModalButton{ID: GoneLoadID, Label: "Load game", Closes: true,
			Disabled: func() string {
				if s, ok := c.sh.Env().Game.(storyReader); ok && s.Story().Commitment {
					return goneLoadReason
				}
				return ""
			},
			OnClick: func() { d.Do(ports.Push{Screen: ports.ScreenLoad}) }},
			uikit.ModalButton{ID: GoneMenuID, Label: "Main menu", Closes: true,
				OnClick: func() { d.Do(ports.ReturnToMenu{}) }})
	}
	c.pushEnding(GoneModalID, goneTitle, goneText, []string{standingLine(w, c.sh.Tick())}, buttons)
}

// wanderersButton appends New wanderers (MAP-19), in a game made from a Setup.
func (c Right) wanderersButton(buttons []uikit.ModalButton, w game.WorldView, d ports.Dispatcher) []uikit.ModalButton {
	if w.AX.WanderersMax == 0 {
		return buttons
	}
	return append(buttons, uikit.ModalButton{ID: GoneWanderersID, Label: "New wanderers", Closes: true,
		Disabled: c.wanderersReason,
		OnClick:  func() { d.Do(ports.OpenWanderers{}) }})
}

// wanderersReason is why New wanderers is off: the wait, in hours, or the
// seven groups used up ("" it is on).
func (c Right) wanderersReason() string {
	ax := c.sh.World().AX
	switch {
	case ax.WanderersLeft == 0:
		return "No more groups"
	case !ax.WanderersOK:
		return "In " + countdown(ax.WanderersIn)
	}
	return ""
}

// storyReader is the colony's story read model (optional on Reader; the
// game implements it): the commitment flag.
type storyReader interface{ Story() game.StoryView }

// pushEnding sizes and pushes an ending-style modal: prose, a blank line,
// the facts in dim text, and the footer buttons.
func (c Right) pushEnding(id, title, text string, lines []string, buttons []uikit.ModalButton) {
	inner := float64(endingW - 2*uikit.ModalPad)
	prose := gfx.WrapPx(text, gfx.Body, inner)
	var facts []string
	for _, l := range lines {
		facts = append(facts, gfx.WrapPx(l, gfx.Body, inner)...)
	}
	lh := float32(gfx.LineH(gfx.Body))
	h := float32(uikit.ModalTitleH+uikit.ModalFooterH+2*uikit.ModalPad) + float32(len(prose)+len(facts))*lh + lh + 8
	c.kit().Push(uikit.Modal{
		ID: id, Title: title, W: endingW, H: h,
		Draw: func(body gfx.Rect) {
			y := body.Y
			for _, ln := range prose {
				c.kit().Label(body.X, y, ln, gfx.Body)
				y += lh
			}
			y += lh
			for _, ln := range facts {
				c.kit().Label(body.X, y, ln, uikit.DimText())
				y += lh
			}
		},
		Buttons: buttons,
	})
}
