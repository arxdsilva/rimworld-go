package ebiten

import (
	"fmt"
	"slices"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/command"
)

// The trade dialog (MAIN_UI_SCREENS §28, PLAY-024): a centred modal that
// is open while the game reports a trade session. The amounts are
// presentation state; Accept sends ProposeTrade with the encoded offer
// and the game settles it. The display pauses while it is open.

const (
	tradeModalID = "colony.trade"
	tradeModalW  = 880
	tradeModalH  = 640
	tradeRowH    = 30
	tradeHeadH   = 24
)

// dealingsUI is the trade and comms dialogs' app-level state: which
// dialog is open, which the player dismissed (so the still-open session
// does not reopen it) and the speed to resume.
type dealingsUI struct {
	trade          *tradeDraft
	tradeDismissed bool
	comms          *commsDraft
	commsDismissed bool
	paused         bool
	resume         speed
}

// tradeDraft is the player's unsent deal.
type tradeDraft struct {
	view game.TradeView
	net  map[string]int // by row Kind: positive buys, negative sells
	gift bool
	sig  string // the rows' stock when the amounts were last valid
}

func newTradeDraft(v game.TradeView) *tradeDraft {
	return &tradeDraft{view: v, net: map[string]int{}, sig: tradeSig(v)}
}

// tradeSig fingerprints the stock, so an accepted deal (new quantities)
// clears the amounts.
func tradeSig(v game.TradeView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d/%d", v.ColonySilver, v.TraderSilver)
	for _, r := range v.Rows {
		fmt.Fprintf(&b, "|%s:%d:%d", r.Kind, r.ColonyQty, r.TraderQty)
	}
	return b.String()
}

// refresh takes the newest view; changed stock resets the amounts, and
// the rest are clamped to what is left.
func (d *tradeDraft) refresh(v game.TradeView) {
	d.view = v
	if s := tradeSig(v); s != d.sig {
		d.sig = s
		clear(d.net)
	}
	for _, r := range v.Rows {
		d.set(r, d.net[r.Kind])
	}
}

// bounds is the amount range of row r: negative sells (to the colony's
// stock), positive buys (to the trader's); gifts only sell.
func (d *tradeDraft) bounds(r game.TradeRowView) (lo, hi int) {
	if r.WillSell {
		lo = -r.ColonyQty
	}
	if r.WillBuy && !d.gift {
		hi = r.TraderQty
	}
	return lo, hi
}

// set stores amount n for r, clamped to its bounds.
func (d *tradeDraft) set(r game.TradeRowView, n int) {
	lo, hi := d.bounds(r)
	n = max(lo, min(hi, n))
	if n == 0 {
		delete(d.net, r.Kind)
		return
	}
	d.net[r.Kind] = n
}

// step moves row r's amount by delta (clamped).
func (d *tradeDraft) step(r game.TradeRowView, delta int) { d.set(r, d.net[r.Kind]+delta) }

// setGift switches gift mode; buying amounts are dropped.
func (d *tradeDraft) setGift(on bool) {
	d.gift = on
	if on {
		for _, r := range d.view.Rows {
			d.set(r, d.net[r.Kind])
		}
	}
}

// totalC is what the colony pays (negative: it is paid), hundredths of a
// silver, from the rows' unit prices.
func (d *tradeDraft) totalC() int {
	t := 0
	for _, r := range d.view.Rows {
		switch n := d.net[r.Kind]; {
		case n > 0:
			t += n * r.BuyC
		case n < 0:
			t += n * r.SellC
		}
	}
	return t
}

// giftSilver is the value, in whole silver, of what a gift gives.
func (d *tradeDraft) giftSilver() int {
	v := 0
	for _, r := range d.view.Rows {
		if n := d.net[r.Kind]; n < 0 {
			v += -n * r.SellC
		}
	}
	return v / 100
}

// colonyAfterC and traderAfterC are the silver on each side after the
// deal, hundredths.
func (d *tradeDraft) colonyAfterC() int { return d.view.ColonySilver*100 - d.totalC() }
func (d *tradeDraft) traderAfterC() int { return d.view.TraderSilver*100 + d.totalC() }

