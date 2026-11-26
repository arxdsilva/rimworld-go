// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package worldview

import (
	"fmt"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game/tabview"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/command"
)

const marketModalID = "world.market"

type marketDialog struct{ *View }

// marketDraft is presentation-only. The server rechecks the offer against
// live stock, cargo, negotiator and the settlement before moving any goods.
type marketDraft struct {
	view       tabview.MarketView
	amounts    []int
	gift       bool
	giftChoice bool
	pending    bool
	serial     uint64
	rejection  string
}

func (d *marketDraft) bounds(i int) (int, int) {
	r := d.rows()[i]
	lo, hi := 0, 0
	if d.gift || r.WillSell {
		lo = -r.CargoQty
	}
	if !d.gift && r.WillBuy {
		hi = r.StockQty
	}
	return lo, hi
}

func (d *marketDraft) rows() []tabview.MarketRow {
	if d.gift && d.view.GiftRows != nil {
		return d.view.GiftRows
	}
	return d.view.Rows
}
func (d *marketDraft) step(i, n int) {
	d.rejection = ""
	lo, hi := d.bounds(i)
	d.amounts[i] = max(lo, min(hi, d.amounts[i]+n))
}

func (d *marketDraft) giftMode(gift bool) {
	d.giftChoice = gift
	if gift != d.gift && d.view.GiftRows != nil {
		old := map[string]int{}
		for i, r := range d.rows() {
			old[r.Kind] = d.amounts[i]
		}
		d.gift = gift
		d.amounts = make([]int, len(d.rows()))
		for i, r := range d.rows() {
			lo, hi := d.bounds(i)
			d.amounts[i] = max(lo, min(hi, old[r.Kind]))
		}
		d.rejection = ""
		return
	}
	d.gift = gift
	d.rejection = ""
	for i, n := range d.amounts {
		lo, hi := d.bounds(i)
		d.amounts[i] = max(lo, min(hi, n))
	}
}

func (d *marketDraft) offer() []trade.Delta {
	var out []trade.Delta
	for i, qty := range d.amounts {
		if qty == 0 {
			continue
		}
		r := d.rows()[i]
		g := trade.Goods{Kind: r.ItemKind, Building: r.Building}
		if d.view.Quote == 0 {
			var ok bool
			g, ok = trade.ParseGoods(item.Book{}, r.Kind)
			if !ok {
				continue
			}
		}
		out = append(out, trade.Delta{Kind: g.Kind, Building: g.Building, Qty: qty, StackID: r.StackID})
	}
	return out
}

func (d *marketDraft) silverChange() int {
	centi := 0
	for i, qty := range d.amounts {
		if qty > 0 {
			centi -= qty * d.rows()[i].BuyC
		} else if qty < 0 {
			centi -= qty * d.rows()[i].SellC
		}
	}
	return trade.RoundedSilver(centi)
}

func (d *marketDraft) reason() string {
	if d.pending {
		return "Trade is awaiting confirmation."
	}
	if d.rejection != "" {
		return d.rejection
	}
	if d.view.Negotiator == 0 {
		return "No caravan member can negotiate this trade."
	}
	if len(d.offer()) == 0 {
		return "Choose goods to trade."
	}
	if d.gift {
		return ""
	}
	change := d.silverChange()
	if change < -d.view.CaravanSilver {
		return "The caravan lacks silver."
	}
	if change > d.view.TraderSilver {
		return "The settlement lacks silver."
	}
	return ""
}

const staleMarketQuote = "The trade quote changed. Review the current offer."

// live reads the same cached snapshot as the inspector. The command
// owner still validates the authoritative quote immediately before trade.
func (v marketDialog) live(d *marketDraft) (tabview.MarketView, tabview.MarketReply, bool) {
	for _, m := range v.markets {
		if m.CaravanID == d.view.CaravanID {
			return m, v.marketReply, true
		}
	}
	return tabview.MarketView{}, v.marketReply, false
}

// refreshNow uses the existing world read boundary for explicit user
// actions. Ordinary modal layout/disabled callbacks never force a read.
func (v marketDialog) refreshNow() {
	env := v.sh.Env()
	if env.Game != nil && env.Game == v.game && v.View.open {
		if p := v.planetFor(env); p != nil {
			v.read.dirty = true
			v.refresh(env, p)
		}
	}
}

func (v marketDialog) reason(d *marketDraft) string {
	if d.pending {
		return d.reason()
	}
	if d.view.Quote != 0 {
		m, _, ok := v.live(d)
		if !ok || m.Quote != d.view.Quote {
			return staleMarketQuote
		}
	}
	return d.reason()
}

func (v marketDialog) receive(d *marketDraft, close func()) {
	if !d.pending {
		return
	}
	_, r, _ := v.live(d)
	if r.Serial <= d.serial || r.CaravanID != d.view.CaravanID || r.Quote != d.view.Quote {
		return
	}
	d.pending = false
	if r.Reason == "" {
		close()
		return
	}
	d.rejection = r.Reason
}

