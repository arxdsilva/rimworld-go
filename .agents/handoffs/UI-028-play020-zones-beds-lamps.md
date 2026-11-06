# Task Handoff

Task ID: UI-028 (PLAY-020; plan `.cursor/PLAY020_PLAN.md`)
Status: complete

## Summary
Seven fixes and features from play (user, 2026-10-25).

**Chat bubbles:**
- A bubble is the round interaction icon with nothing behind it; it used to sit in a white box.
- A plain chat shows the chit-chat icon; without art, a dark disc with dots.
- Drawn through the canvas, with the guard exemption removed.

**Growing zone fill:** the wash and stripes skip cells holding the zone's own plants (its crop, or trees in a tree zone), at every zoom. Far out, where plants are baked under the zones, this is what keeps them visible.

**Lamps** (`light_sources.go`, `light_lamp.go`): point lights.
- **Reach:** the light falls off over the metadata `light_radius`, inside its room or, outdoors, in its line of sight.
- **Direction:** packed per tile, so batch builders are unchanged.
- **Glow:** `LampSky` fades it on open sky by day.
- **Shadows:** roofed rooms keep lamp shadows at noon.
- **Sources:** one helper lists every light source; PLAY-021 gates it on power.

**Menu scene:** the house counts as roofed.
- A Blender point light at the bulb shades the furniture toward it, with soft shadows away from it.
- The bulb glows, the outdoor lamp adds a walled-in pool, and the dusk grade is outdoors only.
- Additive `stage.py` helpers; the sun and sprites are untouched.

**Sowing:** a Sow job claims its tile (`startJob`), so one grower per cell. A forced sow on a held cell is refused, and "Sow here" says "sowing is off in this zone".

**Bed rest** (`bedrest.go`):
- **Starts** when one of these holds:
  - healable injury ≥ 40;
  - an untended wound;
  - sick;
  - awaiting surgery.
- **Stops** at ≤ 15, nothing to tend, not sick, and at least 2 h.
- **No free bed:** retry after 6 h; those waiting for a doctor lie on the floor.
- **Self-tend** in bed when there is no other doctor; doctors on bed rest don't count.
- **Hungry patients** eat and return; they used to be put back to bed every tick.
- **Setting:** "Rest when hurt" per colonist, on by default.

**Medical beds:**
- Per bed (`SetBedMedical`).
- No owner, and healthy colonists never use it.
- Patients and rescuers prefer it.
- Hospital beds start medical; save v4 migrates old ones via `migrateSave(v)`, a version switch.
- UI: a "Medical" toggle beside "Prisoners", plus "Rest when hurt" on colonists.

**Zone management** (RimWorld-style):
- **Selecting:** a zone is the last selection candidate on its tile, and its label selects it.
- **Pane:** size, crop, growing now / too cold / too hot, sown, ripe and growth.
- **Commands:** Plant, Allow sowing, Allow cutting, Expand and Shrink (drag-rectangle tools for that zone only, on until right-click or Esc), Delete, Hide, Rename.
- **Sim:** `zone_cmds.go`, plus `zone.Map.AddCells`/`RemoveCells`/`Remove`/`Update`.
  - Settings never affect merging.
  - Zero values mean today's behaviour, so state hashes are unchanged.

## Notes
- **Goldens:** the scale (48x40 seeds 1 and 42, 120x120 seed 7) and grown goldens were re-pinned, because wounded colonists now rest until tended.
- **Tests:** the full `-race` suite is green apart from the known local `TestAtlasCoversKeys` (stale pawn PNGs).
- **RAM budget:** `cmd/rambudget` gained a `capture` kind (2.2 GiB, a small-map game run).
- **Captures:** `-capture-ui zone` and `zone-shrink`.
- **Fixed on the way:** the floating cell info covered a selected zone's pane.
- **Process:** plan mode paused two agents mid-task; they resumed with SendMessage (PARALLEL_WORK note).

## Left open
- Double-click to select all similar zones in view.
- Crop selector columns (skill, growth time, yield).
- Lamp pools step per tile (smoothing would need corner-averaged ground light).
- The overlay family's ash PNGs differ from their source (pre-existing drift; the agent restored the committed PNGs).
- Electricity (PLAY-021) next: lamps will need power, and a torch lamp lights colonies before it.
