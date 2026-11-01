package overlay

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestTempTintRamp(t *testing.T) {
	t.Parallel()
	cold, comfy, hot := TempTint(-250), TempTint(210), TempTint(500)
	if !(cold.B > cold.R) || !(hot.R > hot.B) {
		t.Fatalf("cold %v should be blue and hot %v red", cold, hot)
	}
	if comfy.A >= cold.A || comfy.A >= hot.A {
		t.Fatalf("comfortable %v should be fainter than the extremes %v %v", comfy, cold, hot)
	}
	if TempTint(-900) != TempTint(-300) || TempTint(900) != TempTint(600) {
		t.Fatal("the ramp clamps at its ends")
	}
	if TempTint(100) == TempTint(101) && TempTint(100) == TempTint(110) {
		t.Fatal("the ramp is continuous, not flat")
	}
}

func TestTempDangerAndLabel(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		tenths int
		danger bool
		label  string
	}{{-55, true, "-6 °C"}, {0, true, "0 °C"}, {60, false, "6 °C"}, {210, false, "21 °C"}, {399, false, "39 °C"}, {400, true, "40 °C"}} {
		if TempDangerous(c.tenths) != c.danger || TempLabel(c.tenths) != c.label {
			t.Errorf("%d tenths: danger %v label %q, want %v %q", c.tenths, TempDangerous(c.tenths), TempLabel(c.tenths), c.danger, c.label)
		}
	}
}

// The per-room cache rebuilds on a new TempVer, outdoor temperature or room
// count only, and a quiet frame allocates nothing.
func TestTempCacheOnTempVer(t *testing.T) {
	w := game.WorldView{TerrainW: 4, OutdoorTemp: -100, TempVer: 1,
		Rooms: []game.RoomView{{ID: 0, Temp: 210}, {ID: 1, Temp: -50}}}
	var s TempDrawState
	s.sync(&w)
	if s.Builds != 1 || s.tint[0] != TempTint(210) || !s.danger[1] || s.label[0] != "21 °C" || s.outLbl != "-10 °C" {
		t.Fatalf("builds %d tint %v danger %v labels %q %q", s.Builds, s.tint, s.danger, s.label, s.outLbl)
	}
	if n := testing.AllocsPerRun(50, func() { s.sync(&w) }); n != 0 || s.Builds != 1 {
		t.Fatalf("a quiet frame rebuilt %d times and allocated %v", s.Builds, n)
	}
	w.TempVer++
	w.Rooms[0].Temp = 300
	s.sync(&w)
	if s.Builds != 2 || s.tint[0] != TempTint(300) {
		t.Fatalf("a new TempVer did not rebuild: builds %d", s.Builds)
	}
}
