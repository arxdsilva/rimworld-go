# PLAY-022: power follow-ups (lamp bulb, solar weather, smooth lamp light)

These are the "left open" items from ENG-005, made to behave like RimWorld with our own numbers.

## Behaviour
- **Standing lamp**:
  - The bulb is now a `P.glow` part. The sprite shows unlit glass, and the new `furniture/lamp_standing_glow` layer lights it only while the lamp is powered (`drawPowerParts`).
  - Only the lamp was re-rendered.
- **Solar output** follows the weather:
  - `weather.SunFactor(kind, intensity)` (‰) gives clear 1000, cloudy 700, rain 450, storm 350, snow 400, blizzard 250 at full intensity, dimmed by ×(0.5 + 0.5·intensity).
  - Solar output (`power_gen.go`) and the renderer's sun (`weatherSun`) share it.
  - There is no seasonal day length (`lighting.Outdoor` has none), and eclipses are not modelled.
- **Smooth lamp light**:
  - Baked and live ground quads get per-corner lamp levels, each the mean of the open tiles around that corner (walls excluded), drawn through `drawLitGround`.
  - Sprites read the lamp level at their foot from continuous distance (`lightGrid.atF`), with one direction code per sprite.
  - Light falls off smoothly at every zoom.

## Verify
- Tests: `TestUnpoweredLampDrawsNoGlow`, `TestSolarFollowsWeather`, `TestRendererSunUsesDomainFactor`, `TestGroundLampLightInterpolatesAcrossTiles`, `TestLampLevelContinuousAtSpriteFoot`, plus the weather table tests. The full suite passes under `-race`.
- No golden moved.
- `BenchmarkDrawArt500`: z1 +4 %, zmin about +8 % (noisy; the new code is about 1 % of a frame).
- Captures (before/after) are in the session scratchpad, `look/wsp/`.

## Left open
- A per-tile light step remains at roof/sky edges.
- The no-power badge covers the unlit bulb in captures.
