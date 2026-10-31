package incident

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
)

// TestSkyEvents (FOOD-6, wiki: Events): the eclipse lasts 0.75-1.25 days
// and the fallout 2.5-10.5; one of each at a time; fallout never before
// day 60; both go in the Nature pool at the wiki's weights.
func TestSkyEvents(t *testing.T) {
	t.Parallel()
	if SkyTicks(Eclipse, 0) != 45000 || SkyTicks(Eclipse, SkySpan(Eclipse)) != 75000 ||
		SkyTicks(ToxicFallout, 0) != 150000 || SkyTicks(ToxicFallout, SkySpan(ToxicFallout)) != 630000 || SkyTicks(Party, 5) != 0 {
		t.Fatal("sky event lengths")
	}
	var st State
	now := uint64(70 * clock.TicksPerDay)
	if !st.StartSky(ToxicFallout, now, 1000) || st.StartSky(ToxicFallout, now+10, 1000) || !st.Fallout(now+999) || st.Fallout(now+1000) {
		t.Fatal("one fallout at a time, over at its end")
	}
	sky := func(t uint64) string { s, _ := st.SkyView(t); return s }
	if s, end := st.SkyView(now); s != "toxic fallout" || end != now+1000 || !st.StartSky(Eclipse, now, 500) || sky(now+600) != "toxic fallout" || sky(now+2000) != "" {
		t.Fatal("the view names the sky event")
	}
	if st.StartSky(Party, now, 10) {
		t.Fatal("not a sky event")
	}
	if DefOf(ToxicFallout).Allowed(State{}, 59*clock.TicksPerDay, biome.TemperateForest) || !DefOf(ToxicFallout).Allowed(State{}, 60*clock.TicksPerDay, biome.TemperateForest) {
		t.Fatal("toxic fallout from day 60")
	}
	if DefOf(Eclipse).Weight != 12 || DefOf(ToxicFallout).Weight != 1 || DefOf(Eclipse).Cooldown != 15 || DefOf(ToxicFallout).Cooldown != 90 {
		t.Fatal("wiki weights and cooldowns")
	}
	if title, text := SkyLetter(ToxicFallout, 4*clock.TicksPerDay); title != "Toxic fallout" || text == "" {
		t.Fatal("letter")
	}
}
