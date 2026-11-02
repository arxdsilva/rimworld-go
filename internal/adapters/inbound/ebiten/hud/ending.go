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
	endingW            = 560
)

// endingTitle and endingText are the ship ending's original prose.
const (
	endingTitle = "Beyond the sky"
	endingText  = "The engines thundered and the ship climbed until the sky turned black. Below, the colony grew small: " +
		"its fields, its graves, the walls that held through every raid. Asleep in their caskets, the travellers dream " +
		"of a world where they can begin again. Their story here is complete."
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
	if !w.AX.ShipLaunched || st.ending {
		return
	}
	st.ending = true
	c.openEnding(w)
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
	done := 0
	for _, r := range w.Research {
		if r.Done {
			done++
		}
	}
	return append(out, fmt.Sprintf("The colony stood %d days and finished %d research projects.", int(now)/clock.TicksPerDay, done))
}

// openEnding pushes the ending screen.
func (c Right) openEnding(w game.WorldView) {
	inner := float64(endingW - 2*uikit.ModalPad)
	prose := gfx.WrapPx(endingText, gfx.Body, inner)
	var facts []string
	for _, l := range EndingLines(w, c.sh.Pawns(), c.sh.Tick()) {
		facts = append(facts, gfx.WrapPx(l, gfx.Body, inner)...)
	}
	lh := float32(gfx.LineH(gfx.Body))
	h := float32(uikit.ModalTitleH+uikit.ModalFooterH+2*uikit.ModalPad) + float32(len(prose)+len(facts))*lh + lh + 8
	buttons := []uikit.ModalButton{{ID: EndingWatchID, Label: "Keep watching", Style: uikit.BtnPrimary, Closes: true}}
	if d, ok := c.sh.(ports.Dispatcher); ok {
		buttons = append(buttons, uikit.ModalButton{ID: EndingMenuID, Label: "Main menu", Closes: true,
			OnClick: func() { d.Do(ports.ReturnToMenu{}) }})
	}
	c.kit().Push(uikit.Modal{
		ID: EndingModalID, Title: endingTitle, W: endingW, H: h,
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
