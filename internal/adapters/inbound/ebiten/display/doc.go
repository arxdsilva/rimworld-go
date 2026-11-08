// Package display holds the client's calls on the Ebitengine window and
// monitor that the composition root (internal/compose/game) makes: the
// fullscreen and vsync switches, the window size presets and borderless
// mode (OPT-029), the OS cursor (OPT-028), the monitor's size and scale,
// and the window setup before RunGame. internal/compose/game imports no
// third-party module, so these live here.
package display
