// Package options is the Options screen and the settings it edits: the
// tabs (graphics, audio, controls with key capture, the story tab's
// difficulty form, RAM notice), loading, applying and saving
// config.Settings, and the save slots the Load screen lists. Screen reaches
// the program through Shell; the root applies a changed setting to the
// renderer, window and sound (ports.Dispatcher / Host).
package options
