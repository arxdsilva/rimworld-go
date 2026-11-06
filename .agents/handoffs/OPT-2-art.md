# OPT-2 art: Gear tab icons, the game's pointer, colour-vision palettes

This covers the art for GEAR-2 (Gear tab buttons) and OPT-028 (custom cursor), plus the colour-vision values for OPT-018. The art is our own design, with nothing traced. Nothing reads it yet: the readers land in Wave 2.

## What changed
- `assets/source/blender/artlib/families/gearopt.py` is a new family. Rebuild it with `make art FAMILIES=gearopt`.
  - `ui_icon/gear_lock`: a shut brass padlock. `gear_unlock`: the same lock in chrome grey with its shackle swung open, so the pair reads as one toggle.
  - `ui_icon/gear_drop`: a tied sack on a ground line, with a down arrow beside it.
  - `ui_icon/gear_eat`: a drumstick. The map's eat order keeps its plate (`ui_icon/eat`).
  - `ui_icon/gear_info`: a bold "i" with a flag.
  - All five sit on the order-badge disc (`overlay._badge_icon`): 128 px, @2x of 64, the same stroke and halo as `equip`, `wear` and `eat`.
- New metadata family `cursor`:
  - `cursor/pointer` is an off-white dart with a near-black ink line. Its lower-right facet is shaded away from the upper-left light.
  - `cursor/pointer_busy` adds a gold progress arc on a dark disc at the lower right.
  - The 64 px canvas is the @2x master of a 32 px OS cursor (`scale: 2`). The pipeline has no density variants, so the renderer scales this one master with the UI.
  - The hotspot is `attachments.hotspot` = `[7, 5]` in canvas px. `art.Meta.Attach["hotspot"]` reads it with no code change, and the catalog serves the sprites as `cursor/pointer` and `cursor/pointer_busy`.
  - PNGs are in `generated/ui/cursors/`. Ebitengine 2.9 has only system cursor shapes, so OPT-028 hides the system cursor and draws this sprite.
- `assets/source/blender/build.py`: `"gearopt"` added to `FAMILIES` (approved by the lead). No other pipeline code changed.
- `assets/metadata/orphan_allow.txt`: the `gear_*` icons ("drawn by GEAR-2 Gear tab (Wave 2)") and `metadata/cursor/pointer*.yaml` ("OPT-028 cursor (Wave 2)"). Remove these lines when the readers land.
  - There is no PNG line for the cursors: `orphan_assets_test.go` `spriteDirs` has no `cursor` entry, so it looks in `generated/world/cursor` and a glob on `generated/ui/cursors` is reported stale.
  - Suggested follow-up for the owner of that test: add `"cursor": "generated/ui/cursors"`.

## Colour-vision values (OPT-018), for the lead to paste into `assets/palettes/ui.yaml`
The keys are proposed as `ui.<mode>.<token>`. `TestUITokensMatchPaletteYAML` needs a Go token for each one in `uikit.Tokens` in the same change.

Design:
- Red-green modes: good is sky blue, danger a soft coral, held darker than a lemon warning. The info letter is a pale cool grey, and the danger alert backing is plum.
- Tritan mode keeps the red-cyan axis: coral danger, teal good, and a pale peach warning set apart by lightness.
- Deutan and protan share values because they pass the check for both. Separate keys let them be tuned later.