func (v marketDialog) open(m tabview.MarketView) {
	// Legacy view fixtures do not carry separate gift rows.
	if m.GiftRows == nil && m.CaravanSilver > 0 {
		m.Rows = append(append([]tabview.MarketRow(nil), m.Rows...), tabview.MarketRow{Kind: item.Silver.String(), ItemKind: item.Silver, Label: item.Silver.String(), Category: item.Silver.Category(), CargoQty: m.CaravanSilver, SellC: 100})
	}
	d := &marketDraft{view: m, amounts: make([]int, len(m.Rows))}
	resume := v.sh.SpeedTier()
	if resume != 0 {
		v.sh.SetSpeed(0)
	}
	restore := func() {
		if resume != 0 && v.sh.SpeedTier() == 0 {
			v.sh.SetSpeed(resume)
		}
	}
	close := func() { v.kit().CloseModal(marketModalID); restore() }
	v.kit().Push(uikit.Modal{
		ID: marketModalID, Title: v.sh.T("Settlement trade"), W: 810, H: 570, OnClose: restore,
		Draw: func(body gfx.Rect) {
			if v.sh.SpeedTier() != 0 {
				v.sh.SetSpeed(0)
			}
			v.receive(d, close)
			if v.kit().Top() == marketModalID {
				v.draw(d, body)
			}
		},
		Buttons: []uikit.ModalButton{
			{ID: "accept", Label: v.sh.T("Accept"), Style: uikit.BtnPrimary, Disabled: func() string { return marketReasonText(v.sh, v.reason(d)) },
				OnClick: func() {
					v.refreshNow()
					if v.reason(d) != "" {
						return
					}
					_, r, _ := v.live(d)
					d.serial = r.Serial
					d.pending = true
					v.send(command.Command{Kind: command.WorldCaravanTrade, Target: d.view.CaravanID, Pawn: d.view.Negotiator, Name: trade.EncodeOffer(d.offer()), Enabled: d.gift, Quote: d.view.Quote})
					v.refreshNow()
					v.receive(d, close)
				}},
			{ID: "refresh", Label: v.sh.T("Refresh"), Disabled: func() string {
				if d.pending {
					return v.sh.T("Trade is awaiting confirmation.")
				}
				return ""
			},
				OnClick: func() {
					v.refreshNow()
					if m, _, ok := v.live(d); ok {
						old := map[string]int{}
						for i, r := range d.rows() {
							old[r.Kind] = d.amounts[i]
						}
						d.view = m
						d.rejection = ""
						d.amounts = make([]int, len(d.rows()))
						for i, r := range d.rows() {
							lo, hi := d.bounds(i)
							d.amounts[i] = max(lo, min(hi, old[r.Kind]))
						}
					}
				}},
			{ID: "cancel", Label: v.sh.T("Cancel"), OnClick: close},
		},
	})
}

