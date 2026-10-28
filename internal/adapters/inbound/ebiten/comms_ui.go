package ebiten

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/command"
)

// The radio console dialog (MAIN_UI_SCREENS §29, PLAY-024): a centred
// modal open while the game reports a comms session. Each faction row
// offers Gift (with a silver stepper), Request caravan and Request aid;
// the orbital row calls the trade ship. Every action shows its cost and,
// when it cannot be used, why.

const (
	commsModalID    = "colony.comms"
	commsModalW     = 760
	commsModalH     = 560
	commsRowH       = 100
	commsGiftStep   = 10
	commsGiftStart  = 40
	commsGoodwillLo = -100
	commsGoodwillHi = 100
)

// commsDraft is the open dialog: the newest view and the gift amounts.
type commsDraft struct {
	view game.CommsView
	gift map[int]int // faction id -> silver
}

// giftOf is the silver the stepper holds for faction id, kept inside what
// the beacons reach.
func (d *commsDraft) giftOf(id int) int {
	n, ok := d.gift[id]
	if !ok {
		n = commsGiftStart
	}
	return max(1, min(n, max(1, d.view.BeaconSilver)))
}

// stepGift moves faction id's gift amount.
func (d *commsDraft) stepGift(id, delta int) { d.gift[id] = d.giftOf(id) + delta }

// openComms opens the dialog for session v.
func (a *App) openComms(v game.CommsView) {
	d := &commsDraft{view: v, gift: map[int]int{}}
	a.dealings.comms = d
	a.uiPushModal(modal{
		id: commsModalID, title: "Radio console", w: commsModalW, h: commsModalH,
		draw:    func(a *App, body rect) { a.commsBody(d, body) },
		onClose: a.dismissComms,
		buttons: []modalButton{{id: "close", label: "Close", closes: true, onClick: a.dismissComms}},
	})
}

// dismissComms ends the session from the player's side.
func (a *App) dismissComms() {
	a.dealings.comms = nil
	a.dealings.commsDismissed = true
	a.shell.Enqueue(command.Command{Kind: command.CloseComms})
	a.dealingsPause()
}

// emit sends c if the renderer is up.
func (a *App) emit(c command.Command) {
	a.shell.Enqueue(c)
}

// commsActionLabel is a button label with its goodwill cost.
func commsActionLabel(base string, act game.CommsActionView) string {
	if act.Cost > 0 {
		return fmt.Sprintf("%s · %d goodwill", base, act.Cost)
	}
	return base
}

// commsReason is the action's blocked reason, with a fallback.
func commsReason(act game.CommsActionView) string {
	if act.Allowed {
		return ""
	}
	if act.Reason == "" {
		return "Not available."
	}
	return act.Reason
}

// commsBody declares the header, the faction rows and the orbital row.
func (a *App) commsBody(d *commsDraft, body rect) {
	v := d.view
	a.uiLabel(body.X, body.Y, fmt.Sprintf("%s is at the console · %d silver within reach of a trade beacon", v.OperatorName, v.BeaconSilver), tsLabel)
	orbH := float32(64)
	list := rect{X: body.X, Y: body.Y + 28, W: body.W, H: body.H - 28 - orbH - 8}
	orb := rect{X: body.X, Y: body.Bottom() - orbH, W: body.W, H: orbH}

	if len(v.Factions) == 0 {
		a.uiLabel(list.X, list.Y+6, "No other faction answers on the radio.", dimText())
	}
	area := a.uiScrollArea("comms.rows", list, float32(len(v.Factions)*commsRowH), scrollOpts{Step: commsRowH})
	y := area.View.Y - area.Off
	for _, f := range v.Factions {
		ry := y
		y += commsRowH
		if ry < area.View.Y || ry+commsRowH > area.View.Bottom() {
			continue
		}
		a.commsFaction(d, f, rect{X: area.View.X, Y: ry, W: area.View.W, H: commsRowH - 6})
	}

	a.uiSt().queue(func(dst *ebiten.Image) { strokeRect(dst, orb, uiPanelBorder) })
	a.uiLabel(orb.X+10, orb.Y+8, "Orbital trader", withColor(tsLabel, uiHeading))
	o := v.Orbital
	if !o.Present {
		a.uiLabel(orb.X+10, orb.Y+32, "No ship in range", dimText())
		return
	}
	a.uiLabel(orb.X+150, orb.Y+8, fmt.Sprintf("%s · %s · leaves in %d h", o.Ship, o.Kind, o.HoursLeft), tsBody)
	why := ""
	if !o.CallOK {
		why = o.CallReason
		if why == "" {
			why = "Not available."
		}
		a.uiLabel(orb.X+10, orb.Y+42, why, dimText())
	}
	if a.uiButton("comms.call", rect{X: orb.Right() - 190, Y: orb.Y + 14, W: 180, H: 32}, "Call trade ship", btnOpts{
		Style: btnPrimary, Disabled: why != "", Reason: why}) {
		a.emit(command.Command{Kind: command.CommsCallTrader})
	}
}

