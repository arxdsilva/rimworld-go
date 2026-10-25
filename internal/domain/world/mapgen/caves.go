package mapgen

// cavesStep is the "caves" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type cavesStep struct{}

func (cavesStep) Name() string { return "caves" }

func (cavesStep) Run(c *Ctx) error { return nil }
