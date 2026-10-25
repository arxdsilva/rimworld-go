package mapgen

// roadsStep is the "roads" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type roadsStep struct{}

func (roadsStep) Name() string { return "roads" }

func (roadsStep) Run(c *Ctx) error { return nil }
