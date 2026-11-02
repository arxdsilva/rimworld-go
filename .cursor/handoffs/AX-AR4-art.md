# AX-AR4: art for the escape ship (A8) and for caskets, pods, scanner and hitching spot (A9)

Agent AR4, 2026-11-02, AX programme wave 2. All art is clean-room original, drawn from code through the Blender pipeline (skill `rimworld-go-art-assets`; ART §1, §2, §12-§16, §35). Footprints follow the construction Defs (wiki sizes). No `shadow:` blocks and no new palette: the art uses existing materials and emissives only.

## Files
- **New families**, both registered in `build.py` FAMILIES:
  - `artlib/families/starship.py` (A8);
  - `artlib/families/transit.py` (A9 and the world-map markers).
- **Generated** sprites, glow layers, metadata, normal maps and icons. `make art FAMILIES="starship transit"` reproduces them.
- **ART doc:** a new §42 lists every key; the Wave 8/9 rows are updated.

## Keys (every one stays unread until A8/A9 wire it)
- **A8** (structure family unless noted):
  - `ship_beam_<facing>` (2x6), with lamp glow. It is not an autotile family; its end flanges make adjacent beams read as joined;
  - `ship_casket_<facing>` (empty, no glow) and `ship_casket_occupied_<facing>`, with glow;
  - `ship_core` and `ship_sensor` (2x2), each with glow;
  - `ship_reactor` (6x7). Its `ship_reactor_glow` is the active/started state;
  - `ship_engine_<facing>` (3x4), with throat glow;
  - `item/mind_core`, `effect/launch_flare`;
  - icons `ui_icon/ship_beam|ship_casket|ship_core|ship_reactor|ship_engine|ship_sensor|mind_core`.
- **A9:**
  - `furniture/stasis_casket_<facing>` (empty) and `furniture/stasis_casket_occupied_<facing>`, with glow;
  - `structure/transport_pod`;
  - `structure/pod_launcher_<facing>`. Its `gauge0`/`gauge1` attachments mark the fuel slot, as on the battery, so `overlay.BatteryBarRect` can be reused;
  - `power/mineral_scanner_<facing>`. Its glow is the working state;
  - `overlay/hitching_spot`;
  - world markers `ui_icon/pod_flight` and `ui_icon/mining_site`, in the `ui_icon/caravan` badge style;
  - icons `ui_icon/stasis_casket|transport_pod|pod_launcher|mineral_scanner|hitching_spot`.

Every word in these keys already occurs in Go literals, so no `orphan_allow.txt` lines are needed.

## Checks
- `make art-check` passes: 6753 sprites with 0 errors, the atlas is ok, and the orphan, plant-outline and audio tests pass with -race.
- Normal maps were generated (`cmd/normalgen`).
- Review sheets were drawn on light soil and on dark ground at night, with the glow layers.

## Open
- **In-game review:** pending until A8/A9 wire the keys. The ART rows are `[-]` for that reason.
- **Victory presentation:** the ending screen has no art yet (ART Wave 9 `[ ]`).
- **Pod launch:** a rising-pod streak can reuse `effect/pod_fall` flipped vertically, if A9 wants one.
