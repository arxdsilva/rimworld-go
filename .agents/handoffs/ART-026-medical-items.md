# ART-026: medical item art, vitals monitor, care icons

Art for MED-05 and MED-06, in the ART-025 look (shear 0.35, 6 px dark
outline, muted palette). All designs are original.

## Assets
- **Item sprites** (`families/item.py` `build_med()`):
  - `item/nanomed_kit`: one sprite, not s/m/l.
  - `item/servo_arm`, `servo_leg`, `optic_implant`, `cochlear_implant`, `spinal_brace`, `pump_heart`, `synth_stomach`, `valve_heart`.
  - `item/kidney`, `lung`, `heart`, `liver`: each in a squat glass jar with a teal seal.
- **Vitals monitor**: `furniture/vitals_monitor_{south,east,north,west}` plus glow layers `furniture/vitals_monitor_glow_<facing>`. The screen trace and top lamp use the emissive `med_teal_glow`.
- **UI icons**:
  - the 13 items above and `vitals_monitor`;
  - disc glyphs `care_none`, `care_nomeds`, `care_herbal`, `care_industrial`, `care_best`, now drawn on the Health tab's care buttons.
- **Palette**: a "MED items" block in `materials_ng1.yaml` (`med_teal`, `organ_*`, `specimen_fluid`) and an emissive `med_teal_glow`. Additions only.
- **Renderer fix**: `itemSprite`'s fallback now maps spaces to underscores, so "servo arm" finds `item/servo_arm`. Peg leg, glass eye and other existing items now find their sprites too (`TestItemSpriteFallbackUsesUnderscores`).

## Verify
- Re-rendering changed 0 existing files. normalgen wrote 21 new normal maps. The validator reports 0 errors.
- `TestNoOrphanSprites` and `TestGeneratedArtCoversRenderer` pass.

## Follow-up
Facing east or west, the vitals monitor reads as a tall slab.
