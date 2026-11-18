// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package construction

// plannedA8 is A8's kinds not live yet (the escape ship; AX-0 contract): delete a
// case when its kind works end to end. This file is A8's, and so are any
// predicates it adds here for its own kinds. AX-8: every ship part is live
// (game/ship).
func plannedA8(Kind) bool { return false }
