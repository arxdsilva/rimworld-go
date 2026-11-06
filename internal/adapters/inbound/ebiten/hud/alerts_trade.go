package hud

import (
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
)

// Trade alerts (PLAY-024): an orbital ship in radio range and a trader
// caravan camped near the colony. Both are informational (warning tier,
// never danger) and click through to nothing in particular (no targets).

// tradeStatusSource is what the alerts read beyond the open sessions:
// the orbital ship in range and the visiting trader caravan
// (OrbitalView.CaravanVisiting); any reports either is present.
type tradeStatusSource interface {
	TradeStatus() (status game.OrbitalView, any bool)
}

var _ tradeStatusSource = (*game.Game)(nil)

// commsSource is the open comms session (consumer-owned; optional on the
// simulation, like the dialogs' own reader).
type commsSource interface {
	CommsView() (game.CommsView, bool)
}

// tradeAlerts raises "Trade ship in orbit (Nh left)" and "Trader caravan
// visiting". caravanHours <= 0 shows no countdown.
func tradeAlerts(tr lang.Tr, orbital game.OrbitalView, caravan bool) []Alert {
	var out []Alert
	if orbital.Present {
		out = append(out, Alert{Key: "tradeorbit", Label: lang.Tf(tr, "Trade ship in orbit (%dh left)", max(orbital.HoursLeft, 0))})
	}
	if caravan {
		label := tr.T("Trader caravan visiting")
		if orbital.CaravanHoursLeft > 0 {
			label = lang.Tf(tr, "Trader caravan visiting (%dh left)", orbital.CaravanHoursLeft)
		}
		out = append(out, Alert{Key: "tradecaravan", Label: label})
	}
	return out
}

// TradeAlertsFromIn gathers the status from the game: the status source
// when it has one, else the open comms session's orbital view; then the
// allies on the map (alerts_allies.go, Phase 4).
func TradeAlertsFromIn(tr lang.Tr, src any) []Alert {
	var out []Alert
	if s, ok := src.(tradeStatusSource); ok {
		st, _ := s.TradeStatus()
		out = tradeAlerts(tr, st, st.CaravanVisiting)
	} else if c, ok := src.(commsSource); ok {
		if v, open := c.CommsView(); open {
			out = tradeAlerts(tr, v.Orbital, false)
		}
	}
	return append(out, AllyAlertsFromIn(tr, src)...)
}
