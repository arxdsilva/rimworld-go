package mapgen

// materialStep is the "material" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type materialStep struct{}

func (materialStep) Name() string { return "material" }

func (materialStep) Run(c *Ctx) error { return nil }
