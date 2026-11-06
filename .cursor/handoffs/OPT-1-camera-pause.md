# OPT-1: camera, pause and window options

Presentation only; the state hash is unchanged.

## What takes effect
- OPT-021 zoom to mouse: `Prefs.CenterZoom` (wheel anchors at the viewport middle when off), `Renderer.applyCamera`.
- OPT-022 smooth jumps: `camera.Fx` glide. A camera move of more than 6 tiles in one frame at the same zoom (jump to, minimap, chase) eases out over ~200 ms. Reduced motion and captures force instant. The glide is a draw-time offset only.
- OPT-023 drag scale: `Prefs.Drag` (DragPct).
- OPT-024 zoom out to world: `applyCamera` returns true for an out notch at the floor with no ease in flight; `App.Update` calls `world.Show("")`.
- OPT-027 shake: `Fx.Shake` from new explosions near the view (`watchBlasts`, threat_ui.go) and lightning flashes (`overlayHost.flash`); decays on the wall clock (`stepCamera` dt); zero under reduced motion and in captures. `fxBegin`/`fxEnd` (layers.go) add and exactly restore the offset around `Renderer.Draw` and the map marks in `App.Draw`.
- OPT-005 auto pause: never / major (threat letters) / threat (danger message or threat letter) / letter (any new non-intro letter, or danger). Gameplay.PauseOnThreat is no longer read. Old letters and messages at start never pause.
- OPT-004 pause on load: first `syncRenderer` of a session with tick > 0.
- OPT-020 unpause speed: `Prefs.LastSpeed`, set on every pause (manual, setSpeed(paused), automatic).
- OPT-029: `applySettings` calls the new `AppConfig.Display(size, borderless)` (lead added it and cmd/game's `applyDisplay`).

## State
No fields or methods added to App or Renderer. `Renderer.pauseOnThreat` was replaced by `smooth.Prefs.Mode`; `Smooth` gained `Prefs` and `Fx` (camera/fx.go).

## Tests
`camera/fx_test.go`; root `camera_options_test.go` (pause modes, unpause speed, pause on load, zoom to mouse, drag, world, shake, glide). Two existing tests changed one line each: `r.pauseOnThreat = false` became `r.smooth.Prefs.Mode = config.AutoPauseNever` (threat_ui_test.go, chase_ui_test.go).

## Left for journeys (W2-JOURNEY)
- Pause modes by a real raid and a letter (default mode is now "major": `TestJourneyTimeThreatPauses` may need a threat letter).
- Space with UnpauseSpeed=last; wheel with zoom-to-mouse off; middle drag at 200 %; zoom out at floor opens world.
- `TestOptionsTogglePauseOnThreat` (threat_ui_test.go) fails: the "options.pausethreat" row is gone from the Options screen (OPT-2's change); it should be removed or repointed to the auto-pause row.
