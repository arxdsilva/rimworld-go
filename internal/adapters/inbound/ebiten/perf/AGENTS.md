# internal/adapters/inbound/ebiten/perf

## Purpose
The frame-timing instruments: the always-on clock behind the performance overlay (PERF-002), the dev frame-rate probe and the F10 screenshot. `Meter` owns their state; the root feeds it each `Update` and `Draw`. Feature package (ARCH-005 wave 5.3, agent A12).

## Owns
- `Meter` (15 methods, 10 exported) built by `New(sh Shell) *Meter`: the clock, probe, screenshot and the rolling series.
- `Clock`, `Ring` (fixed-size frame ring buffer), `Series`, `Sample`, `Stats` (`FrameStats(frames, update, draw)`), `Lines` (the overlay's text lines).
- `FPSProbe` (the dev frame-rate probe) and `Probe`; `Shot` (screenshot state: `Last` path).
- `Env`; `Host` (8 methods): `Now`, `Env`, `ResetMaxOwed`, `PerfRect`, `StartProbe`, `Toast`, `ArtMem`, `ForceSpeed`.
- `Shell` = `ports.Settings + Host`.

## Depends on
- Leaves: `gfx`, `ports`, `uikit`; `platform/config`, `platform/input`; `domain/sim/clock` (the tick for the probe).
- Stdlib `runtime/metrics` (memory and GC counters), `image/png`, `os`.

## Must not depend on
- Other feature packages (`hud`, `architect`, `inspector`, `windows`, `orders`, `dialogs`, `learn`, `devtools`, `options`, `wizard`, `mainmenu`, `selection`).
- Simulation mutation: the probe changes speed only through `Host.ForceSpeed`.

## Contracts
- `Meter.Begin/End` hooks: the root calls `perfBeginUpdate`, `perfEndUpdate` and `perfEndDraw`, which pass `a.now()` straight through with no interface call on the hot path.
- `FrameStats` is the pure summary used by the overlay and the probe report.
- Screenshots are PNGs written to `<user config dir>/rimworld-go/screenshots` (`os.MkdirAll`, `os.Create`, `png.Encode`), named after the in-game date; the only direct file write in this package tree.

## Invariants
- The ring wraps without growing (`TestRingWraps`); the meter freezes when asked (`TestMeterRecordsFramesAndFreezes`).
- `FrameStats` matches the probe report (`TestFrameStatsMatchProbeReport`).
- Wall-clock timing by design: `Host.Now` is the only clock, and it is presentation only; the probe's speed change is a presentation request.

## Hot paths
- `EndUpdate` and `EndDraw` run every frame; they append to the ring and must not allocate.

## Tests
- 6 source files, 2 test files, 7 `Test*` functions.
- Coverage 79 % (ARCH-005 row).

## Notes
- The root's hot-path wrappers `perfBeginUpdate`, `perfEndUpdate`, `perfEndDraw` stay in the root; they are the only callers on the frame path.
