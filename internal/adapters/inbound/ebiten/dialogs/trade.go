package dialogs

import (
	"fmt"
	"slices"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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
	TradeModalID = "colony.trade"
	tradeModalW  = 880
	tradeModalH  = 640
	tradeRowH    = 30
	tradeHeadH   = 24
)

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

// SilverText formats hundredths of a silver ("12.50").
func SilverText(c int) string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	return fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
}

// SyncDealings opens and closes the trade and comms dialogs with the
// game's sessions (the shell calls it once a frame) and pauses the display
// while either is open.
func (dl *Dealings) SyncDealings() {
	if v, ok := dl.sh.Trade(); ok {
		switch {
		case dl.trade != nil:
			dl.trade.refresh(v)
		case !dl.tradeDismissed:
			dl.openTrade(v)
		}
	} else {
		if dl.trade != nil {
			dl.trade = nil
			dl.kit().CloseModal(TradeModalID)
		}
		dl.tradeDismissed = false
	}
	if v, ok := dl.sh.Comms(); ok {
		switch {
		case dl.comms != nil:
			dl.comms.view = v
		case !dl.commsDismissed:
			dl.openComms(v)
		}
	} else {
		if dl.comms != nil {
			dl.comms = nil
			dl.kit().CloseModal(CommsModalID)
		}
		dl.commsDismissed = false
	}
	dl.pause()
}

// Open reports whether a trade or comms dialog is open.
func (dl *Dealings) Open() bool { return dl.trade != nil || dl.comms != nil }

// Paused reports whether the dialogs hold the display paused.
func (dl *Dealings) Paused() bool { return dl.paused }

// HoldPause records that the display was paused over an open dialog while
// it ran at speed tier resume, so closing the last dialog restores it.
func (dl *Dealings) HoldPause(resume int) { dl.resume, dl.paused = resume, true }

// pause pauses the display while a dialog is open and resumes the speed
// it interrupted once both are shut.
func (dl *Dealings) pause() {
	open := dl.Open()
	switch tier := dl.sh.SpeedTier(); {
	case open && !dl.paused && tier != speedPausedTier:
		dl.resume, dl.paused = tier, true
		dl.sh.HoldSpeed(speedPausedTier)
	case !open && dl.paused:
		if tier == speedPausedTier {
			dl.sh.HoldSpeed(dl.resume)
		}
		dl.paused = false
	}
}

// openTrade opens the dialog for session v.
func (dl *Dealings) openTrade(v game.TradeView) {
	d := newTradeDraft(v)
	dl.trade = d
	dl.kit().Push(uikit.Modal{
		ID: TradeModalID, Title: "Trade with " + v.Trader, W: tradeModalW, H: tradeModalH,
		Draw:    func(body gfx.Rect) { dl.tradeBody(d, body) },
		OnClose: dl.dismissTrade,
		Buttons: []uikit.ModalButton{
			{ID: "accept", Label: "Accept", Style: uikit.BtnPrimary,
				Disabled: d.reason, OnClick: func() { dl.proposeTrade(d) }},
			{ID: "reset", Label: "Reset", OnClick: func() { clear(d.net) }},
			{ID: "cancel", Label: "Cancel", Closes: true, OnClick: dl.dismissTrade},
		},
	})
}

// dismissTrade ends the session from the player's side.
func (dl *Dealings) dismissTrade() {
	dl.trade = nil
	dl.tradeDismissed = true
	dl.sh.Enqueue(command.Command{Kind: command.CloseTrade})
	dl.pause()
}

// proposeTrade sends the deal.
func (dl *Dealings) proposeTrade(d *tradeDraft) {
	if d.reason() != "" {
		return
	}
	dl.sh.Enqueue(command.Command{Kind: command.ProposeTrade, Name: game.EncodeTradeOffer(d.offer()), Enabled: d.gift})
}

// TradeLine is one row of the table: a category header or a goods row.
type TradeLine struct {
	Head string
	Row  game.TradeRowView
}

