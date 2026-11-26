# Historical writer fixtures (v1)

These two frozen files are output of the real historical Game.Save writer, not modified modern Gob records.

- historical-v1-labor-11.sav: e8dc2bcf6 (11 labor columns, no explicit priority table). SHA256 b9afb17845d89e79277e3547aa276db8b426ff69993222f3150b1af82d7a8861.
- historical-v1-labor-12.sav: 432641002 (12 labor columns, explicit Prios). SHA256 6acbb72ee699a66b01a65312449c571a200b8d536add32d92e34c543bf015409.

Both used seed 42, a 20x20 game, one pawn named Historic at (2,2), all work disabled then Haul, Clean and Craft enabled. The 12-column writer additionally sets Handle=1, Haul=0, Clean=2, Craft=4, Grow=2 through its native priority setter. Save name: historical-labor.

Generation used the historical Save implementation in an isolated module. Only the module's minimum Go toolchain version was adjusted; the writer and its data representation were unchanged.
