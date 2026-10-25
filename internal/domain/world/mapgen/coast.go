package mapgen

// coastStep is the "coast" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type coastStep struct{}

func (coastStep) Name() string { return "coast" }

func (coastStep) Run(c *Ctx) error { return nil }
