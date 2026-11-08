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

## MOSS-SoundEffect generated audio

- **Model:** [MOSS-SoundEffect v2.0](https://huggingface.co/OpenMOSS-Team/MOSS-SoundEffect-v2.0) by the OpenMOSS team, Apache License 2.0. The model, its code and its weights are not in this repository; `cmd/sfxgen -backend moss` (the default since 2026-10-31) runs it locally through `assets/source/tools/moss/generate.py`.
- **What:** the ambience beds `assets/generated/audio/ambience/{day,night,rain}.ogg` (prompts in [`assets/source/audio/prompts.json`](assets/source/audio/prompts.json)) and every animal and mechanoid voice `assets/generated/audio/sfx/animals/*.ogg` (each species' call, attack, hurt and death, the 9 voice groups and the hive burst; not the `generic_*` placeholders, which are made in code), generated from our own prompts in [`assets/source/audio/animal_prompts.json`](assets/source/audio/animal_prompts.json) and reviewed by ear. Each take's prompt and seed are in [`assets/source/audio/elevenlabs/ledger.json`](assets/source/audio/elevenlabs/ledger.json) (`backend: moss`) and the sound list [`assets/metadata/audio/audio.json`](assets/metadata/audio/audio.json) (`source: moss`). Sounds made earlier with the ElevenLabs free tier were all replaced on 2026-11-06; the ledger keeps their takes as a record, and `make audio-check` rejects any non-commercial licence.
- **Not covered:** every other sound in the game is original and made in code: the effects and generic animal placeholders (`cmd/audiogen`) and the music (`cmd/audiogen -music`).
- **License:** the Apache-2.0 model licence puts no restriction on the generated audio.

## Noto Sans

- **Project:** [Noto Sans](https://github.com/notofonts/latin-greek-cyrillic) by the Noto Project Authors
- **License:** SIL Open Font License 1.1. The full text is in [`internal/adapters/inbound/ebiten/gfx/fontdata/OFL.txt`](internal/adapters/inbound/ebiten/gfx/fontdata/OFL.txt).
- **What:** `NotoSans-Regular.ttf` and `NotoSans-SemiBold.ttf` (hinted static builds from the notofonts release site), embedded unmodified as the interface font (NG-1).

## Docs tooling

- **Archify** ([tt-a1i/archify](https://github.com/tt-a1i/archify) 3.2.0, Apache-2.0) renders the diagrams in [`docs/architecture/`](docs/architecture/README.md) from their JSON specs (`make diagrams`).
- It is a global agent skill installed outside the repo. No code from it is vendored, and the game never links it.
- The committed HTML is its output. CI never runs it.

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
| `github.com/jfreymuth/pulse` | MIT | Linux and the BSDs (oto's PulseAudio driver) |
| `github.com/jfreymuth/vorbis` | MIT | all |
| `github.com/rivo/uniseg` | MIT | all |
| `github.com/srwiley/rasterx` | BSD-3-Clause | all (Ebitengine `text/v2` glyph rasterising) |
| `golang.org/x/image` | BSD-3-Clause | all |
| `golang.org/x/mod` | BSD-3-Clause | all |
| `golang.org/x/sync` | BSD-3-Clause | all |
| `golang.org/x/sys` | BSD-3-Clause | all |
| `golang.org/x/text` | BSD-3-Clause | all |
