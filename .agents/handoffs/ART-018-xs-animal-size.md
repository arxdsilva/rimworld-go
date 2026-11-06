# Task Handoff

Task ID: ART-018 (PLAY-010)
Status: complete

## Summary
XS animals were too small to read. The XS body length is now 0.44 tiles (was 0.35): rat, cinder rat, chinchilla, fennec fox, guinea pig, squirrel, Yorkshire terrier, chicken, duck, and the hive beetle (XS+, 0.506). The largest XS stays below the smallest S (S−, 0.51: cat, hare, snowhare, goose).

## Files
- `internal/domain/animal/animal.go` `sizeLength`; `TestXSBetweenOldAndS` (roster_test.go) pins the rule.
- `assets/source/blender/artlib/families/animal_roster.py` `SIZE_LENGTH`; the 10 species and their corpses re-rendered (822 sprites + normals).
- `TestOverlayAboveTallAnimals` uses the new rat size.

## Notes
The size only reaches the read model (`BodyLength` → marker and bar height), so the simulation and goldens are unaffected. Contact sheet reviewed by eye.