// offer is the deal as game.EncodeTradeOffer takes it (zero lines out).
func (d *tradeDraft) offer() []trade.Delta {
	var out []trade.Delta
	for _, r := range d.view.Rows {
		n := d.net[r.Kind]
		if n == 0 {
			continue
		}
		if k, ok := item.ParseKind(r.Kind); ok {
			out = append(out, trade.Delta{Kind: k, Qty: n})
		}
	}
	return out
}

// reason is why Accept is disabled ("" it is not).
func (d *tradeDraft) reason() string {
	switch {
	case len(d.net) == 0:
		return "Nothing on the table."
	case d.gift:
		return ""
	case d.colonyAfterC() < 0:
		return "Not enough silver."
	case d.traderAfterC() < 0:
		return "They cannot pay that much."
	}
	return ""
}

// silverText formats hundredths of a silver ("12.50").
func silverText(c int) string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	return fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
}

// tradeSourceOf is the open trade session, if the game has one.
func (a *App) tradeSourceOf() (game.TradeView, bool) {
	if a.renderer == nil {
		return game.TradeView{}, false
	}
	if s, ok := a.renderer.game.(tradeSource); ok {
		return s.TradeView()
	}
	return game.TradeView{}, false
}

// commsSourceOf is the open comms session, if the game has one.
func (a *App) commsSourceOf() (game.CommsView, bool) {
	if a.renderer == nil {
		return game.CommsView{}, false
	}
	if s, ok := a.renderer.game.(commsSource); ok {
		return s.CommsView()
	}
	return game.CommsView{}, false
}

// syncDealings opens and closes the trade and comms dialogs with the
// game's sessions (uiBegin calls it once a frame) and pauses the display
// while either is open.
func (a *App) syncDealings() {
	if a.renderer == nil {
		return
	}
	d := &a.dealings
	if v, ok := a.tradeSourceOf(); ok {
		switch {
		case d.trade != nil:
			d.trade.refresh(v)
		case !d.tradeDismissed:
			a.openTrade(v)
		}
	} else {
		if d.trade != nil {
			d.trade = nil
			a.uiCloseModal(tradeModalID)
		}
		d.tradeDismissed = false
	}
	if v, ok := a.commsSourceOf(); ok {
		switch {
		case d.comms != nil:
			d.comms.view = v
		case !d.commsDismissed:
			a.openComms(v)
		}
	} else {
		if d.comms != nil {
			d.comms = nil
			a.uiCloseModal(commsModalID)
		}
		d.commsDismissed = false
	}
	a.dealingsPause()
}

// dealingsPause pauses the display while a dialog is open and resumes the
// speed it interrupted once both are shut.
func (a *App) dealingsPause() {
	d := &a.dealings
	open := d.trade != nil || d.comms != nil
	switch {
	case open && !d.paused && a.renderer.speed != speedPaused:
		d.resume, d.paused = a.renderer.speed, true
		a.renderer.speed = speedPaused
	case !open && d.paused:
		if a.renderer.speed == speedPaused {
			a.renderer.speed = d.resume
		}
		d.paused = false
	}
}

// openTrade opens the dialog for session v.
func (a *App) openTrade(v game.TradeView) {
	d := newTradeDraft(v)
	a.dealings.trade = d
	a.uiPushModal(modal{
		id: tradeModalID, title: "Trade with " + v.Trader, w: tradeModalW, h: tradeModalH,
		draw:    func(a *App, body rect) { a.tradeBody(d, body) },
		onClose: a.dismissTrade,
		buttons: []modalButton{
			{id: "accept", label: "Accept", style: btnPrimary,
				disabled: d.reason, onClick: func() { a.proposeTrade(d) }},
			{id: "reset", label: "Reset", onClick: func() { clear(d.net) }},
			{id: "cancel", label: "Cancel", closes: true, onClick: a.dismissTrade},
		},
	})
}

// dismissTrade ends the session from the player's side.
func (a *App) dismissTrade() {
	a.dealings.trade = nil
	a.dealings.tradeDismissed = true
	a.shell.Enqueue(command.Command{Kind: command.CloseTrade})
	a.dealingsPause()
}

// proposeTrade sends the deal.
func (a *App) proposeTrade(d *tradeDraft) {
	if d.reason() != "" || a.renderer == nil {
		return
	}
	a.shell.Enqueue(command.Command{Kind: command.ProposeTrade, Name: game.EncodeTradeOffer(d.offer()), Enabled: d.gift})
}

