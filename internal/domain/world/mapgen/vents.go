package mapgen

// ventsStep is the "vents" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type ventsStep struct{}

func (ventsStep) Name() string { return "vents" }

func (ventsStep) Run(c *Ctx) error { return nil }