// TradeLines groups the rows by category, sorted by label.
func TradeLines(rows []game.TradeRowView) []TradeLine {
	sorted := slices.Clone(rows)
	slices.SortStableFunc(sorted, func(x, y game.TradeRowView) int {
		if c := strings.Compare(x.Category, y.Category); c != 0 {
			return c
		}
		return strings.Compare(x.Label, y.Label)
	})
	var out []TradeLine
	last := "\x00"
	for _, r := range sorted {
		if r.Category != last {
			last = r.Category
			out = append(out, TradeLine{Head: r.Category})
		}
		out = append(out, TradeLine{Row: r})
	}
	return out
}

var tradeSteps = []struct {
	id    string
	label string
	n     int // 0: all
}{{"a", "All", 0}, {"10", "10", 10}, {"1", "1", 1}}

// tradeBody declares the header, the table and the summary.
func (dl *Dealings) tradeBody(d *tradeDraft, body gfx.Rect) {
	v := d.view
	hs := gfx.WithColor(gfx.Label, uikit.Heading)
	kind := v.TraderKind
	if v.Orbital {
		kind += " (orbital ship)"
	}
	dl.kit().Label(body.X, body.Y, fmt.Sprintf("%s · %s · %s · goodwill %d", v.Trader, kind, v.Faction, v.Goodwill), hs)
	dl.kit().Label(body.X, body.Y+22, fmt.Sprintf("%s negotiates (Social %d) · Price modifier %+d%% · leaves in %d h",
		v.NegotiatorName, v.Social, v.PriceModPct, v.HoursLeft), uikit.DimText())

	top := body.Y + 50
	sum := gfx.Rect{X: body.X, Y: body.Bottom() - 84, W: body.W, H: 84}
	table := gfx.Rect{X: body.X, Y: top + tradeHeadH, W: body.W, H: sum.Y - top - tradeHeadH - 6}

	// column geometry, shared by the headers and the rows
	const sw, aw, qw, pw = 40.0, 64.0, 46.0, 62.0
	x := body.X
	colPrice := gfx.Rect{X: x, Y: 0, W: pw, H: 0}
	x += pw
	colQty := gfx.Rect{X: x, Y: 0, W: qw, H: 0}
	x += qw + 4
	colSell := gfx.Rect{X: x, Y: 0, W: 3 * (sw + 2), H: 0}
	x += colSell.W + 4
	colAmt := gfx.Rect{X: x, Y: 0, W: aw, H: 0}
	x += aw + 4
	colBuy := gfx.Rect{X: x, Y: 0, W: 3 * (sw + 2), H: 0}
	x += colBuy.W + 4
	colTQty := gfx.Rect{X: x, Y: 0, W: qw, H: 0}
	x += qw
	colTPrice := gfx.Rect{X: x, Y: 0, W: pw, H: 0}
	x += pw + 10
	colName := gfx.Rect{X: x, Y: 0, W: body.Right() - x - uikit.ScrollBarW, H: 0}

	hd := gfx.WithColor(gfx.Small, uikit.TextDim)
	dl.kit().Label(colPrice.X, top+4, "Sell for", hd)
	dl.kit().Label(colQty.X+4, top+4, "Colony", hd)
	dl.kit().Label(colSell.X, top+4, "Give to trader", hd)
	dl.kit().Label(colAmt.X+8, top+4, "Amount", hd)
	dl.kit().Label(colBuy.X, top+4, "Take from trader", hd)
	dl.kit().Label(colTQty.X+2, top+4, "Trader", hd)
	dl.kit().Label(colTPrice.X+4, top+4, "Buy for", hd)
	dl.kit().Label(colName.X, top+4, "Goods", hd)

	lines := TradeLines(v.Rows)
	if len(lines) == 0 {
		dl.kit().Label(table.X, table.Y+6, "Neither side has anything to trade.", uikit.DimText())
	}
	area := dl.kit().ScrollArea("trade.rows", table, float32(len(lines)*tradeRowH), uikit.ScrollOpts{Step: tradeRowH * 2})
	y := area.View.Y - area.Off
	for _, ln := range lines {
		ry := y
		y += tradeRowH
		if ry < area.View.Y || ry+tradeRowH > area.View.Bottom() {
			continue
		}
		if ln.Head != "" {
			dl.kit().Label(area.View.X, ry+6, ln.Head, gfx.WithColor(gfx.Label, uikit.Heading))
			continue
		}
		dl.tradeRow(d, ln.Row, ry, colPrice, colQty, colSell, colAmt, colBuy, colTQty, colTPrice, colName)
	}

	// summary
	total := d.totalC()
	verb := "You pay"
	if total < 0 {
		verb = "You receive"
	}
	dl.kit().Label(sum.X, sum.Y, fmt.Sprintf("%s %s silver", verb, SilverText(absInt(total))), gfx.Label)
	if dl.kit().Check("trade.gift", gfx.Rect{X: sum.X, Y: sum.Y + 26, W: 220, H: 22}, "Gift (no payment)", &d.gift,
		uikit.CheckOpts{Tip: "Gift\nGive goods away for goodwill instead of trading."}) {
		d.setGift(d.gift)
	}
	if d.gift {
		dl.kit().Label(sum.X+240, sum.Y+30, fmt.Sprintf("Goodwill %d -> %d", v.Goodwill, v.Goodwill+faction.GiftGoodwill(d.giftSilver())), gfx.Body)
	} else {
		after := gfx.WithColor(gfx.Body, uikit.Text)
		if d.colonyAfterC() < 0 {
			after = gfx.WithColor(gfx.Body, uikit.Danger)
		}
		dl.kit().Label(sum.X+240, sum.Y+30, "Colony silver after: "+SilverText(d.colonyAfterC()), after)
		dl.kit().Label(sum.X+500, sum.Y+30, "Trader silver after: "+SilverText(d.traderAfterC()), uikit.DimText())
	}
	if v.Reason != "" {
		dl.kit().Label(sum.X, sum.Y+58, "Refused: "+v.Reason, gfx.WithColor(gfx.Body, uikit.Danger))
	}
}

