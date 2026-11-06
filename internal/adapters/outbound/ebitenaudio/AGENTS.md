# internal/adapters/outbound/ebitenaudio

## Purpose
Plays the game's sound list through Ebitengine's audio package. It is the concrete `Sound` the Ebitengine client talks to; `cmd/game` wires it in. Loads `assets/metadata/audio/audio.json` and the files it names under `assets/generated/audio`, then mixes one-shots, positional loops and music on the audio device at 48 kHz, honoring the player's volume settings.

## Owns
- `Engine`: `Has`, `Play`, `Loop`, `Music`, `SetVolumes`, `Update` (exactly the client's `Sound` port; it never imports the client).
- `New(assetsRoot)` and `LocateAudio()` (walk up from cwd to `assets/`).
- Loader (`load.go`): manifest first, WAVs as mono 48 kHz PCM, OGGs as encoded bytes; bad files are logged and skipped.
- Gain: cue vol x entry volume x `config.Gain(bus)` x `config.Gain(Master)`, 0 when `MuteUnfocused` and the window is unfocused.
- One-shots: random non-repeating variant, pan baked into stereo bytes, 24-voice cap (oldest closed), finished voices reaped in `Update`. OGG one-shots decode once to mono PCM (stereo is downmixed), are kept in an LRU capped at 24 MiB of PCM (`oggcache.go`), and play and pan like WAVs.
- Loops: per-emitter key, 0.8 s fades, WAV loops panned live (atomic pan read on the audio goroutine), OGG loops via `audio.NewInfiniteLoop`.
- Music: 2.5 s crossfades; looping tracks repeat, others play their files in turn with 40-150 s of silence between plays.
- The `player`/`backend` seam (`device.go`) that tests replace with fakes.

## Depends on
- `github.com/hajimehoshi/ebiten/v2/audio` (+ `audio/vorbis`), and `ebiten.IsFocused` for the focus mute.
- `pkg/audiomanifest`, `pkg/sfxsynth` (WAV reading), `internal/platform/config` (`Audio`, `Gain`, defaults), `pkg/randx` (`Mix64`, for the variant generator).

## Must not depend on
- Inbound adapters, including `internal/adapters/inbound/ebiten`, whose `Sound` interface this package satisfies structurally.
- Anything reachable from `cmd/simulate` must not import this package or `ebiten/v2/audio` (`arch_test.go` checks).

## Invariants
- `New` reads and validates the manifest before opening the audio context; a bad manifest returns an error with no device opened.
- Never panic on unknown ids, missing or corrupt files: log once per id and play nothing.
- Every method runs on the game loop; only `loopReader`'s pan is shared with the audio goroutine, through `sync/atomic`.
- Variant choice uses an internal splitmix, never the sim's RNG.

## Tests
Never call `audio.NewContext`: use `newEngine` with a fake backend and a stubbed `decode`. The real device test is behind the `audiodevice` build tag.
