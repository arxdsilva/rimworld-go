package plant

// Toxic fallout (FOOD-6, wiki: Toxic fallout): every plant under the open
// sky, trees included, takes 48.9 to 66.1 damage an in-game hour (57.52
// base, ±15 %) until it dies. Damage here is a share of the plant's hit
// points: 85 for a crop, bush or ground plant (wiki: Potato plant), 200
// for a tree (wiki: Oak tree). Each plant keeps its own draw of the ±15 %
// (Roll on its spot), so a field withers over a few hours, not at once.
const (
	falloutDamage = 5752 // hundredths of a hit point an hour
	falloutSpread = 15   // percent either side
	cropHP        = 85
	treeHP        = 200
)

// FalloutRate is the damage plant p takes an hour under the fallout,
// tenths of a percent of its hit points.
func FalloutRate(p Plant) int {
	hp := cropHP
	if DefOf(p.Species).Trunk {
		hp = treeHP
	}
	pct := 100 - falloutSpread + int(Roll(uint64(p.X), uint64(p.Y), uint64(p.Species))%(2*falloutSpread+1))
	return falloutDamage * pct / (10 * hp)
}

// Fallout is p after the fallout's damage over the ticks t0+1 .. t1 under
// the open sky (Dead at full damage).
func (p Plant) Fallout(t0, t1 uint64) Plant {
	if t1 > t0 {
		p.Damage = uint8(min(int(p.Damage)+damageSpan(FalloutRate(p), t0, t1), DamageDead))
	}
	return p
}
