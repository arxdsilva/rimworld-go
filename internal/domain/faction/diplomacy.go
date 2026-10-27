package faction

// Diplomacy over the radio (PLAY-024, original tuning in
// .cursor/PLAY024_PLAN.md).

// Diplomacy rules.
const (
	// GiftSilverPerGoodwill: one goodwill per this much silver given, at
	// most GiftGoodwillMax per gift.
	GiftSilverPerGoodwill = 40
	GiftGoodwillMax       = 15
	// AidCost is the goodwill military aid costs (friendly factions only),
	// AidCooldownDays the days before the faction answers again.
	AidCost         = 20
	AidCooldownDays = 3
	// CaravanCost is the goodwill a trade caravan request costs; it needs
	// goodwill of at least CaravanMinGoodwill, arrives after
	// CaravanArriveMinHours..CaravanArriveMaxHours and can be asked again
	// after CaravanCooldownDays.
	CaravanCost           = 12
	CaravanMinGoodwill    = 0
	CaravanArriveMinHours = 6
	CaravanArriveMaxHours = 12
	CaravanCooldownDays   = 4
)

// Player-facing refusal reasons.
const (
	ReasonGone       = "that faction is gone"
	ReasonHostile    = "they are hostile"
	ReasonNotFriend  = "they are not friendly enough"
	ReasonGoodwill   = "they don't trust you enough"
	ReasonCoolingOff = "they answered recently"
)

// Cooldowns is when a faction answers requests again: the tick each
// cooldown ends (0: none). Saved by the game, per faction.
type Cooldowns struct {
	AidUntil     uint64
	CaravanUntil uint64
}

// GiftGoodwill is the goodwill a gift worth silver (whole silver) earns.
func GiftGoodwill(silver int) int {
	if silver <= 0 {
		return 0
	}
	return min(silver/GiftSilverPerGoodwill, GiftGoodwillMax)
}

// CanRequestAid reports whether f answers a military aid request at tick
// now, or the player-facing reason it won't.
func CanRequestAid(f Faction, cd Cooldowns, now uint64) (bool, string) {
	switch {
	case f.Defeated:
		return false, ReasonGone
	case f.Relation == Hostile:
		return false, ReasonHostile
	case f.Relation != Friendly:
		return false, ReasonNotFriend
	case f.Goodwill < AidCost:
		return false, ReasonGoodwill
	case now < cd.AidUntil:
		return false, ReasonCoolingOff
	}
	return true, ""
}

// CanRequestCaravan reports whether f sends a trade caravan on request at
// tick now, or the player-facing reason it won't.
func CanRequestCaravan(f Faction, cd Cooldowns, now uint64) (bool, string) {
	switch {
	case f.Defeated:
		return false, ReasonGone
	case f.Relation == Hostile:
		return false, ReasonHostile
	case f.Goodwill < CaravanMinGoodwill:
		return false, ReasonGoodwill
	case now < cd.CaravanUntil:
		return false, ReasonCoolingOff
	}
	return true, ""
}
