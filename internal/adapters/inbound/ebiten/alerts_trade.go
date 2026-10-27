package ebiten

import (
	"fmt"

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

// tradeAlerts raises "Trade ship in orbit (Nh left)" and "Trader caravan
// visiting". caravanHours <= 0 shows no countdown.
func tradeAlerts(orbital game.OrbitalView, caravan bool) []alert {
	var out []alert
	if orbital.Present {
		out = append(out, alert{key: "tradeorbit", label: fmt.Sprintf("Trade ship in orbit (%dh left)", max(orbital.HoursLeft, 0))})
	}
	if caravan {
		label := "Trader caravan visiting"
		if orbital.CaravanHoursLeft > 0 {
			label = fmt.Sprintf("Trader caravan visiting (%dh left)", orbital.CaravanHoursLeft)
		}
		out = append(out, alert{key: "tradecaravan", label: label})
	}
	return out
}

// tradeAlertsFrom gathers the status from the game: the status source
// when it has one, else the open comms session's orbital view.
func tradeAlertsFrom(src any) []alert {
	if s, ok := src.(tradeStatusSource); ok {
		st, _ := s.TradeStatus()
		return tradeAlerts(st, st.CaravanVisiting)
	}
	if c, ok := src.(commsSource); ok {
		if v, open := c.CommsView(); open {
			return tradeAlerts(v.Orbital, false)
		}
	}
	return nil
}
