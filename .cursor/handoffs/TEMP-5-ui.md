# TEMP-5: temperature UI

- Overlay: `render/overlay/temp_draw.go` (`Set.Temp` toggle, `TempDrawState` cache on `WorldView.TempVer`), key `overlay.temperature` = Shift+T (hotkeys.go, input defs). No HUD button: `ports.Overlays` is at 8 methods, so like the fertility overlay it is key-only.
- Inspector: `inspector/temp_ui.go` (vent button `cmd.vent.toggle` emitting `ToggleVent`, passive cooler and heat-rate lines, exposure stage lines, frostbite, animal comfort lines, gear layer order). Hooks in cmds_build, cmds_pawn, cmds_thing, cmds_power, health_ui, inspector.
- HUD: `hud/alerts_temp.go` (`warmclothes`, `animalcold`), volcanic winter chip, `RoomHeatLines` on the room card. Veil: `SkyEventVeil` on `ClimateEvent == "volcanic_winter"`. Zone line: "Growing slowly" outside 6-42 C.
- Original choices: warm-clothes low = biome winter low in fall and winter (no forecast model); danger bands 0 C and 40 C; ramp colours.
- Not done: per-garment insulation lines; selected-room panel (hover card only).