```yaml
# OPT-018 colour-vision sets (Machado 2009 checked; see .cursor/handoffs/OPT-2-art.md).
ui.deutan.danger: "#EA9E94FF"
ui.deutan.warning: "#F8EA52FF"
ui.deutan.good: "#8EC4FBFF"
ui.deutan.mood_bad: "#EA9E94FF"
ui.deutan.mood_mid: "#F8EA52FF"
ui.deutan.mood_good: "#8EC4FBFF"
ui.deutan.letter_threat: "#EA9E94FF"
ui.deutan.letter_caution: "#F8EA52FF"
ui.deutan.letter_info: "#D0CDD8FF"
ui.deutan.letter_good: "#8EC4FBFF"
ui.deutan.alert_danger_bg: "#5A1A3EE8"
ui.deutan.alert_warn_bg: "#4A4010E8"
ui.protan.danger: "#EA9E94FF"
ui.protan.warning: "#F8EA52FF"
ui.protan.good: "#8EC4FBFF"
ui.protan.mood_bad: "#EA9E94FF"
ui.protan.mood_mid: "#F8EA52FF"
ui.protan.mood_good: "#8EC4FBFF"
ui.protan.letter_threat: "#EA9E94FF"
ui.protan.letter_caution: "#F8EA52FF"
ui.protan.letter_info: "#D0CDD8FF"
ui.protan.letter_good: "#8EC4FBFF"
ui.protan.alert_danger_bg: "#5A1A3EE8"
ui.protan.alert_warn_bg: "#4A4010E8"
ui.tritan.danger: "#FF9080FF"
ui.tritan.warning: "#FFDDB0FF"
ui.tritan.good: "#62D6C0FF"
ui.tritan.mood_bad: "#FF9080FF"
ui.tritan.mood_mid: "#FFDDB0FF"
ui.tritan.mood_good: "#62D6C0FF"
ui.tritan.letter_threat: "#FF9080FF"
ui.tritan.letter_caution: "#FFDDB0FF"
ui.tritan.letter_info: "#B8BCCCFF"
ui.tritan.letter_good: "#62D6C0FF"
ui.tritan.alert_danger_bg: "#5E1A2CE8"
ui.tritan.alert_warn_bg: "#4A3A14E8"
```

### Check
The check simulates each palette with the Machado, Oliveira & Fernandes (2009) matrices at severity 1.0, applied in linear sRGB. It measures:
- CIEDE2000 between every pair of tokens that share a screen;
- WCAG contrast on `ui.panel` over a white map, the worst case;
- CIE76 distance from `uiReferenceHexes`.

The contrast floors are the ones in `widgets_test.go`:
- 4.5:1 for danger, warning and good (text);
- 3:1 for the mood and letter marks;
- 4.5:1 for `ui.text` on the alert backings.

Every proposed token passes, and each is at least dE76 13 from the reference hexes.

Worst pair, in CIEDE2000, for the status, mood and letters groups under the target vision. A difference above about 10 is clearly visible side by side.

| set | normal | deutan | protan | tritan |
|---|---|---|---|---|
| current palette | 24.6 | **5.6** (warning/good) | **3.9** (mood mid/good) | **4.1** (danger/warning) |
| deutan/protan set | 20.2 | 16.5 | 14.1 | 14.5 |
| tritan set | 25.4 | 5.1 | 12.1 | 20.4 |

- The alert backings under the target vision: 22.3 (deutan), 31.9 (protan), 13.2 (tritan). The current pair scores 8.0 under deutan.
- Lowest contrasts: danger 4.64:1 in the red-green set and 4.52:1 in the tritan set. The tritan danger has the least margin to the 4.5 floor.
- The letter kinds also differ by seal shape (triangle, diamond, round, star), so colour is never the only cue.
- The full printout is in the scratchpad (`art/cvd_report.txt`, script `art/cvd.py`). If OPT-1 wants a guard test, port the matrices into `widgets_test.go`.

## Verification
- `make art FAMILIES=gearopt`: 6855 sprites checked, 0 errors. The pawn atlas was unchanged.
- `make art-check` with `PATH=/usr/local/bin:$PATH`: clean (validator, pawn atlas, `TestNoOrphanSprites`, `TestPlantRecipesHaveNoOutline`, audio manifest).
- `go test -race ./internal/adapters/inbound/ebiten/art/`: ok.
- I reviewed a contact sheet: the icons at 128/48/32/20 px beside `eat`, `equip`, `wear`, `order_haul`, `zone_dump`, `letter_info` and `dossier_gear`, and the cursors at 64/32 px on dark chrome, light ground, grass and soil.
  - All read at 20 px.
  - `gear_drop` and `zone_dump` are both down arrows. They differ by hue (tan against grey-brown) and object (a sack against a crate).

## Gaps
- No HUD in this change: art only. GEAR-2 and OPT-1 draw it.
- No in-game inspection yet: nothing draws these sprites. Check them in the real renderer when the readers land (skill completion gate).
- No checklist edits: the lead does them at integration. Suggested states:
  - OPT-028 art: done. The row stays open until the cursor is drawn.
  - OPT-018: values proposed, wiring open.
