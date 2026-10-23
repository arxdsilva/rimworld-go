package planet

import (
	"math"

	"rimworld-go/pkg/noise3"
)

// Climate numbers (NG-1 §4.1).
var (
	tempOffset = [5]float64{-12, -6, 0, 6, 12}
	rainMult   = [5]float64{0.4, 0.7, 1.0, 1.4, 1.8}
)

const (
	tempEquator    = 28.0
	tempPoleDrop   = 52.0
	tempLatExp     = 3.0
	lapsePerKm     = 6.5
	moistureDecay  = 9.0 // tiles
	shadowElev     = 2500
	shadowFactor   = 0.65
	shadowReach    = 3 // tiles upwind checked
	tropicsLatDeg  = 30.0
	maxCoastDist   = 255
	seaIceTempX10  = -80
	rainBandPoints = 6
	tropicReach    = 2.0
	tropicReachLat = 20.0
	rainNoiseAmp   = 0.25 // rain varies ±25% with a smooth noise field
	rainNoiseFreq  = 2.5
)

// rainBand is the latitude rain curve: ≈2400 mm at the equator staying
// wet (2100) to 12°, a dry minimum of ≈300 at 25°, recovering to 800 by 38° and ≈900 at 50° and ≈200
// at the poles, smoothly interpolated between those knots.
var rainBandLat = [rainBandPoints]float64{0, 12, 25, 38, 50, 90}
var rainBandMM = [rainBandPoints]float64{2400, 2100, 300, 800, 900, 200}

func rainBand(latDeg float64) float64 {
	a := math.Abs(latDeg)
	for k := 1; k < rainBandPoints; k++ {
		if a <= rainBandLat[k] {
			t := (a - rainBandLat[k-1]) / (rainBandLat[k] - rainBandLat[k-1])
			t = t * t * (3 - 2*t)
			return rainBandMM[k-1] + (rainBandMM[k]-rainBandMM[k-1])*t
		}
	}
	return rainBandMM[rainBandPoints-1]
}

// TempAt is the yearly mean (°C) at a latitude and elevation for the
// world's temperature setting.
func TempAt(latDeg float64, elevM int, setting int) float64 {
	s := math.Abs(math.Sin(latDeg * math.Pi / 180))
	return tempEquator - float64(tempPoleDrop*math.Pow(s, tempLatExp)) -
		float64(lapsePerKm*math.Max(0, float64(elevM))/1000) + tempOffset[clampInt(setting, 0, 4)]
}

// beginClimate measures every tile's hop distance to the ocean.
func beginClimate(b *Builder) int {
	p := b.w
	b.g.rain = noise3.New(seed64(b.phaseRand(saltClimate)))
	n := p.TileCount()
	queue := make([]int32, 0, n)
	for i := 0; i < n; i++ {
		if p.Water[i] == Ocean {
			p.CoastDist[i] = 0
			queue = append(queue, int32(i))
		} else {
			p.CoastDist[i] = maxCoastDist
		}
	}
	for h := 0; h < len(queue); h++ {
		t := queue[h]
		d := p.CoastDist[t]
		if d >= maxCoastDist-1 {
			continue
		}
		for _, nb := range p.Neighbours(int(t)) {
			if p.CoastDist[nb] > d+1 {
				p.CoastDist[nb] = d + 1
				queue = append(queue, nb)
			}
		}
	}
	return n
}

func itemClimate(b *Builder, i int) {
	p := b.w
	lat := float64(p.Lat[i]) / 10
	t := TempAt(lat, int(p.Elev[i]), b.p.Temp)
	d := float64(p.CoastDist[i])
	// Over the tropics rain is recycled by the forest, so the sea's
	// moisture reaches up to three times farther inland at the equator.
	decay := moistureDecay * (1 + tropicReach*clampF(1-math.Abs(lat)/tropicReachLat, 0, 1))
	moisture := 0.35 + 0.65*math.Exp(-d/decay)
	c := f64(p.Center[i])
	wet := 1 + rainNoiseAmp*(2*b.g.rain.FBm(c[0], c[1], c[2], 3, rainNoiseFreq, 0.5)-1)*2
	wet = clampF(wet, 1-rainNoiseAmp, 1+rainNoiseAmp)
	r := rainBand(lat) * moisture * wet * rainMult[clampInt(b.p.Rain, 0, 4)]
	if p.Water[i] == Land && p.upwindPeak(i) {
		r *= shadowFactor
	}
	p.Temp[i] = int16(math.Round(t * 10))
	p.Rain[i] = int16(math.Round(math.Min(r, math.MaxInt16)))
}

// upwindPeak reports a tile over shadowElev within shadowReach steps
// upwind: westerlies (wind from the west) outside the tropics, easterlies
// inside them.
func (p *Planet) upwindPeak(i int) bool {
	cur := i
	for s := 0; s < shadowReach; s++ {
		east, _ := p.eastNorth(cur)
		lat := math.Abs(float64(p.Lat[cur]) / 10)
		sign := -1.0 // west
		if lat < tropicsLatDeg {
			sign = 1
		}
		c := f64(p.Center[cur])
		best, bestDot := -1, 0.0
		for _, nb := range p.Neighbours(cur) {
			d := sign * dot64(sub64(f64(p.Center[nb]), c), east)
			if d > bestDot {
				best, bestDot = int(nb), d
			}
		}
		if best < 0 {
			return false
		}
		if p.Elev[best] > shadowElev {
			return true
		}
		cur = best
	}
	return false
}

// eastNorth is the local east and north unit vectors at tile i (north is
// +Z; at a pole, east is +Y).
func (p *Planet) eastNorth(i int) (east, north [3]float64) {
	c := f64(p.Center[i])
	h := math.Hypot(c[0], c[1])
	if h < 1e-9 {
		east = [3]float64{0, 1, 0}
	} else {
		east = [3]float64{-c[1] / h, c[0] / h, 0}
	}
	north = cross64(c, east)
	return east, north
}

// bearing is the map edge (Dir8: 0 N, 2 E, 4 S, 6 W) facing neighbour j
// from tile i.
func (p *Planet) bearing(i, j int) uint8 {
	east, north := p.eastNorth(i)
	d := sub64(f64(p.Center[j]), f64(p.Center[i]))
	a := math.Atan2(dot64(d, east), dot64(d, north)) // clockwise from north
	k := int(math.Round(a/(math.Pi/4))) % 8
	if k < 0 {
		k += 8
	}
	return uint8(k)
}
