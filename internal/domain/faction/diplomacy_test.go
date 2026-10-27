package faction

import "testing"

func TestGiftGoodwillCapped(t *testing.T) {
	cases := map[int]int{-5: 0, 0: 0, 39: 0, 40: 1, 400: 10, 600: 15, 100000: 15}
	for silver, want := range cases {
		if got := GiftGoodwill(silver); got != want {
			t.Errorf("%d silver: %d want %d", silver, got, want)
		}
	}
}

func TestAidNeedsFriendly(t *testing.T) {
	f := Faction{Relation: Friendly, Goodwill: 50}
	if ok, why := CanRequestAid(f, Cooldowns{}, 0); !ok || why != "" {
		t.Fatal(why)
	}
	for _, f := range []Faction{
		{Relation: Neutral, Goodwill: 50}, {Relation: Hostile, Goodwill: 50},
		{Relation: Friendly, Goodwill: AidCost - 1}, {Relation: Friendly, Goodwill: 50, Defeated: true},
	} {
		if ok, why := CanRequestAid(f, Cooldowns{}, 0); ok || why == "" {
			t.Fatalf("%+v allowed", f)
		}
	}
}

func TestRequestCooldown(t *testing.T) {
	f := Faction{Relation: Neutral, Goodwill: 5}
	cd := Cooldowns{AidUntil: 100, CaravanUntil: 100}
	if ok, why := CanRequestCaravan(f, cd, 99); ok || why != ReasonCoolingOff {
		t.Fatal("cooling")
	}
	if ok, _ := CanRequestCaravan(f, cd, 100); !ok {
		t.Fatal("cooldown over")
	}
	if ok, _ := CanRequestCaravan(Faction{Goodwill: -1}, Cooldowns{}, 0); ok {
		t.Fatal("needs goodwill >= 0")
	}
	if ok, why := CanRequestCaravan(Faction{Relation: Hostile}, Cooldowns{}, 0); ok || why != ReasonHostile {
		t.Fatal("hostile")
	}
	if ok, why := CanRequestCaravan(Faction{Defeated: true}, Cooldowns{}, 0); ok || why != ReasonGone {
		t.Fatal("gone")
	}
	fr := Faction{Relation: Friendly, Goodwill: 50}
	if ok, why := CanRequestAid(fr, Cooldowns{AidUntil: 10}, 5); ok || why != ReasonCoolingOff {
		t.Fatal("aid cooling")
	}
}
