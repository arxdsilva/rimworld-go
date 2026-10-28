package dialogs

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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
	CommsModalID    = "colony.comms"
	commsModalW     = 760
	commsModalH     = 560
	commsRowH       = 100
	CommsGiftStep   = 10
	CommsGiftStart  = 40
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
		n = CommsGiftStart
	}
	return max(1, min(n, max(1, d.view.BeaconSilver)))
}

// stepGift moves faction id's gift amount.
func (d *commsDraft) stepGift(id, delta int) { d.gift[id] = d.giftOf(id) + delta }

// openComms opens the dialog for session v.
func (dl *Dealings) openComms(v game.CommsView) {
	d := &commsDraft{view: v, gift: map[int]int{}}
	dl.comms = d
	dl.kit().Push(uikit.Modal{
		ID: CommsModalID, Title: "Radio console", W: commsModalW, H: commsModalH,
		Draw:    func(body gfx.Rect) { dl.commsBody(d, body) },
		OnClose: dl.dismissComms,
		Buttons: []uikit.ModalButton{{ID: "close", Label: "Close", Closes: true, OnClick: dl.dismissComms}},
	})
}

// dismissComms ends the session from the player's side.
func (dl *Dealings) dismissComms() {
	dl.comms = nil
	dl.commsDismissed = true
	dl.sh.Enqueue(command.Command{Kind: command.CloseComms})
	dl.pause()
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
func (dl *Dealings) commsBody(d *commsDraft, body gfx.Rect) {
	v := d.view
	dl.kit().Label(body.X, body.Y, fmt.Sprintf("%s is at the console · %d silver within reach of a trade beacon", v.OperatorName, v.BeaconSilver), gfx.Label)
	orbH := float32(64)
	list := gfx.Rect{X: body.X, Y: body.Y + 28, W: body.W, H: body.H - 28 - orbH - 8}
	orb := gfx.Rect{X: body.X, Y: body.Bottom() - orbH, W: body.W, H: orbH}

	if len(v.Factions) == 0 {
		dl.kit().Label(list.X, list.Y+6, "No other faction answers on the radio.", uikit.DimText())
	}
	area := dl.kit().ScrollArea("comms.rows", list, float32(len(v.Factions)*commsRowH), uikit.ScrollOpts{Step: commsRowH})
	y := area.View.Y - area.Off
	for _, f := range v.Factions {
		ry := y
		y += commsRowH
		if ry < area.View.Y || ry+commsRowH > area.View.Bottom() {
			continue
		}
		dl.commsFaction(d, f, gfx.Rect{X: area.View.X, Y: ry, W: area.View.W, H: commsRowH - 6})
	}

	dl.kit().Queue(func(dst *ebiten.Image) { uikit.StrokeRect(dst, orb, uikit.PanelBorder) })
	dl.kit().Label(orb.X+10, orb.Y+8, "Orbital trader", gfx.WithColor(gfx.Label, uikit.Heading))
	o := v.Orbital
	if !o.Present {
		dl.kit().Label(orb.X+10, orb.Y+32, "No ship in range", uikit.DimText())
		return
	}
	dl.kit().Label(orb.X+150, orb.Y+8, fmt.Sprintf("%s · %s · leaves in %d h", o.Ship, o.Kind, o.HoursLeft), gfx.Body)
	why := ""
	if !o.CallOK {
		why = o.CallReason
		if why == "" {
			why = "Not available."
		}
		dl.kit().Label(orb.X+10, orb.Y+42, why, uikit.DimText())
	}
	if dl.kit().Button("comms.call", gfx.Rect{X: orb.Right() - 190, Y: orb.Y + 14, W: 180, H: 32}, "Call trade ship", uikit.BtnOpts{
		Style: uikit.BtnPrimary, Disabled: why != "", Reason: why}) {
		dl.sh.Enqueue(command.Command{Kind: command.CommsCallTrader})
	}
}

// commsFaction declares one faction's row.
func (dl *Dealings) commsFaction(d *commsDraft, f game.CommsFactionView, r gfx.Rect) {
	dl.kit().Queue(func(dst *ebiten.Image) { uikit.StrokeRect(dst, r, uikit.PanelBorder) })
	dl.kit().Label(r.X+10, r.Y+8, uikit.FitText(f.Name, gfx.Label, 230), gfx.WithColor(gfx.Label, uikit.Heading))
	rel := gfx.WithColor(gfx.Body, uikit.TextDim)
	switch f.Relation {
	case "hostile":
		rel = gfx.WithColor(gfx.Body, uikit.Danger)
	case "friendly":
		rel = gfx.WithColor(gfx.Body, uikit.Good)
	}
	dl.kit().Label(r.X+250, r.Y+8, f.Relation, rel)
	bar := gfx.Rect{X: r.X + 340, Y: r.Y + 11, W: r.W - 340 - 70, H: 10}
	dl.kit().Queue(func(dst *ebiten.Image) { goodwillBar(dst, bar, f.Goodwill) })
	dl.kit().Label(bar.Right()+8, r.Y+8, fmt.Sprintf("%+d", f.Goodwill), gfx.Body)
	if f.Relation == "hostile" {
		dl.kit().Label(r.X+10, r.Y+40, "Hostile: they will not answer requests.", uikit.DimText())
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
	if dl.kit().Button(fmt.Sprintf("comms.%d.giftless", id), gfx.Rect{X: r.X + 10, Y: by, W: 30, H: h}, "-", uikit.BtnOpts{Disabled: amount <= 1, Reason: "Least is 1 silver."}) {
		d.stepGift(id, -CommsGiftStep)
	}
	dl.kit().Label(r.X+46, by+7, fmt.Sprintf("%d", amount), gfx.Label)
	if dl.kit().Button(fmt.Sprintf("comms.%d.giftmore", id), gfx.Rect{X: r.X + 86, Y: by, W: 30, H: h}, "+", uikit.BtnOpts{
		Disabled: amount >= d.view.BeaconSilver, Reason: "That is all the silver in reach."}) {
		d.stepGift(id, CommsGiftStep)
	}
	earn := faction.GiftGoodwill(amount)
	if dl.kit().Button(fmt.Sprintf("comms.%d.gift", id), gfx.Rect{X: r.X + 124, Y: by, W: 170, H: h}, "Gift silver", uikit.BtnOpts{
		Disabled: why != "", Reason: why, Tip: fmt.Sprintf("Gift\nGives %d silver for about %+d goodwill.", amount, earn)}) {
		dl.sh.Enqueue(command.Command{Kind: command.CommsGift, Index: id, Count: amount})
	}
	cWhy, aWhy := commsReason(f.Caravan), commsReason(f.Aid)
	if dl.kit().Button(fmt.Sprintf("comms.%d.caravan", id), gfx.Rect{X: r.X + 304, Y: by, W: 210, H: h}, commsActionLabel("Request caravan", f.Caravan), uikit.BtnOpts{
		Disabled: cWhy != "", Reason: cWhy}) {
		dl.sh.Enqueue(command.Command{Kind: command.CommsRequestCaravan, Index: id})
	}
	if dl.kit().Button(fmt.Sprintf("comms.%d.aid", id), gfx.Rect{X: r.X + 524, Y: by, W: 190, H: h}, commsActionLabel("Request aid", f.Aid), uikit.BtnOpts{
		Disabled: aWhy != "", Reason: aWhy}) {
		dl.sh.Enqueue(command.Command{Kind: command.CommsRequestAid, Index: id})
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
		dl.kit().Label(r.X+10, by+h+6, uikit.FitText(line, gfx.Small, r.W-20), gfx.WithColor(gfx.Small, uikit.TextDim))
	}
}

// goodwillBar draws the -100..100 bar: a centre tick and a fill from it.
func goodwillBar(dst *ebiten.Image, r gfx.Rect, gw int) {
	uikit.FillRect(dst, r, uikit.Field)
	uikit.StrokeRect(dst, r, uikit.PanelBorder)
	gw = max(commsGoodwillLo, min(commsGoodwillHi, gw))
	mid := r.X + r.W/2
	w := r.W / 2 * float32(absInt(gw)) / commsGoodwillHi
	c := uikit.Good
	x := mid
	if gw < 0 {
		c, x = uikit.Danger, mid-w
	}
	uikit.FillRect(dst, gfx.Rect{X: x, Y: r.Y + 1, W: w, H: r.H - 2}, c)
	uikit.FillRect(dst, gfx.Rect{X: mid, Y: r.Y - 2, W: 1, H: r.H + 4}, uikit.TextDim)
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
