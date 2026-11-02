# AX-AR2: art for orders (A1), power and industry (A4), fire foam and security (A5)

Agent AR2, 2026-11-01, AX programme wave 1. All art is clean-room original, drawn from code through the Blender pipeline (skill `rimworld-go-art-assets`; ART §1, §2, §12-§16, §35). Footprints follow the construction Defs (wiki sizes). No `shadow:` blocks.

## Files
- New families, all registered in `build.py` FAMILIES:
  - `artlib/families/orders.py` (A1);
  - `industry.py` (A4);
  - `security.py` (A5).
- New palette `assets/palettes/materials_ax_ar2.yaml`: conduit_jacket, foam, foam_paint, warhead_band. It is muted like the others with palette_mute sat 0.8, contrast 0.8, light 0.96, warm 8.
- Generated sprites, metadata and normal maps: `make art FAMILIES="orders industry security"` reproduces them.

## Keys (every one stays unread until its owner wires it)
- **A1:**
  - `item/crated_building`;
  - `ui_icon/order_uninstall|order_chop|order_claim|order_strip|order_open|roof_area_clear|roof_remove_clear|floor_remove`;
  - `overlay/designation_uninstall|designation_open|designation_strip`.
  - Delete zone reuses `ui_icon/zone_delete`, and slaughter reuses `ui_icon/slaughter` (agreed with the lead).
- **A4:**
  - `power/waterproof_conduit_00..15` (autotile conduit);
  - `power/chemfuel_generator` (+glow, `smoke` attachment);
  - `power/watermill_<facing>` (+glow), plus the wheel layer `power/watermill_wheel_<facing>_01..06` (7.5 degrees per frame, looping). The wheel has a tight canvas: draw it at base tile + `offset_tiles`, by its own origin. The base is held out of it.
  - `power/deep_drill_<facing>`, `power/ground_scanner_<facing>`, `power/moisture_pump` (all with glow);
  - `furniture/electric_crematorium_<facing>` (+glow, `stone_tint` mask, worker [1.5, 2.15]);
  - `overlay/deep_ore` (near-white; multiply it by the ore colour, `ore_colour: true`);
  - icons `ui_icon/waterproof_conduit|chemfuel_generator|watermill|deep_drill|ground_scanner|moisture_pump|electric_crematorium`.
- **A5:**
  - `defense/foam_popper`, `defense/ied_foam`, `defense/ied_warhead`;
  - `defense/foam_turret_base` (+glow) and `foam_turret_gun_00..15`;
  - `defense/rocket_battery_base` (+glow) and `rocket_battery_gun_00..15`. Both turrets use the sentry scheme: frame 0 south, clockwise at 22.5 degrees, a shared canvas, and `mount`/`muzzle`.
  - `item/foam_shells`, `item/antimatter_warhead`;
  - `overlay/foam_0..2` (stain decals like `ash_<n>`);
  - `effect/rocket_projectile`, `effect/foam_projectile` (pointing east, origin at the nose);
  - icons `ui_icon/foam_popper|ied_foam|ied_warhead|foam_turret|rocket_battery|foam_shells|antimatter_warhead`.

Every word in these keys already occurs in Go literals, so the orphan check passes without `orphan_allow.txt` lines.

## Checks
- `validate_assets.py`: 6641 sprites, 0 errors from AR2. At the last run, the 4 remaining errors were AR1's ornate-door normal maps, which still need normalgen.
- `atlasgen -check`: ok.
- `TestNoOrphanSprites`, `TestPlantRecipesHaveNoOutline`, `TestGeneratedArtCoversRenderer` and `TestPowerArtCoversRenderer` (-race) pass. They ran in a throwaway HEAD worktree with the live assets, because the shared tree briefly failed to build (another agent's `game/move.go`).
- `normalgen` was run.
- Review sheets were drawn on light and dark ground.

## Open
- **In-game review:** pending until A1/A4/A5 wire the keys. ART rows are marked `[-]` where that review is the missing step.
- **Watermill wheel:** it reads as a paddled drum. If it looks static in play, raise WM_PADDLES or open up the rims.
- **Foam stain:** `stain.Foam` is drawn from sprites (like ash), not baked in code like blood.

## Checklists
ART §32 rows updated: basic resources/items, selection/designation markers, fortify and turret guns, generators/conduits, projectiles, and advanced production/power (now `[-]`).
