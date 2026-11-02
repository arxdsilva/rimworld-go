package infection

import "testing"

// TestRoomCleanFactor (AX-3): a clean room on plain floors halves the
// chance, one in sterile tile throughout cuts it to 30 %, a part-tiled
// room lies between, and dirty floors never raise it above the clean
// room's factor.
func TestRoomCleanFactor(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ clean, want int32 }{{0, RoomClean}, {60, RoomSterile}, {30, 400}, {-10, RoomClean}, {200, RoomSterile}} {
		if got := RoomCleanFactor(c.clean); got != c.want {
			t.Errorf("RoomCleanFactor(%d) = %d, want %d", c.clean, got, c.want)
		}
	}
}

// TestRoomFactor (AX-3): a room that is not clean keeps the whole chance
// whatever its floors.
func TestRoomFactor(t *testing.T) {
	t.Parallel()
	if RoomFactor(false, 60) != RoomOther || RoomFactor(true, 60) != RoomSterile || RoomFactor(true, 0) != RoomClean {
		t.Fatal("room factor")
	}
}
