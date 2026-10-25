package mapgen

// rocksStep is the "rocks" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type rocksStep struct{}

func (rocksStep) Name() string { return "rocks" }

func (rocksStep) Run(c *Ctx) error { return nil }