// tradeLine is one row of the table: a category header or a goods row.
type tradeLine struct {
	head string
	row  game.TradeRowView
}

// tradeLines groups the rows by category, sorted by label.
func tradeLines(rows []game.TradeRowView) []tradeLine {
	sorted := slices.Clone(rows)
	slices.SortStableFunc(sorted, func(x, y game.TradeRowView) int {
		if c := strings.Compare(x.Category, y.Category); c != 0 {
			return c
		}
		return strings.Compare(x.Label, y.Label)
	})
	var out []tradeLine
	last := "\x00"
	for _, r := range sorted {
		if r.Category != last {
			last = r.Category
			out = append(out, tradeLine{head: r.Category})
		}
		out = append(out, tradeLine{row: r})
	}
	return out
}

var tradeSteps = []struct {
	id    string
	label string
	n     int // 0: all
}{{"a", "All", 0}, {"10", "10", 10}, {"1", "1", 1}}

// tradeBody declares the header, the table and the summary.
func (a *App) tradeBody(d *tradeDraft, body rect) {
	v := d.view
	hs := withColor(tsLabel, uiHeading)
	kind := v.TraderKind
	if v.Orbital {
		kind += " (orbital ship)"
	}
	a.uiLabel(body.X, body.Y, fmt.Sprintf("%s · %s · %s · goodwill %d", v.Trader, kind, v.Faction, v.Goodwill), hs)
	a.uiLabel(body.X, body.Y+22, fmt.Sprintf("%s negotiates (Social %d) · Price modifier %+d%% · leaves in %d h",
		v.NegotiatorName, v.Social, v.PriceModPct, v.HoursLeft), dimText())

	top := body.Y + 50
	sum := rect{body.X, body.bottom() - 84, body.W, 84}
	table := rect{body.X, top + tradeHeadH, body.W, sum.Y - top - tradeHeadH - 6}

	// column geometry, shared by the headers and the rows
	const sw, aw, qw, pw = 40.0, 64.0, 46.0, 62.0
	x := body.X
	colPrice := rect{x, 0, pw, 0}
	x += pw
	colQty := rect{x, 0, qw, 0}
	x += qw + 4
	colSell := rect{x, 0, 3 * (sw + 2), 0}
	x += colSell.W + 4
	colAmt := rect{x, 0, aw, 0}
	x += aw + 4
	colBuy := rect{x, 0, 3 * (sw + 2), 0}
	x += colBuy.W + 4
	colTQty := rect{x, 0, qw, 0}
	x += qw
	colTPrice := rect{x, 0, pw, 0}
	x += pw + 10
	colName := rect{x, 0, body.right() - x - scrollBarW, 0}

	hd := withColor(tsSmall, uiTextDim)
	a.uiLabel(colPrice.X, top+4, "Sell for", hd)
	a.uiLabel(colQty.X+4, top+4, "Colony", hd)
	a.uiLabel(colSell.X, top+4, "Give to trader", hd)
	a.uiLabel(colAmt.X+8, top+4, "Amount", hd)
	a.uiLabel(colBuy.X, top+4, "Take from trader", hd)
	a.uiLabel(colTQty.X+2, top+4, "Trader", hd)
	a.uiLabel(colTPrice.X+4, top+4, "Buy for", hd)
	a.uiLabel(colName.X, top+4, "Goods", hd)

	lines := tradeLines(v.Rows)
	if len(lines) == 0 {
		a.uiLabel(table.X, table.Y+6, "Neither side has anything to trade.", dimText())
	}
	area := a.uiScrollArea("trade.rows", table, float32(len(lines)*tradeRowH), scrollOpts{Step: tradeRowH * 2})
	y := area.View.Y - area.Off
	for _, ln := range lines {
		ry := y
		y += tradeRowH
		if ry < area.View.Y || ry+tradeRowH > area.View.bottom() {
			continue
		}
		if ln.head != "" {
			a.uiLabel(area.View.X, ry+6, ln.head, withColor(tsLabel, uiHeading))
			continue
		}
		a.tradeRow(d, ln.row, ry, colPrice, colQty, colSell, colAmt, colBuy, colTQty, colTPrice, colName)
	}

	// summary
	total := d.totalC()
	verb := "You pay"
	if total < 0 {
		verb = "You receive"
	}
	a.uiLabel(sum.X, sum.Y, fmt.Sprintf("%s %s silver", verb, silverText(abs(total))), tsLabel)
	if a.uiCheck("trade.gift", rect{sum.X, sum.Y + 26, 220, 22}, "Gift (no payment)", &d.gift,
		checkOpts{Tip: "Gift\nGive goods away for goodwill instead of trading."}) {
		d.setGift(d.gift)
	}
	if d.gift {
		a.uiLabel(sum.X+240, sum.Y+30, fmt.Sprintf("Goodwill %d -> %d", v.Goodwill, v.Goodwill+faction.GiftGoodwill(d.giftSilver())), tsBody)
	} else {
		after := withColor(tsBody, uiText)
		if d.colonyAfterC() < 0 {
			after = withColor(tsBody, uiDanger)
		}
		a.uiLabel(sum.X+240, sum.Y+30, "Colony silver after: "+silverText(d.colonyAfterC()), after)
		a.uiLabel(sum.X+500, sum.Y+30, "Trader silver after: "+silverText(d.traderAfterC()), dimText())
	}
	if v.Reason != "" {
		a.uiLabel(sum.X, sum.Y+58, "Refused: "+v.Reason, withColor(tsBody, uiDanger))
	}
}

