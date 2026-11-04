# PERF-003: the render cap (the Mac kernel panics of 2026-11-04)

## The problem

Two kernel panics at 09:36 and 09:43 on 2026-11-04, both from the M2 Max's memory controller (`AMCC0 DCS GROUP 0 CHANNEL 0`), both with `game` as the panicked task a few seconds after launch: ~1.08 GB resident, four running threads (the four `prewarmWorkers` of `art/memory.go` deflating sprites) and the main thread inside Metal drawing the main menu. No panic in the previous ~60 days of uptime.

The trigger was a display change: the DELL 4K panel was switched from 2560x1440 (2x, 5120x2880 backing) to "looks like 3360x1890" (2x, 6720x3780 backing). The game ran fullscreen with HiDPI on, VSync off and the frame cap Off, so the menu drew a 25.4 M pixel frame as fast as the GPU could, while four cores deflated the art. 14.7 M pixels a frame (5K at 2x) had run on this machine for months.

## The fix (adapter only; the simulation is untouched)

1. **Render cap** (`ui_scale.go`, `capDev`): one frame never holds more than `maxRenderPixels` = 5120x2880 physical pixels, nor a side above `maxRenderSide` = 8192 (Metal's texture limit on every Mac). Above either, `dev` shrinks (`sqrt(cap / (w*h))`, below 1 if it must) and the window system scales the frame up. `computeUIScale` is the one place `dev` is decided and `LayoutF` the one place the physical size is born, so the cap holds everywhere: the framebuffer and the map offscreen (`world_scale.go`, physical x world scale ≤ physical) are the only screen-sized allocations. The UI is laid out in points (`autoUIScale`), so nothing changes size; only the map's sharpness above the cap. On the panic's screen: dev 2 → 1.52, physical exactly 5120x2880 (16:9 like the cap), logical 1680x945 at UI scale 2, as uncapped.
2. **60 fps off the colony** (`background.go`, `menuDrawGap`): the menu, the wizard, loading and the prewarm never draw faster than 60 fps, with the cap Off or at any value. Off still means uncapped on the colony, where it is for measuring. VSync, captures and the fps probe stay as they were.
3. **Options** (`options/screen_options.go`, `hidpiRow`): "Sharp on high-density screens" reads "On, limited to WxH" while the cap holds the frame (`Env.RenderLimit` ← `App.renderLimit`), its tooltip names the cap, and the Frame cap tooltip says the menu and loading stay at 60.

Left alone on purpose: `prewarmWorkers` stays 4 (its pressure is small beside the GPU fill); no boot log, the trigger is identified. If the user's test shows 14.7 M is still too much on a mode above the panel's native size, lower `maxRenderPixels` to 4K (3840x2160) and the test table follows.

## Tests (no window)

- `TestRenderCapAnyScreen`: ten screens (720p, 1080p, 1440p at 2x, 4K at 1x and 2x, 5K at 1x, 6K at 2x, 3360x1890 at 2x, 8K at 1x, a 16384-wide ultrawide): physical ≤ cap, sides ≤ 8192, logical ≥ 1280x720, dev untouched under the cap, the physical corners map into the logical window and the logical window covers the frame.
- `TestLayoutFHiDPI`: 3360x1890 at 2x → 5120x2880 and `renderLimit` reports it; 1280x720 at 2x unchanged and uncapped.
- `TestComputeUIScale`: capped dev, dev below 1 (8K at 1x), HiDPI off on a capped screen.
- `TestFrameCapMenuStaysAtSixty`: the menu at Off/120/144 → 1/60 s, at 30 → 1/30 s, VSync → none; the colony keeps Off and 144.
- `TestHiDPIRowNamesTheRenderCap` (options): the row's text with and without the limit.

## The user's checks (each can restart the Mac; save work first)

1. With the new display mode still active: `go run ./cmd/game`, stay on the menu for a minute. No panic: the cap did it.
2. Still a panic: back to 2560x1440 and run again. If that opens, the above-native mode is ground where even the cap is not enough; keep the native mode, and the cap drops to 4K.
3. A panic at 1440 too: hardware or Apple (Apple Diagnostics; the two reports in `/Library/Logs/DiagnosticReports`).

Captures for the HUD work (DOS-10) wait for check 1.

## Third panic, 10:19, with the cap in place

A third report (`panic-full-2026-11-04-102002`) came from a different game binary (UUID `79f898be…` against `e8ab0521…` for the first two), so after the cap landed. Same error registers, same moment: 5.5 s of CPU, 1.07 GB resident, 34.8 k page faults, five threads running in `compress/flate` (the four `prewarmFiles` goroutines and the page cutter), the main thread waiting in a cgo callback. The user saw the top ~20 % of the frame drawn and green below it before the panic: the display engine failing to scan the framebuffer, and the error names the display clients (`DISP0`, `DISPEXT0..3`).

So the render cap is not what decides it: the common load of the three panics is the boot prewarm's burst (four cores deflating ~765 MiB of freshly decoded sprites, ~560 MB of fresh pages in five seconds) while an external display in a scaled mode scans its framebuffer. The same panic string is reported on other M-series Macs with external HiDPI displays under Moonlight, scrcpy and Vectorworks (moonlight-stream/moonlight-qt#1299, Genymobile/scrcpy#5072, BetterDisplay discussion #2981), with no Apple fix; the Moonlight thread reports VSync on stopping it for one user.

Change: `prewarmWorkers` 4 → 1 (`art/memory.go`): the prewarm is background work and now takes ~2 s instead of ~0.5 s. The cap stays: it is still right on its own.

Open: whether the 10:19 run was still on the 3360x1890 mode (the user is asked), the native-mode run (test 2) and, if that panics too, Apple Diagnostics; a hardware fault on DRAM channel 0 that the game's bulk page touch happens to hit is still possible and only the native-mode run and the diagnostics separate the two.
