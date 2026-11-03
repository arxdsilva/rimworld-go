# Third-party notices

## spritefusion-sprite-normals

- **Project:** [spritefusion-sprite-normals](https://github.com/Hugo-Dz/spritefusion-sprite-normals) by Hugo Duprez
- **License:** MIT, Copyright (c) 2026 Hugo Duprez. The full text is in [`third_party/spritefusion-sprite-normals/LICENSE`](third_party/spritefusion-sprite-normals/LICENSE).
- **Version studied:** commit `d14b06726cabab09c68a403ce2e684a52efbfe4c`
- **What we ported:** its relief and normal-map generation algorithm (`src/relief.rs`, `field.rs`, `distance.rs`, `color.rs`, `components.rs` and `settings.rs`), translated from Rust to Go:
  - 8-connected islands
  - Oklab lightness detail
  - an exact Euclidean distance transform shaped into a soft bevel
  - Gaussian and masked Gaussian blurs
  - Sobel slopes to normals
- **Where:** [`pkg/spritenormals`](pkg/spritenormals). [`cmd/normalgen`](cmd/normalgen) runs it over our sprites (`make normals`), and the game lights the result with a moving sun and moon (ART spec §6.1).
- **Differences:**
  - Our sprites are drawn at twice their display resolution, so the shape radius, shape softness and detail scale are doubled.
  - There is no CLI, no WebAssembly build and no sprite-sheet frame splitting; each PNG is one frame.

## ElevenLabs generated audio

- **What:** the ambience beds `assets/generated/audio/ambience/{day,night,rain}.ogg` were generated with [ElevenLabs](https://elevenlabs.io) from the prompts in [`assets/source/audio/prompts.json`](assets/source/audio/prompts.json). The animal voices `assets/generated/audio/sfx/animals/*.ogg` (every species' call, attack, hurt and death, the 9 voice groups and the hive burst; not the `generic_*` placeholders and not the four mechanoids' voices, which come from MOSS below) were generated from [`assets/source/audio/animal_prompts.json`](assets/source/audio/animal_prompts.json) and reviewed by ear. Each take is recorded in [`assets/source/audio/elevenlabs/ledger.json`](assets/source/audio/elevenlabs/ledger.json), and each file's prompt, credits and license are in the sound list [`assets/metadata/audio/audio.json`](assets/metadata/audio/audio.json).
- **License:** generated on the ElevenLabs free tier. That output is for non-commercial use and requires attribution to ElevenLabs. Regenerate these files on a paid plan, or replace them, before any commercial release.
- **Not covered:** every other sound in the game is original and made in code, including the generic animal placeholders (`cmd/audiogen -animals`): the effects (`cmd/audiogen`) and the music (`cmd/audiogen -music`). The free tier has no music API.

## MOSS-SoundEffect generated audio

- **Model:** [MOSS-SoundEffect v2.0](https://huggingface.co/OpenMOSS-Team/MOSS-SoundEffect-v2.0) by the OpenMOSS team, Apache License 2.0. The model, its code and its weights are not in this repository; `cmd/sfxgen -backend moss` (the default since 2026-10-31) runs it locally through `cmd/sfxgen/moss/generate.py`.
- **What:** the mechanoid voices `assets/generated/audio/sfx/animals/{harrow,spindle,bulwark,quill}_{call,attack,hurt,die}*.ogg`, generated from our own prompts. Each take's prompt and seed are in [`assets/source/audio/elevenlabs/ledger.json`](assets/source/audio/elevenlabs/ledger.json) (`backend: moss`) and the sound list [`assets/metadata/audio/audio.json`](assets/metadata/audio/audio.json) (`source: moss`).
- **License:** the Apache-2.0 model licence puts no restriction on the generated audio.

## Noto Sans

- **Project:** [Noto Sans](https://github.com/notofonts/latin-greek-cyrillic) by the Noto Project Authors
- **License:** SIL Open Font License 1.1. The full text is in [`internal/adapters/inbound/ebiten/gfx/fontdata/OFL.txt`](internal/adapters/inbound/ebiten/gfx/fontdata/OFL.txt).
- **What:** `NotoSans-Regular.ttf` and `NotoSans-SemiBold.ttf` (hinted static builds from the notofonts release site), embedded unmodified as the interface font (NG-1).

## Go modules

The game's binary links these modules. Their license files, with any NOTICE file, are in [`third_party/go-modules`](third_party/go-modules), refreshed by `make licenses`. A test in `internal/arch` fails when a module in `go.mod` is missing from this table.

| Module | License | Linked on |
| --- | --- | --- |
| Go standard library | BSD-3-Clause, [`third_party/go-modules/go/LICENSE`](third_party/go-modules/go/LICENSE) | all |
| `github.com/cli/browser` | BSD-2-Clause | all |
| `github.com/ebitengine/gomobile` | BSD-3-Clause | not linked |
| `github.com/ebitengine/hideconsole` | Apache-2.0 | Windows |
| `github.com/ebitengine/oto/v3` | Apache-2.0 | all |
| `github.com/ebitengine/purego` | Apache-2.0 | all |
| `github.com/go-text/typesetting` | BSD-3-Clause and Unlicense; `harfbuzz` MIT | all |
| `github.com/google/uuid` | BSD-3-Clause | all |
| `github.com/hajimehoshi/ebiten/v2` | Apache-2.0 | all |
| `github.com/jezek/xgb` | BSD-3-Clause | Linux |
| `github.com/jfreymuth/oggvorbis` | MIT | all |
| `github.com/jfreymuth/vorbis` | MIT | all |
| `github.com/rivo/uniseg` | MIT | all |
| `golang.org/x/image` | BSD-3-Clause | all |
| `golang.org/x/mod` | BSD-3-Clause | all |
| `golang.org/x/sync` | BSD-3-Clause | all |
| `golang.org/x/sys` | BSD-3-Clause | all |
| `golang.org/x/text` | BSD-3-Clause | all |