// tradeRow declares one goods row.
func (a *App) tradeRow(d *tradeDraft, r game.TradeRowView, y float32,
	cPrice, cQty, cSell, cAmt, cBuy, cTQty, cTPrice, cName rect) {
	n := d.net[r.Kind]
	mid := y + 6
	a.uiLabel(cName.X, mid, fitText(r.Label, tsBody, cName.W), tsBody)
	if r.WillSell {
		a.uiLabel(cPrice.X, mid, silverText(r.SellC), tsBody)
	} else {
		a.uiLabel(cPrice.X, mid, "-", dimText())
	}
	if r.WillBuy {
		a.uiLabel(cTPrice.X+4, mid, silverText(r.BuyC), tsBody)
	} else {
		a.uiLabel(cTPrice.X+4, mid, "-", dimText())
	}
	a.uiLabel(cQty.X+4, mid, uiNum(r.ColonyQty), tsBody)
	a.uiLabel(cTQty.X+2, mid, uiNum(r.TraderQty), tsBody)
	amt := withColor(tsBody, uiTextDim)
	switch {
	case n > 0:
		amt = withColor(tsLabel, uiWarning)
	case n < 0:
		amt = withColor(tsLabel, uiGood)
	}
	label := "0"
	if n != 0 {
		label = fmt.Sprintf("%+d", n)
	}
	a.uiLabel(cAmt.X+8, mid, label, amt)

	lo, hi := d.bounds(r)
	btn := rect{0, y + 2, 40, tradeRowH - 4}
	for i, s := range tradeSteps { // give: All, 10, 1 (left to right)
		btn.X = cSell.X + float32(i)*42
		why := ""
		switch {
		case !r.WillSell:
			why = "They don't buy that."
		case r.ColonyQty == 0:
			why = "You have none."
		case n <= lo:
			why = "That is everything you have."
		}
		step := s.n
		if step == 0 {
			step = r.ColonyQty
		}
		if a.uiButton("trade."+r.Kind+".give."+s.id, btn, s.label, btnOpts{Disabled: why != "", Reason: why}) {
			d.step(r, -step)
		}
	}
	for i := len(tradeSteps) - 1; i >= 0; i-- { // take: 1, 10, All
		s := tradeSteps[i]
		btn.X = cBuy.X + float32(len(tradeSteps)-1-i)*42
		why := ""
		switch {
		case d.gift && r.WillBuy:
			why = "A gift only gives goods away."
		case !r.WillBuy:
			why = "They have none to sell."
		case n >= hi:
			why = "That is all they have."
		}
		step := s.n
		if step == 0 {
			step = r.TraderQty
		}
		if a.uiButton("trade."+r.Kind+".take."+s.id, btn, s.label, btnOpts{Disabled: why != "", Reason: why}) {
			d.step(r, step)
		}
	}
}