// commsFaction declares one faction's row.
func (a *App) commsFaction(d *commsDraft, f game.CommsFactionView, r rect) {
	a.uiSt().queue(func(dst *ebiten.Image) { strokeRect(dst, r, uiPanelBorder) })
	a.uiLabel(r.X+10, r.Y+8, fitText(f.Name, tsLabel, 230), withColor(tsLabel, uiHeading))
	rel := withColor(tsBody, uiTextDim)
	switch f.Relation {
	case "hostile":
		rel = withColor(tsBody, uiDanger)
	case "friendly":
		rel = withColor(tsBody, uiGood)
	}
	a.uiLabel(r.X+250, r.Y+8, f.Relation, rel)
	bar := rect{X: r.X + 340, Y: r.Y + 11, W: r.W - 340 - 70, H: 10}
	a.uiSt().queue(func(dst *ebiten.Image) { goodwillBar(dst, bar, f.Goodwill) })
	a.uiLabel(bar.Right()+8, r.Y+8, fmt.Sprintf("%+d", f.Goodwill), tsBody)
	if f.Relation == "hostile" {
		a.uiLabel(r.X+10, r.Y+40, "Hostile: they will not answer requests.", dimText())
		return
	}
	id := f.ID
	by := r.Y + 36
	h := float32(30)
	amount := d.giftOf(id)
	gift := f.Gift
	why := commsReason(gift)
	if gift.Allowed && d.view.BeaconSilver < 1 {
		why = "No silver within reach of a trade beacon."
	}
	if a.uiButton(fmt.Sprintf("comms.%d.giftless", id), rect{X: r.X + 10, Y: by, W: 30, H: h}, "-", btnOpts{Disabled: amount <= 1, Reason: "Least is 1 silver."}) {
		d.stepGift(id, -commsGiftStep)
	}
	a.uiLabel(r.X+46, by+7, fmt.Sprintf("%d", amount), tsLabel)
	if a.uiButton(fmt.Sprintf("comms.%d.giftmore", id), rect{X: r.X + 86, Y: by, W: 30, H: h}, "+", btnOpts{
		Disabled: amount >= d.view.BeaconSilver, Reason: "That is all the silver in reach."}) {
		d.stepGift(id, commsGiftStep)
	}
	earn := faction.GiftGoodwill(amount)
	if a.uiButton(fmt.Sprintf("comms.%d.gift", id), rect{X: r.X + 124, Y: by, W: 170, H: h}, "Gift silver", btnOpts{
		Disabled: why != "", Reason: why, Tip: fmt.Sprintf("Gift\nGives %d silver for about %+d goodwill.", amount, earn)}) {
		a.emit(command.Command{Kind: command.CommsGift, Index: id, Count: amount})
	}
	cWhy, aWhy := commsReason(f.Caravan), commsReason(f.Aid)
	if a.uiButton(fmt.Sprintf("comms.%d.caravan", id), rect{X: r.X + 304, Y: by, W: 210, H: h}, commsActionLabel("Request caravan", f.Caravan), btnOpts{
		Disabled: cWhy != "", Reason: cWhy}) {
		a.emit(command.Command{Kind: command.CommsRequestCaravan, Index: id})
	}
	if a.uiButton(fmt.Sprintf("comms.%d.aid", id), rect{X: r.X + 524, Y: by, W: 190, H: h}, commsActionLabel("Request aid", f.Aid), btnOpts{
		Disabled: aWhy != "", Reason: aWhy}) {
		a.emit(command.Command{Kind: command.CommsRequestAid, Index: id})
	}
	var notes []string
	for _, n := range []string{why, cWhy, aWhy} {
		if n != "" {
			notes = append(notes, n)
		}
	}
	if len(notes) > 0 {
		line := ""
		for i, n := range notes {
			if i > 0 {
				line += " · "
			}
			line += n
		}
		a.uiLabel(r.X+10, by+h+6, fitText(line, tsSmall, r.W-20), withColor(tsSmall, uiTextDim))
	}
}

// goodwillBar draws the -100..100 bar: a centre tick and a fill from it.
func goodwillBar(dst *ebiten.Image, r rect, gw int) {
	fillRect(dst, r, uiField)
	strokeRect(dst, r, uiPanelBorder)
	gw = max(commsGoodwillLo, min(commsGoodwillHi, gw))
	mid := r.X + r.W/2
	w := r.W / 2 * float32(abs(gw)) / commsGoodwillHi
	c := uiGood
	x := mid
	if gw < 0 {
		c, x = uiDanger, mid-w
	}
	fillRect(dst, rect{X: x, Y: r.Y + 1, W: w, H: r.H - 2}, c)
	fillRect(dst, rect{X: mid, Y: r.Y - 2, W: 1, H: r.H + 4}, uiTextDim)
}
