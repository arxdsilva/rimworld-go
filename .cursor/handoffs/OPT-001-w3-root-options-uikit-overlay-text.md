# OPT-001 Wave 3: root, options, uikit, render/overlay text through the catalog

- Options screen (rows, tabs, Controls, Habits, Story, Load, dialogs), the root's zone commands, pause and boot screens, difficulty dialog, speed and weather labels, uikit built-in words and overlay zone/casket names go through `T`/`Tf`.
- `options.Host` embeds `ports.Texts`; `optionsHost` forwards to the shell. Option row builders take a `lang.Tr` (`OptionRows(tab, tr...)`, `cardRows(tr...)`, `habitGroups(tr...)`), variadic so existing tests keep their calls.
- uikit has no new field or method: `Access.Tr` carries the catalog (set in `syncKit`); `DrawButton`, `DrawTooSmall` take an optional trailing `lang.Tr`.
- Overlay: `Host.Tr` (set from `sceneSet.tr`); `ZoneLabelText`, `CasketTitle(For)` take an optional `lang.Tr`.
- Open for other agents: `hud_compat.go` should pass `h.a.shell` to `drawWeatherLabelRight`; inspector `CasketTitle` and `uikit.DrawButton` calls, wizard `uikit.DrawTooSmall` calls should pass their Shell.
- Proposed exemption: `screen_boot.go:"rimworld-go"` (product name).
- No state-hash impact.
