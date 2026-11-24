// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

// planBullet is a bullet of dmg striking p (wiki: Damage Types, Bullet),
// its shares in dealing order; it draws nothing. The wiki's Bullet section
// says that when an inside part is hit the game will "apply the hit damage
// to the parent part (this is bonus damage)": so p takes dmg and, when p is
// inside and its parent is present and not p itself, the parent takes dmg
// too. Cut's bone exemption (cutBonuses) is Cut's alone: a bullet's bonus
// reaches the parent whether or not p is a bone. original tuning: the wiki
// gives no amount for the bonus beyond "the hit damage", so the share is the
// full post-armor dmg, and the parent is the plan's direct parent of p (an
// inside part's parent is outer in every plan).
func planBullet(b *Body, p Part, dmg int) shares {
	var sh shares
	sh.add(p, dmg)
	if inside(b, p) {
		if par := b.table()[p].Parent; par != p && !b.isMissing(par) {
			sh.add(par, dmg)
		}
	}
	return sh
}
