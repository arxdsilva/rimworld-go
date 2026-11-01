package env

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestSkyEventVeil(t *testing.T) {
	t.Parallel()
	if SkyEventVeil(game.WorldView{}, 5).A != 0 {
		t.Fatal("clear sky, no veil")
	}
	ec := SkyEventVeil(game.WorldView{SkyEvent: "eclipse", SkyEventEnds: 10000}, 100)
	fo := SkyEventVeil(game.WorldView{SkyEvent: "toxic fallout", SkyEventEnds: 10000}, 100)
	if ec.A == 0 || fo.A == 0 || fo.G <= fo.B || ec.A <= fo.A {
		t.Errorf("eclipse %v fallout %v", ec, fo)
	}
	if end := SkyEventVeil(game.WorldView{SkyEvent: "eclipse", SkyEventEnds: 10000}, 9900); end.A >= ec.A {
		t.Errorf("fades out at the end: %v vs %v", end, ec)
	}
}

func TestVolcanicWinterVeil(t *testing.T) {
	t.Parallel()
	w := game.WorldView{ClimateEvent: "volcanic_winter", ClimateEventEnds: 100000}
	if v := SkyEventVeil(w, 100); v.A == 0 || v.R == 0 {
		t.Fatalf("no ash haze in a volcanic winter: %v", v)
	}
	w.ClimateEvent = "heat_wave"
	if SkyEventVeil(w, 100).A != 0 {
		t.Fatal("a heat wave has no veil")
	}
	w.ClimateEvent = "volcanic_winter"
	if end := SkyEventVeil(w, w.ClimateEventEnds-10); end.A >= SkyEventVeil(w, 100).A {
		t.Fatal("the haze fades out before the event ends")
	}
}
