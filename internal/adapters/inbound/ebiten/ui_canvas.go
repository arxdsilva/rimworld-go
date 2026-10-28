package ebiten

// The UI canvas (PLAY-012 B.2): every screen-layout draw (panels,
// buttons, text, icons, HUD, menus, tooltips) goes through these helpers.
// They take logical coordinates and draw at physical resolution, x the UI
// scale (curUIScale.s). Signatures mirror the vector package, so a raw
// vector.FillRect(...) becomes uiFillRect(...) with the same arguments.
// Stroke widths scale too. At s = 1 every call is exactly the raw one, so
// captures stay byte-identical.
//
// World-space drawing (anything the camera positions) stays raw, in
// physical pixels; see TestUIDrawsThroughCanvas for the functions allowed to,
// and for the rule that world-anchored labels use worldText / worldIcon.
// With a reduced map render scale (world_scale.go) a canvas draw aimed at
// the map's offscreen is queued and drawn on the screen after the map is
// composited, so labels and icons stay at native resolution.
