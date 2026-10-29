package room

// Vanilla room statistics (BED-2) as pure integer functions, numbers from
// rimworldwiki.com/wiki/Room. Fixed-point: space in tenths, beauty in tenths,
// cleanliness in hundredths, wealth in silver, impressiveness in tenths.

// FullContents extends Contents with the facts the new roles need.
type FullContents struct {
	Contents
	MedicalBeds, PrisonerBeds int
	RecBuildings, Stoves      int
}

// RoleOfFull is RoleOf plus hospital, prison, recreation and kitchen rooms.
// Bedroom/barracks/hospital outrank dining and recreation, and dining
// (12 per table) beats recreation (7 per item) on score, ties to dining
// (wiki: Room role scores). Prison and kitchen priority, and workshop
// outranking both, are original tuning. Without any of the new facts it
// equals RoleOf.
func RoleOfFull(c FullContents) Role {
	switch {
	case c.PrisonerBeds >= 2:
		return PrisonBarracks
	case c.PrisonerBeds == 1:
		return PrisonCell
	case c.MedicalBeds > 0:
		return Hospital
	case c.Stoves > 0 && c.Beds == 0:
		return Kitchen
	}
	if c.Beds > 0 || c.Benches > 0 {
		return RoleOf(c.Contents)
	}
	if c.RecBuildings > 0 && 7*c.RecBuildings > 12*c.Tables {
		return RecRoom
	}
	return RoleOf(c.Contents)
}

// Inputs are the plain measurements behind the room stats.
type Inputs struct {
	Cells, Impassable     int // cells of the room; blocking furniture cells
	BeautySumTenths       int // sum of cell beauty, tenths
	Wealth                int // market value of items, buildings and floors
	CleanlinessHundredths int // average tile cleanliness, hundredths
}

// Stats are the computed room statistics.
type Stats struct {
	SpaceTenths, BeautyTenths, Wealth, CleanlinessHundredths int
	ImpressivenessTenths                                     int
}

// Space is usable space in tenths: 1.4 per cell minus 0.9 per cell blocked
// by furniture (wiki: Room).
func Space(cells, impassable int) int {
	if cells <= 0 {
		return 0
	}
	impassable = max(0, min(impassable, cells))
	return 14*cells - 9*impassable
}

// SpaceLabel names space in tenths (wiki: Room).
func SpaceLabel(tenths int) string {
	switch {
	case tenths < 125:
		return "cramped"
	case tenths < 290:
		return "rather tight"
	case tenths < 550:
		return "average-sized"
	case tenths < 700:
		return "somewhat spacious"
	case tenths < 1300:
		return "quite spacious"
	case tenths < 3495:
		return "very spacious"
	}
	return "extremely spacious"
}

// Beauty is the room beauty in tenths: total over the cell count, with
// rooms of 40 cells or fewer divided by 20+size/2 instead (wiki: Room).
func Beauty(sumTenths, cells int) int {
	if cells <= 0 {
		return 0
	}
	div2 := 2 * cells // twice the divisor
	if cells <= 40 {
		div2 = 40 + cells
	}
	return sumTenths * 2 / div2
}

// BeautyLabel names a room beauty in tenths (wiki: Room).
func BeautyLabel(tenths int) string {
	switch {
	case tenths < -35:
		return "hideous"
	case tenths < 0:
		return "ugly"
	case tenths < 24:
		return "neutral"
	case tenths < 50:
		return "pretty"
	case tenths < 150:
		return "beautiful"
	case tenths < 500:
		return "very beautiful"
	case tenths < 1000:
		return "extremely beautiful"
	}
	return "unbelievably beautiful"
}

// WealthLabel names a wealth total (wiki: Room thresholds; the middle
// names are original wording).
func WealthLabel(w int) string {
	for _, s := range [...]struct {
		below int
		name  string
	}{
		{500, "impoverished"}, {700, "poor"}, {2000, "modest"}, {4000, "comfortable"},
		{10000, "well-off"}, {40000, "rich"}, {100000, "very rich"}, {1000000, "extremely rich"},
	} {
		if w < s.below {
			return s.name
		}
	}
	return "unbelievably luxurious"
}

// CleanlinessLabel names average cleanliness in hundredths (wiki: Room).
func CleanlinessLabel(hundredths int) string {
	switch {
	case hundredths < -110:
		return "very dirty"
	case hundredths < -40:
		return "dirty"
	case hundredths < -5:
		return "slightly dirty"
	case hundredths < 40:
		return "clean"
	}
	return "sterile"
}

const micro = 1_000_000

// Impressiveness4 is the vanilla impressiveness in tenths of a point: bases
// wealth/1500, beauty/3, space/125, 1+cleanliness/2.5, each log-curved, then
// 65 x mean + 35 x min (wiki: Room). The wiki's space cap (500, flagged
// unverified there) is not applied: original tuning.
func Impressiveness4(wealth, beautyTenths, spaceTenths, cleanlinessHundredths int) int {
	terms := [4]int64{
		curve(int64(wealth) * micro / 1500),
		curve(int64(beautyTenths) * micro / 30),
		curve(int64(spaceTenths) * micro / 1250),
		curve(micro + int64(cleanlinessHundredths)*micro/250),
	}
	sum, lo := int64(0), terms[0]
	for _, t := range terms {
		sum += t
		lo = min(lo, t)
	}
	v := 65*sum/4 + 35*lo // micro points
	return int(v / (micro / 10))
}

// curve is b for |b| < 1 and +-(1+ln|b|) beyond, in micro units.
func curve(b int64) int64 {
	switch {
	case b >= micro:
		return micro + lnMicro(b)
	case b <= -micro:
		return -(micro + lnMicro(-b))
	}
	return b
}

// lnMicro is ln(x/1e6) in micro units for x >= 1e6, integer-only so it is
// identical on every platform: range-reduce to [1,2) then atanh series.
func lnMicro(x int64) int64 {
	const ln2 = 693147
	k := int64(0)
	for x >= 2*micro {
		x /= 2 // truncation error below 1e-6 relative
		k++
	}
	// ln(m) = 2*atanh(z), z = (m-1)/(m+1)
	z := (x - micro) * micro / (x + micro)
	z2 := z * z / micro
	term, sum := z, int64(0)
	for n := int64(1); n < 15; n += 2 {
		sum += term / n
		term = term * z2 / micro
	}
	return k*ln2 + 2*sum
}

// ImpressivenessLabel names an impressiveness in tenths (wiki: Room).
func ImpressivenessLabel(tenths int) string {
	for _, s := range [...]struct {
		below int
		name  string
	}{
		{200, "awful"}, {300, "dull"}, {400, "mediocre"}, {500, "decent"},
		{650, "slightly impressive"}, {850, "somewhat impressive"}, {1200, "very impressive"},
		{1700, "extremely impressive"}, {2400, "unbelievably impressive"},
	} {
		if tenths < s.below {
			return s.name
		}
	}
	return "wondrously impressive"
}

// StatsOf computes every room statistic from plain inputs.
func StatsOf(in Inputs) Stats {
	st := Stats{
		SpaceTenths:           Space(in.Cells, in.Impassable),
		BeautyTenths:          Beauty(in.BeautySumTenths, in.Cells),
		Wealth:                max(0, in.Wealth),
		CleanlinessHundredths: in.CleanlinessHundredths,
	}
	st.ImpressivenessTenths = Impressiveness4(st.Wealth, st.BeautyTenths, st.SpaceTenths, st.CleanlinessHundredths)
	return st
}
