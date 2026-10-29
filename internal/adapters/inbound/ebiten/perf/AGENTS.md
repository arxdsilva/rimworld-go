# perf package

## Package Purpose
The frame-timing instruments: the always-on clock behind the performance overlay (PERF-002), the dev frame-rate probe and the F10 screenshot. `Meter` owns their state; the root feeds it each `Update` and `Draw`. Feature package (ARCH-005 wave 5.3, agent A12).

## Owned Concepts
- `Meter` (15 methods, 10 exported) built by `New(sh Shell) *Meter`: the clock, probe, screenshot and the rolling series.
- `Clock`, `Ring` (fixed-size frame ring buffer), `Series`, `Sample`, `Stats` (`FrameStats(frames, update, draw)`), `Lines` (the overlay's text lines).
- `FPSProbe` (the dev frame-rate probe) and `Probe`; `Shot` (screenshot state: `Last` path).
- `Env`; `Host` (8 methods): `Now`, `Env`, `ResetMaxOwed`, `PerfRect`, `StartProbe`, `Toast`, `ArtMem`, `ForceSpeed`.
- `Shell` = `ports.Settings + Host`.

## Allowed Dependencies
- Leaves: `gfx`, `ports`, `uikit`; `platform/config`, `platform/input`; `domain/sim/clock` (the tick for the probe).
- Ebitengine (`ebiten`); stdlib `image`, `image/png`, `os`, `path/filepath`, `runtime`, `runtime/metrics` (memory and GC counters), `log`, `slices`, `strings`, `time`.

## Forbidden Dependencies
- The ebiten root package and `cmd/`.
- Other feature packages (`hud`, `architect`, `inspector`, `windows`, `orders`, `dialogs`, `learn`, `devtools`, `options`, `wizard`, `mainmenu`, `selection`).
- Simulation mutation: the probe changes speed only through `Host.ForceSpeed`.

## Public Contracts
- `Meter.Begin/End` hooks: the root calls `perfBeginUpdate`, `perfEndUpdate` and `perfEndDraw`, which pass `a.now()` straight through with no interface call on the hot path.
- `FrameStats` is the pure summary used by the overlay and the probe report.

## Invariants
- The ring wraps without growing (`TestRingWraps`); the meter freezes when asked (`TestMeterRecordsFramesAndFreezes`).
- `FrameStats` matches the probe report (`TestFrameStatsMatchProbeReport`).
- Screenshot folder is `<user config dir>/rimworld-go/screenshots`, named after the in-game date.

## Hot Paths
- `EndUpdate` and `EndDraw` run every frame; they append to the ring and must not allocate. No benchmark in this package.

## Determinism Rules
- Wall-clock timing by design: `Host.Now` is the only clock, and it is presentation only. Nothing here feeds the simulation; the probe's speed change is a presentation request.

## Save/Mod Impact
- Writes PNG screenshots to `<user config dir>/rimworld-go/screenshots` (`os.MkdirAll`, `os.Create`, `png.Encode`), the only direct file write in this package tree. Settings and save slots are written through `config` and `options`' `Saver`.

## Tests
- 6 source files, 2 test files, 7 `Test*` functions.
- Coverage 79 % (ARCH-005 row).

## Benchmarks
- None.

## Known Design Constraints
- The root's hot-path wrappers `perfBeginUpdate`, `perfEndUpdate`, `perfEndDraw` stay in the root; they are the only callers on the frame path.
