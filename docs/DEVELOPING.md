# Developing

Building, testing, profiling and refreshing media and sound. Back to the [README](../README.md). Read [`AGENTS.md`](../AGENTS.md) before changing code.

## Build and test

To build from source you need Go 1.26.10 or later (the version in [`go.mod`](../go.mod)).

```sh
go run ./cmd/game   # start the game
make build          # build every command into bin/
make test           # unit tests with the race detector and coverage
make test-journeys  # the player-journey suite: fake input through the real game loop (also runs in CI on every push)
make simulate       # headless simulation run for quick checks
```

## Developer flags

Flags for `cmd/game`:

| Flag | What it does |
|---|---|
| `-scenario <name>` | Starts a set piece. For example `-scenario grown -width 250 -height 250` is the large-map benchmark colony. |
| `-fpsprobe N` | Runs the colony for N seconds of real frames at `-capture-speed` and `-capture-zoom`, and logs fps, 1% low, update/draw times and the CPU work per frame. |
| `-cpuprofile`, `-memprofile`, `-trace` | Write Go profiles. |
| `-vsync=false` | Turns vsync off. `make fps` uses it. |
| `-capture-ui-scale 1.5` (or 2) | Takes a `-capture` at that UI scale, to review scaled layouts. |
| `-graphics low` (or `medium`, `high`) | Applies that graphics tier for one run, captures and `-fpsprobe` included, without saving it. |
| `-render-scale 0.75` (or `0.5`) | Draws the map at that share of the screen's pixels for the run. |
| `-noaudio` | Runs silently (or set `RIMWORLD_GO_NOAUDIO=1`). |

Related commands and notes:

- `make fps` is the repeatable probe: the grown 500×500 colony at 3× speed, zoom 0.1 and 0.8, in a floating display-sized window with vsync off (`-vsync=false`), logged to `bin/fps-<tag>.log`. A 30 Hz or occluded display caps the fps number, so compare the `work` line there.
- The probe's report ends with the Go heap and the art memory (sprites freed, prewarm, tint bakes).
- `go run ./cmd/benchmark` prints per-tick percentiles for the grown colony at 120, 250 and 500 tiles a side, with a `views` row for the read model.
- In the game, Options › Graphics › Performance overlay shows fps, 1% low, frame, update and draw time, the sim rate and memory in a small box by the resources.

## Refreshing the README media

- `make media`: records the gallery as short animated WebPs through the game's `-capture` flag.
- `make clips`: captures frame sequences and packs them into animated WebP with `scripts/frames_to_webp.py`. Needs python3 and Pillow.
- `make art` re-renders the sprites in Blender, then `make normals` writes their normal maps.
- To let an AI agent drive a live Blender (Blender MCP), see [`tools/blender-mcp`](../tools/blender-mcp/README.md). `.mcp.json` registers the server for Claude Code; other tools add it from their own settings.

## Sound

- `make audio` renders the sound effects from code (`cmd/audiogen`, recipes in `cmd/audiogen/recipes.go`) and updates the sound list `assets/metadata/audio/audio.json`; `make audio-check` validates it.
- `make music` composes the music in code (`cmd/audiogen/music.go`, `pkg/musicsynth`): the main theme, a calm colony track and a raid loop, encoded to Ogg with ffmpeg.
- Ambience and animal voices are made with the local MOSS-SoundEffect model (`cmd/sfxgen`); the day, night and rain beds are imported with `scripts/audio_import.sh`. ElevenLabs remains a paid option through its MCP server, registered in `.mcp.json`. Put your key in your shell profile as `export ELEVEN_LABS_API_KEY=...` (the config maps it to the server's `ELEVENLABS_API_KEY`), then restart your editor so it inherits it. Prompts and the credit budget are in `assets/source/audio/prompts.json`; import an approved take with `scripts/audio_import.sh <raw> <file> <credits> "<license>"`.

## Releases

Pushing a `v*` tag builds and publishes a new release ([`release.yml`](../.github/workflows/release.yml)).
