package game

import (
	"errors"
	"strconv"
	"strings"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-024 Phase 0 contract (lead-owned): the read models the trade and
// comms dialogs consume and the offer codec ProposeTrade carries. The
// producers are trade_session.go and comms.go (agent S2, Phase 1); the
// dialogs (agent U1, Phase 2) read them through their own interfaces.
// See .cursor/MAIN_UI_SCREENS_SPECIFICATION.md §28 and §29.

// TradeView is the open trade session (Game.TradeView; ok false while
// none is open). Prices are per unit in hundredths of a silver.
type TradeView struct {
	// Trader is the trader's name, TraderKind its kind's label ("bulk
	// hauler", "quartermaster", "curio merchant"), Faction its faction's
	// name (FactionID its id). Orbital: a ship over the radio; HoursLeft
	// until it (or the caravan) leaves.
	Trader, TraderKind, Faction string
	FactionID                   int
	Orbital                     bool
	HoursLeft                   int
	// Negotiator is the colonist trading, with its Social level and the
	// price modifier it earns (PriceModPct: percent off the base markup,
	// shown as "Price modifier").
	Negotiator     entity.EntityID
	NegotiatorName string
	Social         int
	PriceModPct    int
	// Goodwill is the faction's goodwill now; ColonySilver the silver the
	// colony can trade (for orbital trade only what is in beacon reach);
	// TraderSilver what the trader has.
	Goodwill     int
	ColonySilver int
	TraderSilver int
	// Rows are the goods either side has, by category then label; silver
	// is never a row.
	Rows []TradeRowView
	// Reason is why the last ProposeTrade was refused ("" when none was).
	Reason string
}

// TradeRowView is one row of the trade dialog. Kind is the item kind's
// name (item.Kind.String; the key EncodeTradeOffer's caller parses back),
// Label its display name, Category its item category. BuyC is what the
// colony pays for one unit, SellC what it gets for one (hundredths of a
// silver). WillBuy: the trader sells it (it has some); WillSell: the
// trader takes it from the colony.
type TradeRowView struct {
	Kind, Label, Category string
	ColonyQty, TraderQty  int
	BuyC, SellC           int
	WillBuy, WillSell     bool
}

// CommsView is the open comms session at a radio console (Game.CommsView;
// ok false while none is open).
type CommsView struct {
	// Operator is the colonist at Console (the radio console building).
	Operator     entity.EntityID
	OperatorName string
	Console      entity.EntityID
	// Factions lists every known faction except the player, by id.
	Factions []CommsFactionView
	// Orbital is the ship in radio range (Present false: none).
	Orbital OrbitalView
	// BeaconSilver is the silver within reach of a trade beacon (what a
	// gift or an orbital trade can spend).
	BeaconSilver int
}

// CommsFactionView is one faction row: its goodwill and relation
// ("hostile", "neutral", "friendly") and each action it allows now.
type CommsFactionView struct {
	ID       int
	Name     string
	Type     string // faction.Type key ("settlers", ...)
	Goodwill int
	Relation string
	Gift     CommsActionView
	Caravan  CommsActionView
	Aid      CommsActionView
}

// CommsActionView is one comms action: Allowed now, its Cost (goodwill
// for a request; for a gift the silver per goodwill point), and Reason
// when not allowed ("cooling down: 2 days", "needs goodwill 0", ...).
type CommsActionView struct {
	Allowed bool
	Cost    int
	Reason  string
}

// OrbitalView is the orbital trader in radio range: its ship name and
// trader kind label, the hours before it leaves, and whether it can be
// called now (CallReason: "needs a trade beacon", ...).
type OrbitalView struct {
	Present    bool
	Ship, Kind string
	HoursLeft  int
	CallOK     bool
	CallReason string
	// CaravanVisiting: a trader caravan is on the map (filled by
	// Game.TradeStatus only), from CaravanFaction, leaving in
	// CaravanHoursLeft hours.
	CaravanVisiting  bool
	CaravanFaction   string
	CaravanHoursLeft int
}

// Offer codec errors.
var (
	errOfferSyntax = errors.New("trade offer: malformed")
	errOfferLine   = errors.New("trade offer: bad line")
)

// EncodeTradeOffer packs an offer into command.Command.Name (Command
// stays comparable): "kind:qty" pairs joined by ",", kind the item.Kind
// id and qty signed (positive: the colony buys), both decimal. The empty
// offer is "".
func EncodeTradeOffer(offer []trade.Delta) string {
	var b strings.Builder
	for i, d := range offer {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(int(d.Kind)))
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(d.Qty))
	}
	return b.String()
}

// DecodeTradeOffer reads EncodeTradeOffer's output. It refuses malformed
// text, unknown item kinds, zero quantities and a kind listed twice.
func DecodeTradeOffer(s string) ([]trade.Delta, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]trade.Delta, 0, len(parts))
	seen := make(map[item.Kind]bool, len(parts))
	for _, p := range parts {
		ks, qs, ok := strings.Cut(p, ":")
		if !ok {
			return nil, errOfferSyntax
		}
		k, err := strconv.ParseUint(ks, 10, 8)
		if err != nil {
			return nil, errOfferSyntax
		}
		q, err := strconv.Atoi(qs)
		if err != nil {
			return nil, errOfferSyntax
		}
		kind := item.Kind(k)
		if q == 0 || kind.String() == "unknown" || seen[kind] {
			return nil, errOfferLine
		}
		seen[kind] = true
		out = append(out, trade.Delta{Kind: kind, Qty: q})
	}
	return out, nil
}
