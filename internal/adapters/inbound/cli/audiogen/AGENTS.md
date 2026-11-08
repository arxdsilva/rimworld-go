# internal/adapters/inbound/cli/audiogen

## Purpose
The `audiogen` command's flags, dispatch and logic (moved out of `cmd/audiogen`, 2026-11, GAR): renders the procedural sound effects and music, checks and registers the sound list. `cmd/audiogen/main.go` only calls `Run`.

## Owns
`Run`, the recipe tables (`recipes.go`, `*_recipes.go`), the music scores (`music.go`), the manifest check (`check_animals.go`) and `register.go`.

## Depends on
`pkg/sfxsynth`, `pkg/musicsynth`, `pkg/audiomanifest`; `github.com/hajimehoshi/ebiten/v2/audio/vorbis` to decode Ogg in `-check`. External tools: `ffmpeg` (`-music`) and `oggenc` (`-animals`).

## Contracts
Output is byte-identical on every machine; `-check` and `TestAudioFilesMatchManifest` pin the files against `assets/metadata/audio/audio.json`. `make audio`, `make music`, `make audio-check` and `scripts/audio_import.sh` keep their flags.

## Tests
`go test -race ./internal/adapters/inbound/cli/audiogen`. Tests read `../../../../../assets`.

## Notes
Recipes are package-level tables (no `init()`); a new recipe is added to the table of its family file.
