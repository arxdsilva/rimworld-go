---
name: rimworld-go-art-assets
description: Make or review rimworld-go sprites, icons, UI chrome, palettes, pivots, atlases and asset-pipeline tooling. Use when player-visible art is in scope.
---

# Art direction and asset pipeline

Spec: `.agents/specs/ART_DIRECTION_AND_ASSET_PIPELINE.md` (ART). Read by section; skip for headless work with no visual output. Applies to world sprites, terrain, buildings, plants, pawns, animals, items; UI chrome, icons, portraits, emblems; palettes, pivots, metadata, atlases, validators; renderer presentation; plugin asset packs; AI-assisted concept art. Pixels, texture handles, camera and animation stay out of `internal/domain`.

## Load

- Root `AGENTS.md`; ART §1 Definition of done, §2 Principles, §35 Acceptance.
- Clean room: `.agents/specs/CLEAN_ROOM.md` (never trace, extract, recolor or reconstruct RimWorld art, nor use it as image-to-image input).
- Then only the family sections:

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

- `assets/AGENTS.md` when it exists; the matching backlog row (`.agents/backlog/`); the named screen in MAIN_UI when chrome or icons belong to one; neighbouring assets, templates and palettes of the family.

## Flow

requirement -> original brief -> shared template + palette -> source art -> metadata -> validate / export / atlas -> renderer consumes generated assets + `RenderSnapshot`. Domain owns footprint, IDs and state, not pixels. Animation timing never decides simulation outcomes. Visual variation keys off world seed, tile, EntityID and category; visual RNG never feeds the simulation.

## Steps

1. Identify family, gameplay purpose, template, logical footprint.
2. Confirm the Wave 0 contracts (scale, camera, pivots, outlines, palettes, light, naming, metadata) exist; freeze them before bulk production.
3. Honour the ART §34 prototype gate: no bulk production before the vertical-slice scene exists.
4. Search existing templates, palettes and IDs first.
5. Author source under `assets/source/`; never hand-patch `assets/generated/` when source exists.
6. Write sidecar metadata (footprint, pivot, attachments, tint regions, export).
7. Validate with `assettool` / `contentlint` where they exist.
8. Inspect in the real renderer at gameplay zoom, on light and dark terrain.
9. Tick the matching ART wave checkboxes and any vanilla/UI backlog row (root `AGENTS.md` checkbox rule).
10. Delete every asset, alias, metadata file and loader the change leaves unused (ART §25); `make art-check` reports no orphans.
11. Plants and grass: no contour, `outline_width=0` (ART §9).
12. Pawns are limbless (round head, body capsule; attachments `head`, `hold`, `back`; S/E/N rendered, west mirrored; no frames). Camera, lean and outline values live in `artlib/contracts.py` (ART §5–6, ART-025); never hard-code them.
13. Append a section to the family ledger (`.agents/templates/HANDOFF.md`) with the visual impact.

## Task packet extras

On top of `.agents/templates/TASK_PACKET.md`: asset family, template, logical footprint, required variants, material regions / palette, attachments / pivots, existing references.

## Parallel ownership

After contracts freeze, split by family (furniture, plants, weapons, icons). Parallel agents never redefine pawn proportions, tile scale, palette, camera, pivots or building perspective. One integration pass owns validation, contact sheets and style-drift rejection.

## Completion gate

Original (clean room); readable at gameplay zoom; canonical scale, pivot, palette, light, perspective; metadata valid; validators pass; inspected in-game; editable source committed; checkboxes updated; ledger section appended.
