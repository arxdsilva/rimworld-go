# Playing

Install, system requirements, sound and controls. Back to the [README](../README.md).

## Install

Ready-made builds for Windows, macOS (Apple Silicon and Intel) and Linux are on the [Releases](https://github.com/arxdsilva/rimworld-go/releases) page: unpack the archive and run `rimworld-go`, keeping the `assets` folder next to it. The macOS build is unsigned, so open it the first time with right-click → Open.

## System requirements

**System requirements:** 64-bit Windows, macOS or Linux, 6 GB of RAM or more (8 GB recommended), and a GPU with Metal (macOS), DirectX 11 (Windows) or OpenGL (Linux) drivers.
- The game uses about 1.1 GB of resident memory, and its peak footprint is 2.2 GB on a 250×250 map and 3.4 GB on a grown 500×500 colony.
- At the first start it reads your RAM and cores and picks a graphics tier: High from 12 GB and 6 cores, Medium from 8 GB, Low below that. You can change it in Options › Graphics, where "Detect again" re-applies the tier.
- With less than 6 GB, the main menu warns once. Map sizes whose estimate exceeds 70% of your RAM say so in the new-game wizard; nothing is blocked.

## Sound and settings

Sound plays through your default audio device; `go run ./cmd/game -noaudio` (or `RIMWORLD_GO_NOAUDIO=1`) runs silently. On Linux, audio needs cgo and the ALSA headers (`sudo apt install libasound2-dev`). Options (volumes, fullscreen, graphics quality and detail, effects, autosave, edge scrolling, reduced motion) are saved to `settings.json` in your user config folder, next to the saves.

## Controls

Every key can be changed in Options › Controls (conflicts ask before
swapping; Reset to defaults puts them back).

| What | Keys |
|---|---|
| Camera | WASD or arrows pan (Shift: faster), PgDn / PgUp or the wheel zoom, middle-drag pans, edge scrolling in Options |
| Time | Space pause, 1 / 2 / 3 speeds, Home / End one speed up / down, or the speed buttons at the lower-right |
| Tabs | Tab Architect, F1 Work, F2 Schedule, F4 Animals, F5 Wildlife, F6 Research (F3, F7-F9 not yet) |
| View | F10 screenshot (saved under `screenshots` next to the settings), F11 hide the interface |
| Selection | click, Shift+click add/remove, drag a box, double-click for all of a kind on screen (in the colonist bar: centre on that colonist), `,` / `.` previous / next colonist, Numpad * next thing in the cell, Z search |
| Selected | R draft / undraft (all selected), F forbid / allow, V power on / off (a colonist flicks it), B H Y N J U M K I L O P the command buttons in order (right-click one for its options) |
| Orders | right-click (drafted: the group spreads around the tile; right-drag lines them up), Shift queues |
| Architect | Q / E rotate, or change the shape of drag tools (line, rectangle, oval, filled or empty); Ctrl squares the shape; C Cancel tool, X Deconstruct tool |
| Overlays | Shift+R roofs, Shift+P power grid, G room stats, Shift+M minimap on maps over 150 tiles, hold Alt for cell details |
| Amounts | Ctrl x10, Shift x100, Ctrl+Shift x1000 on bill +/- |
| Developer mode (Options › Gameplay) | 4 ultrafast, Numpad + one tick while paused, / debug actions, ` debug log, ' debug inspector |
