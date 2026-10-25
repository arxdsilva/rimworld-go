package mapgen

// terrainStep is the "terrain" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type terrainStep struct{}

func (terrainStep) Name() string { return "terrain" }

func (terrainStep) Run(c *Ctx) error { return nil }
