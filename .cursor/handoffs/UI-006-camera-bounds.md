# Camera bounds

- The viewport is the screen minus the top bar and the colonist bar/tab strip: `colonyViewport`.
- Furthest zoom-out comes from `minZoomFor`:
  - The whole map fits with an 8% margin.
  - The map must still fill at least 60% of the viewport's wider side (`minCover`), so black never dominates.
- Panning is limited by `clampCamera`/`clampAxis`:
  - Each map edge can be pulled at most 18% of the viewport inside it (`edgeSlack`). This lets edge tiles come out from under side panels; the gap draws black.
  - An axis where the map is smaller than the viewport is centred.
  - The first frame centres the camera on the map.
- Zoom is multiplicative (10% per step):
  - The wheel zooms around the cursor.
  - The +/- keys zoom around the viewport centre.
- Alert jumps are clamped too.
- Captures skip the camera input; they frame the camera themselves.
- New capture UIs: `-capture-ui zoomout` and `-capture-ui zoomedge`.
