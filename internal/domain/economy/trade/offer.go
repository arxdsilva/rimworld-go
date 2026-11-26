// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

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
		if d.StackID != 0 {
			b.WriteByte('@')
			b.WriteString(strconv.FormatUint(d.StackID, 10))
		}
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(d.Qty))
	}
	return b.String()
}

// DecodeOffer reads EncodeOffer's output. It refuses malformed
// text, item kinds outside the run's book b, zero or out-of-bound (|qty| > MaxOfferQty) quantities and a kind
// listed twice.
func DecodeOffer(b item.Book, s string) ([]Delta, error) {
	return decodeOffer(b.Len(), s)
}

// RemapOffer restores numeric offer kinds using the saved book's key map.
// Any missing key or malformed line invalidates the whole offer. Local
// exact IDs are persistent entity/lot IDs; world IDs instead hash the
// entire numeric payload, so they cannot be safely reused after remapping.
func RemapOffer(s string, savedKinds int, remap func(item.Kind) item.Kind, world bool) (string, bool) {
	offer, err := decodeOffer(savedKinds, s)
	if err != nil {
		return "", false
	}
	for i := range offer {
		d := &offer[i]
		k := remap(d.Kind)
		if k == item.None || world && d.StackID != 0 {
			return "", false
		}
		d.Kind = k
	}
	return EncodeOffer(offer), true
}

func decodeOffer(kinds int, s string) ([]Delta, error) {
	if s == "" {
		return nil, nil
	}
	// Client text is untrusted: cap parsing work before allocating parts.
	if len(s) > 65536 || strings.Count(s, ",") >= 2048 {
		return nil, errOfferSyntax
	}
	parts := strings.Split(s, ",")
	out := make([]Delta, 0, len(parts))
	seen := make(map[struct {
		Goods
		StackID uint64
	}]bool, len(parts))
	for _, p := range parts {
		ks, qs, ok := strings.Cut(p, ":")
		if !ok {
			return nil, errOfferSyntax
		}
		ks, is, selected := strings.Cut(ks, "@")
		var stackID uint64
		if selected {
			var err error
			stackID, err = strconv.ParseUint(is, 10, 64)
			if err != nil || stackID == 0 {
				return nil, errOfferSyntax
			}
		}
		ks, bs, crated := strings.Cut(ks, "/")
		k, err := strconv.ParseUint(ks, 10, 8)
		if err != nil {
			return nil, errOfferSyntax
		}
		var bld uint64
		if crated {
			if bld, err = strconv.ParseUint(bs, 10, 8); err != nil || bld == 0 || item.Kind(k) != item.MinifiedBuilding {
				return nil, errOfferSyntax
			}
		}
		q, err := strconv.Atoi(qs)
		if err != nil {
			return nil, errOfferSyntax
		}
		g := Goods{item.Kind(k), uint8(bld)}
		key := struct {
			Goods
			StackID uint64
		}{g, stackID}
		if q == 0 || q < -MaxOfferQty || q > MaxOfferQty || int(g.Kind) >= kinds || seen[key] {
			return nil, errOfferLine
		}
		seen[key] = true
		out = append(out, Delta{Kind: g.Kind, Qty: q, Building: g.Building, StackID: stackID})
	}
	return out, nil
}
