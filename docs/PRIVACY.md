## What data the game sends

Release builds send usage data to the project's website (`arxdsilva.com`), so I can see how many people play, on what, and how far they get. Builds you make yourself send nothing.

**Always sent:**

- a game started, a heartbeat every 5 minutes, and the game closed, with how long it ran
- the game version, operating system and CPU architecture
- a random install ID and a random ID for each launch

**Sent unless you turn it off** in Options › Gameplay › Share detailed usage data:

- your total RAM (rounded to whole GiB), screen size and graphics quality
- each colony you start or load: storyteller, difficulty, biome and map size
- every 10 minutes while playing: the in-game day, colonist count and frame rate

**Never sent:** colony, colonist or scenario names, file paths, map seeds or save files. The website uses your IP address in memory to limit abuse and doesn't save it, but the hosting provider's short-lived request logs do include it, as they do for any visit to the site.

**Checking for a new version:** each time it starts, a release build asks GitHub for the latest release, so it can tell you when there's a newer one to download. The request carries only the game version, and GitHub sees your IP address as it does for any visit. This check isn't part of the usage data, doesn't use the install ID, and the Options switch above doesn't affect it. The game never downloads or installs anything by itself.

The install ID is `telemetry.install_id` in `settings.json`, in your user config folder (`~/Library/Application Support/rimworld-go` on macOS). To have your data deleted, [open an issue](https://github.com/arxdsilva/rimworld-go/issues) quoting it. Doing so publicly links that ID to your GitHub account.

Back to the [README](../README.md).
