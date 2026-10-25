package mapgen

// vegetationStep is the "vegetation" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type vegetationStep struct{}

func (vegetationStep) Name() string { return "vegetation" }

func (vegetationStep) Run(c *Ctx) error { return nil }
