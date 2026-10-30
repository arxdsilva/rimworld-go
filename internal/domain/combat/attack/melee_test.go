package attack

import (
	"testing"

	"rimworld-go/internal/domain/world/weather"
)

// TestMeleeHitCurve: the wiki's post-processed hit chances. wiki: Melee
// hit chance
func TestMeleeHitCurve(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ skill, want int }{
		{-25, 50}, {-20, 50}, {-15, 75}, {-10, 100}, {-5, 300}, {0, 500}, {5, 650}, {10, 800},
		{15, 850}, {20, 900}, {30, 930}, {40, 960}, {50, 970}, {60, 980}, {80, 980},
	} {
		if got := HitChanceOf(Fresh(c.skill)); got != c.want {
			t.Errorf("score %d: %d, want %d", c.skill, got, c.want)
		}
	}
	if got := HitChanceOf(Melee{Sight: 1000, Manip: 1000}); got != 620 {
		t.Fatalf("skill-less creature: %d, want the wiki's 62 %%", got)
	}
	m := Fresh(10)
	m.Sight, m.Manip, m.Brawler = 2000, 1250, true // +6 (capped at 150 %), +3, +4
	if HitScore(m) != 23_000 {
		t.Fatalf("score %d", HitScore(m))
	}
	m = Fresh(20)
	m.Offset = 200
	if HitChanceOf(m) != 1000 {
		t.Fatal("offset clamps")
	}
}

// TestMeleeDodge: none to a score of 5, 2 % a point to 20, 0.5 % a point to
// 60, capped at 50 %. wiki: Melee dodge chance
func TestMeleeDodge(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ skill, want int }{{0, 0}, {5, 0}, {10, 100}, {20, 300}, {40, 400}, {60, 500}, {90, 500}} {
		if got := DodgeChanceOf(Fresh(c.skill)); got != c.want {
			t.Errorf("score %d: %d, want %d", c.skill, got, c.want)
		}
	}
	m := Fresh(10)
	m.Moving, m.Sight, m.Nimble = 1580, 2000, true // +10.44, +3.2 (capped), +15
	if DodgeScore(m) != 38_640 {
		t.Fatalf("dodge score %d", DodgeScore(m))
	}
	m.Down = true
	if DodgeChanceOf(m) != 0 {
		t.Fatal("a downed fighter never dodges")
	}
	if DodgeChanceOf(Melee{Sight: 1000, Moving: 1000}) != 0 {
		t.Fatal("no skill, no dodge")
	}
}

func TestMeleeDamageAndXP(t *testing.T) {
	t.Parallel()
	at := func(r int) func(int) int { return func(int) int { return r } }
	if MeleeDamage(10, at(0)) != 8 || MeleeDamage(10, at(400)) != 12 || MeleeDamage(10, at(200)) != 10 ||
		MeleeDamage(1, at(0)) != 1 || MeleeDamage(0, at(0)) != 0 {
		t.Fatal("x0.8..1.2")
	}
	if CombatXP(true, 120, 1) != 160 || CombatXP(false, 120, 1) != 60 || CombatXP(true, 120, 2) != 80 || CombatXP(false, 0, 0) != 1 {
		t.Fatal("experience by cycle, hit and miss")
	}
}

func TestWeatherFactor(t *testing.T) {
	t.Parallel()
	for k, want := range map[weather.Kind]int{weather.Clear: 1000, weather.Cloudy: 1000, weather.Rain: 800,
		weather.Storm: 800, weather.Snow: 800, weather.Blizzard: 700} {
		if got := WeatherFactor(k); got != want {
			t.Errorf("%v: %d, want %d", k, got, want)
		}
	}
}
