// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package tradeview is the PLAY-024 contract's read models: what the trade
// and comms dialogs consume (ProposeTrade's offer codec is
// trade.EncodeOffer). The game fills them (trade_session.go, comms.go) and
// names them game.TradeView, game.CommsView and so on; the dialogs read
// them through their own interfaces (MAIN_UI_SCREENS_SPECIFICATION.md §28
// and §29). Plain data: no state, no host.
package tradeview

import (
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
)

// Trade is the open trade session (game.Game.TradeView; ok false while
// none is open). Prices are per unit in hundredths of a silver.
type Trade struct {
	// Trader is the trader's name, TraderKind its kind's label ("bulk
	// hauler", "quartermaster", "curio merchant"), Faction its faction's
	// name (FactionID its id). Orbital: a ship over the radio; HoursLeft
	// until it (or the caravan) leaves.
	Trader, TraderKind, Faction string
	FactionID                   int
	FactionType                 faction.Type
	Orbital                     bool
	HoursLeft                   int
	// Negotiator is the colonist trading, with its Social level and the
	// price modifier it earns (PriceModPct: percent trade price improvement,
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
	Rows     []Row
	GiftRows []Row // physical gift quotes; nil when no faction receives gifts
	// Reason is why the last ProposeTrade was refused ("" when none was).
	Reason string
}

// Row is one row of the trade dialog. Kind is the item kind's
// name (item.Kind.String; the key trade.EncodeOffer's caller parses back),
// Label its display name, Category its item category. BuyC is what the
// colony pays for one unit, SellC what it gets for one (hundredths of a
// silver). WillBuy: the trader sells it (it has some); WillSell: the
// trader takes it from the colony.
type Row struct {
	Kind, Label, Category string
	ColonyQty, TraderQty  int
	BuyC, SellC, GiftC    int
	StackID               uint64 // colony stack for selling/gift, trader lot for buying
	WillBuy, WillSell     bool
}

// Comms is the open comms session at a radio console (game.Game.CommsView;
// ok false while none is open).
type Comms struct {
	// Operator is the colonist at Console (the radio console building).
	Operator     entity.EntityID
	OperatorName string
	Console      entity.EntityID
	// Factions lists every known faction except the player, by id.
	Factions []Faction
	// Orbital is the ship in radio range (Present false: none).
	Orbital Orbital
	// BeaconSilver is the silver within reach of a trade beacon (what a
	// gift or an orbital trade can spend).
	BeaconSilver int
}

// Faction is one faction row: its goodwill and relation
// ("hostile", "neutral", "friendly") and each action it allows now.
type Faction struct {
	ID          int
	Name        string
	Type        string // faction.Type key ("settlers", ...)
	FactionType faction.Type
	Goodwill    int
	Relation    string
	Gift        Action
	Caravan     Action
	Aid         Action
}

// Action is one comms action: Allowed now, its Cost (goodwill
// for a request; for a gift the silver per goodwill point), and Reason
// when not allowed ("cooling down: 2 days", "needs goodwill 0", ...).
type Action struct {
	Allowed bool
	Cost    int
	Reason  string
}

// Orbital is the orbital trader in radio range: its ship name and
// trader kind label, the hours before it leaves, and whether it can be
// called now (CallReason: "needs a trade beacon", ...).
type Orbital struct {
	Present    bool
	Ship, Kind string
	HoursLeft  int
	CallOK     bool
	CallReason string
	// CaravanVisiting: a trader caravan is on the map (filled by
	// game.Game.TradeStatus only), from CaravanFaction, leaving in
	// CaravanHoursLeft hours.
	CaravanVisiting  bool
	CaravanFaction   string
	CaravanHoursLeft int
}

// GiftAction reports the native radio gift admission against current
// goodwill and the available beacon stock.
const (
	ReasonNoBeacon   = "needs a powered trade beacon"
	ReasonNoSilver   = "no silver near a trade beacon"
	ReasonNoGiftGain = "goodwill cannot increase"
)

func GiftAction(f faction.Faction, beacons bool, silver int) Action {
	a := Action{Cost: faction.GiftSilverFor(f, f.Goodwill+1)}
	switch {
	case f.Defeated:
		a.Reason = faction.ReasonGone
	case f.Besieged:
		a.Reason = faction.ReasonBesieged
	case !beacons:
		a.Reason = ReasonNoBeacon
	case a.Cost == 0:
		a.Reason = ReasonNoGiftGain
	case silver < a.Cost:
		a.Reason = ReasonNoSilver
	default:
		a.Allowed = true
	}
	return a
}

func OrbitalStatus(name string, kind trade.TraderKind, until, now uint64, present, powered, beacon bool) Orbital {
	if !present {
		return Orbital{CallReason: "no ship in range"}
	}
	if !powered {
		return Orbital{CallReason: "no power"}
	}
	v := Orbital{Present: true, Ship: name, Kind: LowerLabel(trade.DefOf(kind).Label), HoursLeft: HoursLeft(until, now), CallOK: true}
	if !beacon {
		v.CallOK, v.CallReason = false, ReasonNoBeacon
	}
	return v
}

func FactionRow(f faction.Faction, relation string, beacons bool, silver int) Faction {
	return Faction{ID: f.ID, Name: f.Name, Type: f.Type.String(), FactionType: f.Type, Goodwill: f.Goodwill, Relation: relation, Gift: GiftAction(f, beacons, silver)}
}
