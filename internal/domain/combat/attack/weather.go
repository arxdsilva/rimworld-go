package attack

import "rimworld-go/internal/domain/world/weather"

// Weather accuracy factors (permille) beyond ClearFactor and FogFactor.
// wiki: Weather (accuracy multiplier: rain, rainy thunderstorm and both
// snows 80 %, blizzard 70 %; clear, overcast and a dry storm 100 %)
const (
	RainFactor     = 800
	SnowFactor     = 800
	BlizzardFactor = 700
)

// WeatherFactor is the accuracy factor (permille) of a shot taken in
// weather k under the open sky (CBT-2). The game has no fog weather yet:
// FogFactor waits for one.
func WeatherFactor(k weather.Kind) Permille {
	switch k {
	case weather.Rain, weather.Storm:
		return RainFactor
	case weather.Snow:
		return SnowFactor
	case weather.Blizzard:
		return BlizzardFactor
	}
	return ClearFactor
}
