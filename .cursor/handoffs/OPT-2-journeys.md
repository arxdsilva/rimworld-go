# OPT-2: options and camera journeys (W2-JOURNEY)

Tests only (root ebiten package). Files: `journey_options_test.go`, `journey_camera_options_test.go` (`journey_camera_test.go` already exists and is not ours).
Total time under -race: about 6 s (autosave rotation 2.7 s, the rest under 0.5 s each).

## Journeys
- Options: auto pause cycle, restore defaults (cancel keeps, OK resets, bindings kept), preferred names dialog (OK stores, Cancel does not), Developer tab only with dev mode, Language row shows English, Kelvin (row cycle and the map temperature overlay unit).
- Saves: autosave rotation with 3 kept (-autosave-1,2,3 then 1 again), Load list pages at 8 plus "More saves".
- Pause/speed: each auto-pause mode against a real raid, Space resumes at last speed, pause on load on/off.
- Camera: zoom to mouse off, drag 200 %, zoom out to globe, window size preset calls `AppConfig.Display`, shake off in captures, hats only on the map (portrait layers via `pawnDressLayers`, plumbing from the Options click).

## Not covered
- Saves/log folder buttons: `optionsHost.Env` (options_compat.go) never fills `Env.OpenFolder`/`SavesDir`/`LogDir`, so there is no seam; a click would open a real folder.
- Danger-message-only vs letter distinction (`major` vs `threat`): a real raid always sends both; covered by unit `TestPauseModes`.
- Hats: no parallel-safe draw trace, so the journey checks the layer list, not pixels.

## Open
`TestEveryInputActionHasAJourney` currently fails on ConsumeFromPack, DropGear, SetApparelForced (gear wave, not ours).
