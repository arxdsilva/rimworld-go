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