// tradeRow declares one goods row.
func (dl *Dealings) tradeRow(d *tradeDraft, r game.TradeRowView, y float32,
	cPrice, cQty, cSell, cAmt, cBuy, cTQty, cTPrice, cName gfx.Rect) {
	n := d.net[r.Kind]
	mid := y + 6
	dl.kit().Label(cName.X, mid, uikit.FitText(r.Label, gfx.Body, cName.W), gfx.Body)
	if r.WillSell {
		dl.kit().Label(cPrice.X, mid, SilverText(r.SellC), gfx.Body)
	} else {
		dl.kit().Label(cPrice.X, mid, "-", uikit.DimText())
	}
	if r.WillBuy {
		dl.kit().Label(cTPrice.X+4, mid, SilverText(r.BuyC), gfx.Body)
	} else {
		dl.kit().Label(cTPrice.X+4, mid, "-", uikit.DimText())
	}
	dl.kit().Label(cQty.X+4, mid, uikit.Num(r.ColonyQty), gfx.Body)
	dl.kit().Label(cTQty.X+2, mid, uikit.Num(r.TraderQty), gfx.Body)
	amt := gfx.WithColor(gfx.Body, uikit.TextDim)
	switch {
	case n > 0:
		amt = gfx.WithColor(gfx.Label, uikit.Warning)
	case n < 0:
		amt = gfx.WithColor(gfx.Label, uikit.Good)
	}
	label := "0"
	if n != 0 {
		label = fmt.Sprintf("%+d", n)
	}
	dl.kit().Label(cAmt.X+8, mid, label, amt)

	lo, hi := d.bounds(r)
	btn := gfx.Rect{X: 0, Y: y + 2, W: 40, H: tradeRowH - 4}
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
		if dl.kit().Button("trade."+r.Kind+".give."+s.id, btn, s.label, uikit.BtnOpts{Disabled: why != "", Reason: why}) {
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
		if dl.kit().Button("trade."+r.Kind+".take."+s.id, btn, s.label, uikit.BtnOpts{Disabled: why != "", Reason: why}) {
			d.step(r, step)
		}
	}
}
