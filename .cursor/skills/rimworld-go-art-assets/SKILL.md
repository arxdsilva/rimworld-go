---
name: rimworld-go-art-assets
description: Create, change, or review rimworld-go sprites, icons, UI chrome, palettes, pivots, atlases, renderer presentation, plugin assets, and the asset pipeline using the canonical art-direction spec. Use whenever player-visible art or asset-pipeline tooling is in scope.
---

# Art Direction & Asset Pipeline

Canonical spec: `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md`.

Do not paste that file into prompts. Read it by section. Do not load it for headless simulation-only work with no visual output.

## When this skill applies

Use for:

- world sprites, terrain, buildings, plants, pawns, animals, items, weapons, apparel;
- UI chrome, icons, overlays, portraits, faction emblems;
- palettes, pivots, tint masks, metadata, atlases, validators, exporters;
- renderer presentation that chooses how those assets appear;
- plugin/mod asset packs;
- AI-assisted concept art for this game.

Do not use as a license to put sprites, texture handles, or animation clocks in `internal/domain`.

## Required context

Always:

- root `AGENTS.md`;
- this skill;
- `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md` §1 (definition of done), §2 (principles), §35 (acceptance).

Then only the sections that match the asset family:

| Family | Sections |
|---|---|
| Scale, pivots, camera, light | §3–§6 |
| Palette / materials | §7, §33 |
| Terrain / rocks / plants | §8–§9 |
| Pawns / animals | §10–§11 |
| Buildings / walls / production | §12–§14 |
| Weapons / apparel / items / food | §15–§16 |
| UI / factions / overlays | §17–§18 |
| Repo layout / toolchain / templates | §19–§23 |
| Metadata / pipeline / plugins | §24–§26 |
| Animation, LOD, visual RNG | §27–§28 |
| Tests / QA / task contract | §29–§30 |
| Parallel ownership / waves / prototype gate | §31–§34 |

Also load:

- `assets/AGENTS.md` when that package exists;
- matching item(s) in `.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md` (rendering/content) or `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` (UI chrome/icons);
- matching named screen in `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md` when chrome/icons belong to a player-facing screen;
- existing templates, palettes, and neighboring assets in the same family.

## Clean-room

Never trace, extract, recolor, vectorize, or closely reconstruct proprietary RimWorld sprites, textures, icons, UI art, or other copyrighted assets. Do not use extracted RimWorld assets as image-to-image inputs.

Reproduce readability and design principles, not proprietary artwork.

## Architecture

```text
gameplay requirement
-> original brief
-> shared template + palette
-> source art
-> metadata
-> validate / export / atlas
-> renderer consumes generated assets + RenderSnapshot
```

- Domain owns logical footprint, IDs, and simulation state — not pixels.
- Renderer/UI adapters own sprites, atlases, camera, interpolation, and zoom LOD.
- Animation timing never determines simulation outcomes.
- Visual variation keys off world seed, tile, EntityID, and category; visual RNG must not feed authoritative simulation.

## Workflow

1. Identify asset family, gameplay purpose, template, and logical footprint.
2. Confirm Wave 0 contracts (scale, camera, pivots, outlines, palettes, light, naming, metadata) exist; if not, freeze those before bulk production.
3. Honor the §34 prototype gate: do not bulk-produce a family until the vertical-slice scene exists.
4. Search existing templates, palettes, and IDs before inventing new ones.
5. Author original source under `assets/source/`; never hand-patch `assets/generated/` when source exists.
6. Write sidecar metadata (footprint, pivot, attachments, tint regions, export).
7. Validate with `assettool` / `contentlint` when those tools exist.
8. Inspect in the real renderer at normal gameplay zoom, on light and dark terrain.
9. Update matching ART wave checkboxes in `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md` (`[x]` / `[-]` / `[!]`).
10. If the work also maps to a vanilla or UI checklist item, update that checkbox in the same change.
11. Delete every asset, alias, metadata file and loader the change leaves unused (spec §25 housekeeping); `make art-check` must report no orphans.
12. Plants and grass: no contour (`outline_width=0`, spec §9).
13. Pawns are limbless (round head, body capsule; attachments `head`, `hold`, `back`; S/E/N rendered, west mirrored; no frames). Camera, lean and outline values live in `artlib/contracts.py` (spec §5-6, ART-025); never hard-code them.
14. Write a concise handoff including visual/art impact.

## Task packet extras

In addition to the standard packet, every art task states:

```text
asset family:
template:
logical footprint:
required variants:
material regions / palette:
attachments / pivots:
allowed files:
forbidden files:
existing references:
```

## Parallel ownership

After shared contracts are frozen, split by family (furniture vs plants vs weapons vs icons). Do not let parallel agents redefine pawn proportions, tile scale, palette, camera, pivots, or building perspective. One integration pass owns validation, contact sheets, and style-drift rejection.

## Completion gate

An art/visual change is done only when:

- original (clean-room);
- readable at normal gameplay zoom;
- canonical scale, pivot, palette, light, and perspective;
- metadata valid;
- validator/tests pass;
- inspected in-game, not only in an editor;
- editable source committed where applicable;
- ART wave (and any matching gameplay/UI checklist) checkbox updated;
- handoff written.
