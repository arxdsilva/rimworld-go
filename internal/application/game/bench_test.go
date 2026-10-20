package game

import "testing"

// Movement and blocking performance gates (SIM-014). Lead-owned: compare
// before/after with
//   go test -run '^$' -bench 'Colony' -benchmem -count 5 ./internal/application/game/

// BenchmarkColonyHour steps the start colony through one in-game hour.
func BenchmarkColonyHour(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		g := NewColony(Config{Seed: 1, Width: 48, Height: 40})
		b.StartTimer()
		g.Step(2500)
	}
}

// BenchmarkColonyRaid steps the colony while three raiders attack.
func BenchmarkColonyRaid(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		g := NewColony(Config{Seed: 1, Width: 48, Height: 40})
		g.Raid(3)
		b.StartTimer()
		g.Step(2500)
	}
}
