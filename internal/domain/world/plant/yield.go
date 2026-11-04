package plant

// Plant Harvest Yield and Plant Work Speed (HRV-005..009, published wiki
// data under ENG-003): plain numbers, no skill or body types.

// harvestYieldByLevel is the yield in permille at Plants level 0..20 with
// healthy Manipulation (wiki: Plant Harvest Yield).
var harvestYieldByLevel = [...]int{600, 700, 750, 800, 850, 900, 950, 975, 1000, 1010, 1020, 1030, 1040, 1050, 1060, 1070, 1080, 1090, 1100, 1120, 1130}

// HarvestYieldPermil is a worker's Plant Harvest Yield in permille: the
// level's table value, Manipulation at 30 % weight, Sight at 20 % weight that
// only lowers it, all held in 0..1500. The capacities are percent.
func HarvestYieldPermil(level, manipPct, sightPct int) int {
	y := harvestYieldByLevel[min(max(level, 0), len(harvestYieldByLevel)-1)]
	y = y * (100 + (manipPct-100)*30/100) / 100
	y = y * min(100, 100+(sightPct-100)*20/100) / 100
	return min(max(y, 0), 1500)
}

// WorkSpeedPermil is Plant Work Speed at Plants level level, in permille:
// 8 % + 11.5 % per level, at least 10 %. Sowing and harvesting follow it.
func WorkSpeedPermil(level int) int { return max(80+115*level, 100) }

// YieldAmount is a harvest of qty at yield permille. Above 100 % the bonus is
// proportional; below it nothing shrinks, since a low yield wastes the whole
// harvest by chance instead (WasteChancePermil).
func YieldAmount(qty, yield int) int {
	if qty <= 0 || yield <= 1000 {
		return max(qty, 0)
	}
	return qty * yield / 1000
}

// WasteChancePermil is the chance a harvest at yield permille comes to nothing
// (0 at 100 % and above, so such a worker never rolls). The wiki gives no
// formula: 100 % minus the yield is ours.
func WasteChancePermil(yield int) int { return max(1000-yield, 0) }
