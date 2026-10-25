package mapgen

// wetnessStep is the "wetness" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type wetnessStep struct{}

func (wetnessStep) Name() string { return "wetness" }

func (wetnessStep) Run(c *Ctx) error { return nil }
