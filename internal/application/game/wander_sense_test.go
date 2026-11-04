package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/weather"
)

// bedroomFor walls a room with one bed owned by owner, returns a cell in it.
func bedroomFor(t *testing.T) (g *Game, owner, guest entity.EntityID, x, y int) {
	t.Helper()
	g, owner, guest = couple(t)
	bed := buildNow(t, g, construction.Bed, item.Wood, construction.South, 8, 8)
	walledRoom(g, 6, 6, 10, 10)
	g.rest.Beds.AssignOwner(bed, owner, 0)
	g.RoofRooms()
	g.ensureRooms()
	if g.roomAt(10, 10) < 0 {
		t.Fatal("no room")
	}
	return g, owner, guest, 10, 10
}

// TestWanderAvoidsOthersBedroom: a visitor's idle stroll keeps out of a
// bedroom it does not own.
func TestWanderAvoidsOthersBedroom(t *testing.T) {
	t.Parallel()
	g, _, guest, x, y := bedroomFor(t)
	if !(senseCourtesy{g}).Rude(guest, x, y) {
		t.Fatal("another's bedroom is rude to enter")
	}
}

// TestWanderOwnBedroomOK: its owner may stroll in its own bedroom.
func TestWanderOwnBedroomOK(t *testing.T) {
	t.Parallel()
	g, owner, _, x, y := bedroomFor(t)
	if (senseCourtesy{g}).Rude(owner, x, y) {
		t.Fatal("own bedroom is fine")
	}
	if (senseCourtesy{g}).Rude(owner, 30, 3) {
		t.Fatal("open ground is fine")
	}
}

// TestWanderRainHides: only soaking weather asks a pawn to hide.
func TestWanderRainHides(t *testing.T) {
	t.Parallel()
	g, a, _ := couple(t)
	g.SetWeather(weather.Clear, 0)
	if (senseCourtesy{g}).ShouldHide(a) {
		t.Fatal("hiding in fair weather")
	}
	g.SetWeather(weather.Rain, 0.5)
	if !(senseCourtesy{g}).ShouldHide(a) {
		t.Fatal("not hiding in the rain")
	}
}

// TestWanderRulesDefaultOff: the shelter and manners rules ship off.
func TestWanderRulesDefaultOff(t *testing.T) {
	t.Parallel()
	g, _, _ := couple(t)
	r := senseOf(g).Rules
	if r.On(gameplay.ShelterWander) || r.On(gameplay.PoliteWander) || !r.On(gameplay.SnowWaitsForSky) {
		t.Fatal("rule defaults moved")
	}
}
