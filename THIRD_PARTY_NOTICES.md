# Third-party notices

## spritefusion-sprite-normals

- **Project:** [spritefusion-sprite-normals](https://github.com/Hugo-Dz/spritefusion-sprite-normals) by Hugo Duprez
- **License:** MIT, Copyright (c) 2026 Hugo Duprez. The full text is in [`third_party/spritefusion-sprite-normals/LICENSE`](third_party/spritefusion-sprite-normals/LICENSE).
- **Version studied:** commit `d14b06726cabab09c68a403ce2e684a52efbfe4c`
- **What we ported:** its relief and normal-map generation algorithm (`src/relief.rs`, `field.rs`, `distance.rs`, `color.rs`, `components.rs` and `settings.rs`), translated from Rust to Go:
  - 8-connected islands
  - Oklab lightness detail
  - an exact Euclidean distance transform shaped into a soft bevel
  - Gaussian and masked Gaussian blurs
  - Sobel slopes to normals
- **Where:** [`pkg/spritenormals`](pkg/spritenormals). [`cmd/normalgen`](cmd/normalgen) runs it over our sprites (`make normals`), and the game lights the result with a moving sun and moon (ART spec §6.1).
- **Differences:**
  - Our sprites are drawn at twice their display resolution, so the shape radius, shape softness and detail scale are doubled.
  - There is no CLI, no WebAssembly build and no sprite-sheet frame splitting; each PNG is one frame.
