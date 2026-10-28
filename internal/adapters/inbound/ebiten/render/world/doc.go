// Package world is the renderer's world pipeline: the draw order of one
// frame (ground, floors, zones, objects by row, marks and labels), the
// zoom tiers with the chunk and whole-map bakes, the frame's chunk lists,
// the terrain, plant, item and building art keys, the stack counts, chat
// bubbles and damage marks. Pipeline (the order) composes the layer
// packages (sprite, light, env, creature, overlay, cue) through its parts
// Bakes, Objects and Labels, reads one *render.View per frame and asks the
// renderer only through Source (deps.go); it never reaches the Renderer
// (SOLID_GO.md §7).
package world
