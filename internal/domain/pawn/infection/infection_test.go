package infection

import (
	"math/rand"
	"testing"

	"rimworld-go/internal/domain/pawn/body"
)

var infDef = body.DefOfCond(body.CondInfection)

func TestWoundChance(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kind  body.WoundKind
		tendQ int32
		room  int32
		want  int32
	}{
		{body.Bite, -1, RoomOther, 300},
		{body.Burn, -1, RoomClean, 150},
		{body.Cut, -1, RoomOther, 150},
		{body.Cut, 0, RoomOther, 127},   // 150 × 0.85
		{body.Cut, 1000, RoomOther, 7},  // 150 × 0.05
		{body.Bite, 500, RoomClean, 67}, // 300 × 0.45 × 0.5
		{body.Surgical, 1000, RoomClean, 3},
		{body.Bruise, -1, RoomOther, 0},
	}
	for _, tc := range tests {
		if got := WoundChance(tc.kind, tc.tendQ, tc.room); got != tc.want {
			t.Errorf("WoundChance(%d, %d, %d) = %d, want %d", tc.kind, tc.tendQ, tc.room, got, tc.want)
		}
	}
}

func TestUntendedInfectionKills(t *testing.T) {
	t.Parallel()
	c := body.Condition{Kind: body.CondInfection, Sev: Onset}
	if !Losing(c, infDef, false, 1000) {
		t.Fatal("an untended infection is losing")
	}
	hours := 0
	for ; hours < 100; hours++ {
		if dead, _ := StepRace(&c, infDef, false, 1000, 1); dead {
			break
		}
	}
	if hours < 26 || hours > 30 { // (1000−10)/35 per hour
		t.Fatalf("untended death after %d h", hours)
	}
	if c.Stage != 3 {
		t.Fatalf("stage at death %d", c.Stage)
	}
}

func TestWellTendedInfectionIsBeaten(t *testing.T) {
	t.Parallel()
	c := body.Condition{Kind: body.CondInfection, Sev: Onset, TendQ: 900}
	if Losing(c, infDef, true, 1000) || HoursToDeath(c, infDef, true, 1000) != -1 {
		t.Fatal("a 90% tend wins the race")
	}
	for h := 0; h < 200; h++ {
		dead, cured := StepRace(&c, infDef, true, 1000, 1)
		if dead {
			t.Fatalf("died at %d h: %+v", h, c)
		}
		if cured {
			if c.Immunity != Full {
				t.Fatalf("cured without immunity: %+v", c)
			}
			return
		}
	}
	t.Fatalf("never cured: %+v", c)
}

// TestHigherTendQualityWinsMoreRaces: 200 seeded infections tended every
// 12 hours at a random quality around a mean; the better the mean, the
// more are survived.
func TestHigherTendQualityWinsMoreRaces(t *testing.T) {
	t.Parallel()
	survived := func(mean int32, seed int64) int {
		r := rand.New(rand.NewSource(seed))
		won := 0
		for i := 0; i < 200; i++ {
			c := body.Condition{Kind: body.CondInfection, Sev: Onset}
			for h := 0; ; h++ {
				if h%infDef.TendEvery == 0 {
					c.TendQ = min(max(mean*int32(750+r.Intn(501))/1000, 0), 1000)
				}
				dead, cured := StepRace(&c, infDef, true, 1000, 1)
				if dead {
					break
				}
				if cured {
					won++
					break
				}
			}
		}
		return won
	}
	low, mid, high := survived(300, 1), survived(420, 1), survived(800, 1)
	if !(low < mid && mid < high) || high != 200 || low != 0 {
		t.Fatalf("survived of 200: low %d, mid %d, high %d", low, mid, high)
	}
}

func TestStageHysteresis(t *testing.T) {
	t.Parallel()
	if s := stageWithBand(infDef, 335, 0); s != 1 {
		t.Fatalf("enters major at 330: %d", s)
	}
	if s := stageWithBand(infDef, 320, 1); s != 1 {
		t.Fatalf("stays major just under: %d", s)
	}
	if s := stageWithBand(infDef, 305, 1); s != 0 {
		t.Fatalf("leaves major 20 under: %d", s)
	}
	if s := stageWithBand(infDef, 775, 3); s != 2 {
		t.Fatalf("falls from the last stage to extreme, inside its band: %d", s)
	}
	if s := stageWithBand(infDef, 755, 3); s != 1 {
		t.Fatalf("falls from the last stage past extreme: %d", s)
	}
}

func TestImmuneRecedes(t *testing.T) {
	t.Parallel()
	c := body.Condition{Kind: body.CondInfection, Sev: 800, Immunity: Full}
	if Losing(c, infDef, false, 1000) {
		t.Fatal("immune is winning")
	}
	StepRace(&c, infDef, false, 1000, 24)
	if c.Sev != 800-24*29 {
		t.Fatalf("a day immune: %d", c.Sev)
	}
	c.Sev = 10
	if _, cured := StepRace(&c, infDef, false, 1000, 1); !cured {
		t.Fatal("cured at 0")
	}
}
