package mapgen

// rockmaskStep is the "rockmask" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type rockmaskStep struct{}

func (rockmaskStep) Name() string { return "rockmask" }

func (rockmaskStep) Run(c *Ctx) error { return nil }
