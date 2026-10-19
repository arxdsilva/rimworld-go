# Soil hover panel

- Hovering any map tile shows a panel in the bottom-right corner, just above the colonist bar.
- The panel lists:
  - Soil label.
  - Fertility %.
  - Farmability:
    - Not farmable.
    - Impassable, mine to clear.
    - Crops grow X% faster or slower.
  - Room, plant and stack lines, as before.
- Read model: `game.Soils() []SoilView` (Name, Label, Fertility, Farmable, Walkable), indexed by the `WorldView.Terrain` code.
- Soils:

  | Soil | Fertility | Farmable |
  |---|---|---|
  | Rich soil | 140% | yes |
  | Soil | 100% | yes |
  | Gravel | 70% | yes |
  | Rough stone | 0% | no |
  | Rock wall | 0% | no, impassable |

- Capture: `-capture-ui hover`.
- Test: `internal/application/game/soil_test.go`.
