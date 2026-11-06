# UI-009 Pause on threat, damage numbers

## What changed
- Sim: `MessageView.Danger` marks a threat arriving; `g.threat(...)` posts one
  (raids, wolf packs). `projectile.Trace` / `TraceView` carry `Damage` and
  `Building` (blows on doors/walls; trap hits too).
- Adapter (`threat_ui.go`):
  - `watchThreats` pauses when a danger message newer than the last handled one
    appears. Messages already present at session start or after a load never pause.
    HUD speed reads "paused (threat)" until the player resumes.
  - Options screen: "Pause on threat: On/Off" toggle (`App.noThreatPause`, copied to
    each new renderer).
  - Floating numbers: each new trace spawns a `floater` ("-N" for hits, light amber
    for buildings, "miss" for missed shots) that rises and fades over 50 frames;
    scaled with zoom. Building hits throw wood chips instead of blood.
- `-capture-ui hits` runs until a blow lands and centres on it.

## Tests
`threat_ui_test.go`: only new danger pauses, once; option off; options toggle;
floater text for hit/miss/melee miss.
