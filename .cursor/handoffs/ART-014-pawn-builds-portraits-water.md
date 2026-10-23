# ART-014 Pawn builds, storyteller busts, water and NG-1 world art

Part of NG-1 (`.cursor/NG1_NEW_GAME_PLAN.md` §7; decisions D5, D9, D12). Commits: `ece5b5e8` (pawns, atlas), `f7bebeb1` (busts, water, icons, items), `42cb19d2`, `24836829` (portrait key).

## What changed

### Six pawn builds (`assets/source/blender/artlib/families/pawn.py`)
- One parametrised rig with a `BUILDS` table gives male and female thin, average and heavy bodies: `m_thin, m_avg, m_heavy, f_thin, f_avg, f_heavy`. The same table drives every clothing layer (shirt, pants, vest), all frames and all four facings.
- Keys are `pawn/<layer>_<build>_<facing>_<frame>`. The old `pawn/body_...` keys remain and alias the male average build, so legacy looks (`look.Legacy`) render exactly as before.
- **Hair styles:** short, crop and bun (legacy) plus long, braid, shaved, curls and tail. **Beards:** short, full and moustache (males, about 45 % in the generator). A grey-hair palette entry joins the tinted hair set.

### Atlas packing (`cmd/atlasgen`)
The pawn family (about 15,000 sprites, each with a tint mask and a normal map) is packed into **six texture pages** of at most 4096x4096 (`assets/generated/atlas/pawn_0..5.png`, `_mask`, `_n`) with `assets/metadata/atlas/pawn.yaml` as the index (page, rect, trim, origin). The art catalog serves atlas keys, and the validator checks the atlas (`atlasgen -family pawn -check`, part of `make art-check`). `make atlas` repacks. At run time the atlas pages are decoded once and then dropped from memory (ENG-001).

**Committed artifact versus intermediates:** the committed artifact is the packed atlas plus its metadata. Per-sprite renders and per-sprite metadata of the five non-legacy builds (`*_m_thin_*`, `*_m_heavy_*`, `*_f_thin_*`, `*_f_avg_*`, `*_f_heavy_*`) are build intermediates and are **git-ignored** (`.gitignore`). To rebuild them: `make art` then `make atlas`.

### Storyteller busts (`families/portrait.py`)
Original busts of Orla Vane, Bram Holloway and Kestrel Quay, 512x640 with 120x150 thumbnails, transparent, under the same stage and lighting contract as the other families. Catalog keys are `ui_portrait/steady`, `ui_portrait/hearth` and `ui_portrait/gambler` (with `_thumb`); the storyteller profile's `Portrait` field holds the key. Silhouettes, palettes and props differ clearly from any other game's storytellers (descriptions in ART spec §17.1).

### Water, terrain and sites (`families/terrain.py`, `rock.py`)
- Terrain `shallow_water`, `deep_water`, `mud` and `track`, four variants each (`terrain/<type>_v0..3`), 16-mask edges for mud over soil and shallow over deep, and shore-foam masks. The in-game ripple and glint shader is described in SIM-033.
- Cave mouths in four facings (`rock/cave_mouth_*`) and buried-machinery ore (`rock/ore_machinery_*`, a linked set like the other ores).
- Drop-pod effects: `effect/pod_fall` and `effect/pod_dust`; the pod shell building.

### Items, buildings and icons
- Items: components and industrial medicine, each in small, medium and large stacks.
- Furniture: tool cabinet and hospital bed (four facings).
- UI icons (`ui_icon/...`): four scenario icons plus `scenario_custom`; eleven skill icons; minor and major passion; a trait icon; faction emblems `faction_<settlers|clans|drifters|freebooters|swarm|player>`; settlement icons `settlement_neolithic` and `settlement_industrial` (tinted by the faction accent at run time); the minimap icon.
- The globe is not sprite art: it is vertex-coloured from `assets/palettes/world.yaml`, with hill shading, rivers, dashed roads and the faction-tinted settlement icons on top.

## Verification
- Recorded at commit time; the docs pass re-ran nothing. `make art-check` (validator plus `atlasgen -family pawn -check`) and `TestGeneratedArtCoversRenderer` are the gates; the storyteller portrait key has a test that follows the art catalog key (`24836829`).
- Disk and render budget from the plan: about 12.6k new sprites rendered per build in parallel; the atlas keeps the shipped size to six pages.

## Known limitations
- **Male and female builds read weakly at 1x zoom**, and the thin/average/heavy difference is small there; a follow-up should widen the silhouettes or add a distinguishing clothing cut.
- **Beards are nearly invisible when a colonist faces north.**
- Only the pawn family is atlas-packed; other families still load per sprite.
- No distinct head shapes; the six builds change the body only.
- Original-art rule: nothing here traces or references RimWorld art; reviewers should keep checking busts and icons against it.