func (v marketDialog) draw(d *marketDraft, body gfx.Rect) {
	// Widget callbacks can change the bound value between Draw calls.
	// Consume the value before laying out any row, even if the frame
	// event was cleared by the next input frame.
	if d.giftChoice != d.gift && !d.pending {
		d.giftMode(d.giftChoice)
	}
	k := v.kit()
	head := gfx.WithColor(gfx.Small, uikit.TextDim)
	k.Label(body.X, body.Y, v.sh.Tf("Caravan silver: %d    Settlement silver: %d",
		d.view.CaravanSilver, d.view.TraderSilver), gfx.Body)
	k.Label(body.X, body.Y+30, v.sh.T("Cargo"), head)
	k.Label(body.X+59, body.Y+30, v.sh.T("Base"), head)
	k.Label(body.X+128, body.Y+30, v.sh.T("Amount"), head)
	k.Label(body.X+220, body.Y+30, v.sh.T("Give"), head)
	k.Label(body.X+365, body.Y+30, v.sh.T("Take"), head)
	k.Label(body.X+510, body.Y+30, v.sh.T("Goods"), head)

	const rowH = 29
	table := gfx.Rect{X: body.X, Y: body.Y + 51, W: body.W, H: body.H - 125}
	area := k.ScrollArea("world.market.rows", table, float32(len(d.rows())*rowH),
		uikit.ScrollOpts{Step: rowH * 2})
	y := area.View.Y - area.Off
	for i, r := range d.rows() {
		ry := y + float32(i)*rowH
		if ry < area.View.Y || ry+rowH > area.View.Bottom() {
			continue
		}
		k.Label(area.View.X, ry+6, fmt.Sprintf("%d", r.CargoQty), gfx.Body)
		k.Label(area.View.X+59, ry+6, fmt.Sprintf("%d", r.StockQty), gfx.Body)
		k.Label(area.View.X+128, ry+6, fmt.Sprintf("%+d", d.amounts[i]), gfx.Body)
		lo, hi := d.bounds(i)
		if d.gift {
			k.Label(area.View.X+290, ry+6, fmt.Sprintf("%.2f", float64(r.GiftC)/100), gfx.Small)
		} else {
			if r.WillSell {
				k.Label(area.View.X+290, ry+6, fmt.Sprintf("%.2f", float64(r.SellC)/100), gfx.Small)
			}
			if r.WillBuy {
				k.Label(area.View.X+435, ry+6, fmt.Sprintf("%.2f", float64(r.BuyC)/100), gfx.Small)
			}
		}
		if k.Button(fmt.Sprintf("world.market.sell.%d", i),
			gfx.Rect{X: area.View.X + 218, Y: ry + 2, W: 67, H: rowH - 4},
			"-1", uikit.BtnOpts{Disabled: d.pending || d.amounts[i] <= lo}) {
			d.step(i, -1)
		}
		if k.Button(fmt.Sprintf("world.market.buy.%d", i),
			gfx.Rect{X: area.View.X + 362, Y: ry + 2, W: 67, H: rowH - 4},
			"+1", uikit.BtnOpts{Disabled: d.pending || d.amounts[i] >= hi}) {
			d.step(i, 1)
		}
		k.Label(area.View.X+510, ry+6, r.Label, gfx.Body)
	}
	if k.Check("world.market.gift", gfx.Rect{X: body.X, Y: body.Bottom() - 65, W: 250, H: 22},
		v.sh.T("Gift (no payment)"), &d.giftChoice, uikit.CheckOpts{Disabled: d.pending}) {
		d.giftMode(d.giftChoice)
	}
	if !d.gift {
		k.Label(body.X, body.Bottom()-35, v.sh.Tf("Silver change: %+d", d.silverChange()), gfx.Body)
	} else {
		centi := 0
		for i, q := range d.amounts {
			centi -= q * d.rows()[i].GiftC
		}
		k.Label(body.X, body.Bottom()-35, v.sh.Tf("Gift value: %s", fmt.Sprintf("%.2f", float64(centi)/100)), gfx.Body)
	}
	if why := v.reason(d); why != "" {
		k.Label(body.X+310, body.Bottom()-36, marketReasonText(v.sh, why),
			gfx.WithColor(gfx.Body, uikit.Danger))
	}
}

// marketReasonText translates all native market refusals, including the
// command receipt, through the normal literal catalogue extractor.
func marketReasonText(sh Shell, reason string) string {
	switch reason {
	case "Choose goods to trade.":
		return sh.T("Choose goods to trade.")
	case "No caravan member can negotiate this trade.":
		return sh.T("No caravan member can negotiate this trade.")
	case "Silver is settled by the trader.":
		return sh.T("Silver is settled by the trader.")
	case "The caravan cannot carry the purchased goods.":
		return sh.T("The caravan cannot carry the purchased goods.")
	case "The caravan cannot make that trade.":
		return sh.T("The caravan cannot make that trade.")
	case "The caravan is not visiting a settlement.":
		return sh.T("The caravan is not visiting a settlement.")
	case "The caravan lacks goods for that trade.":
		return sh.T("The caravan lacks goods for that trade.")
	case "The caravan lacks silver for that trade.":
		return sh.T("The caravan lacks silver for that trade.")
	case "The caravan lacks silver.":
		return sh.T("The caravan lacks silver.")
	case "The settlement cannot trade.":
		return sh.T("The settlement cannot trade.")
	case "The settlement cargo is inconsistent.":
		return sh.T("The settlement cargo is inconsistent.")
	case "The settlement has no stock.":
		return sh.T("The settlement has no stock.")
	case "The settlement lacks silver.":
		return sh.T("The settlement lacks silver.")
	case "The settlement lacks the offered goods.":
		return sh.T("The settlement lacks the offered goods.")
	case "The settlement market is unavailable.":
		return sh.T("The settlement market is unavailable.")
	case "The trade contains an invalid item stack.":
		return sh.T("The trade contains an invalid item stack.")
	case "The trade offer is invalid.":
		return sh.T("The trade offer is invalid.")
	case "The trade quote changed. Review the current offer.":
		return sh.T("The trade quote changed. Review the current offer.")
	case "Trade is awaiting confirmation.":
		return sh.T("Trade is awaiting confirmation.")
	case "impossible quantity":
		return sh.T("impossible quantity")
	case "not enough goods":
		return sh.T("not enough goods")
	case "not enough silver":
		return sh.T("not enough silver")
	case "nothing offered":
		return sh.T("nothing offered")
	case "they don't want that":
		return sh.T("they don't want that")
	case "they won't trade with you":
		return sh.T("they won't trade with you")
	default:
		return reason
	}
}
