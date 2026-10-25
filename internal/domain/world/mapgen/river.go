package mapgen

// riverStep is the "river" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type riverStep struct{}

func (riverStep) Name() string { return "river" }

func (riverStep) Run(c *Ctx) error { return nil }
