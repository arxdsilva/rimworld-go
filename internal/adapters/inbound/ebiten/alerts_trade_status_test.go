package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
)

// TestShipAloneRaisesNoCaravanAlert: an orbital ship in range raises only
// the ship alert, never "Trader caravan visiting", even though
// TradeStatus reports something present.
func TestShipAloneRaisesNoCaravanAlert(t *testing.T) {
	t.Parallel()
	got := tradeAlertsFrom(fakeTradeStatus{game.OrbitalView{Present: true, HoursLeft: 30}, true})
	if len(got) != 1 || got[0].key != "tradeorbit" {
		t.Fatalf("ship only: %+v", got)
	}
	got = tradeAlertsFrom(fakeTradeStatus{game.OrbitalView{CaravanVisiting: true, CaravanHoursLeft: 9}, true})
	if len(got) != 1 || got[0].key != "tradecaravan" || got[0].label != "Trader caravan visiting (9h left)" {
		t.Fatalf("caravan only: %+v", got)
	}
}
