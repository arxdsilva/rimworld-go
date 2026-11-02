package trade

import (
	"errors"
	"strconv"
	"strings"

	"rimworld-go/internal/domain/economy/item"
)

// The offer codec ProposeTrade carries (PLAY-024; moved here from the
// game package's trade contract, AX-8).

// Offer codec errors.
var (
	errOfferSyntax = errors.New("trade offer: malformed")
	errOfferLine   = errors.New("trade offer: bad line")
)

// EncodeOffer packs an offer into command.Command.Name (Command
// stays comparable): "kind:qty" pairs joined by ",", kind the item.Kind
// id and qty signed (positive: the colony buys), both decimal; a crated
// building's kind follows its item kind as "kind/building" (AX-1). The
// empty offer is "".
func EncodeOffer(offer []Delta) string {
	var b strings.Builder
	for i, d := range offer {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(int(d.Kind)))
		if d.Building != 0 {
			b.WriteByte('/')
			b.WriteString(strconv.Itoa(int(d.Building)))
		}
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(d.Qty))
	}
	return b.String()
}

// DecodeOffer reads EncodeOffer's output. It refuses malformed
// text, unknown item kinds, zero quantities and a kind listed twice.
func DecodeOffer(s string) ([]Delta, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]Delta, 0, len(parts))
	seen := make(map[Goods]bool, len(parts))
	for _, p := range parts {
		ks, qs, ok := strings.Cut(p, ":")
		if !ok {
			return nil, errOfferSyntax
		}
		ks, bs, crated := strings.Cut(ks, "/")
		k, err := strconv.ParseUint(ks, 10, 8)
		if err != nil {
			return nil, errOfferSyntax
		}
		var b uint64
		if crated {
			if b, err = strconv.ParseUint(bs, 10, 8); err != nil || b == 0 || item.Kind(k) != item.MinifiedBuilding {
				return nil, errOfferSyntax
			}
		}
		q, err := strconv.Atoi(qs)
		if err != nil {
			return nil, errOfferSyntax
		}
		g := Goods{item.Kind(k), uint8(b)}
		if q == 0 || g.Kind.String() == "unknown" || seen[g] {
			return nil, errOfferLine
		}
		seen[g] = true
		out = append(out, Delta{Kind: g.Kind, Qty: q, Building: g.Building})
	}
	return out, nil
}
