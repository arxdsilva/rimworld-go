// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package visit

import (
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/entity"
)

// Cargo records which physical carrier owns a share of a caravan's
// virtual sale stock. Stock is the sole authority on sale quantities.
type Cargo struct {
	Owner  entity.EntityID
	Lines  []trade.Line
	Silver int
}

// AssignCargo partitions sale stock among living carriers in sorted order.
func AssignCargo(st trade.Stock, owners []entity.EntityID) []Cargo {
	if len(owners) == 0 {
		return nil
	}
	owners = slices.Clone(owners)
	slices.Sort(owners)
	owners = slices.Compact(owners)
	out := make([]Cargo, len(owners))
	for i, id := range owners {
		out[i].Owner = id
	}
	for i, ln := range st.Lines {
		ln.Lots = slices.Clone(ln.Lots)
		out[i%len(out)].Lines = append(out[i%len(out)].Lines, ln)
	}
	out[0].Silver = st.Silver
	return out
}

// SettleCargo applies a validated physical trade to the carriers' exact
// manifest. Missing or removed stock aborts without changing any owner.
func SettleCargo(cargo []Cargo, trader entity.EntityID, res trade.Result) bool {
	traderIndex := slices.IndexFunc(cargo, func(c Cargo) bool { return c.Owner == trader })
	if traderIndex < 0 {
		return false
	}
	next := slices.Clone(cargo)
	for i := range next {
		next[i].Lines = trade.CloneLines(cargo[i].Lines)
	}
	for _, bought := range res.Buy {
		for _, lot := range bought.Lots {
			take := trade.Line{Kind: bought.Kind, Building: bought.Building,
				Qty: lot.Stack.Qty, Lots: []trade.Lot{lot}}
			found := false
			for i := range next {
				lines, ok := trade.TakeLine(next[i].Lines, take)
				if ok {
					next[i].Lines, found = lines, true
					break
				}
			}
			if !found {
				return false
			}
		}
		left := bought.OrdinaryQty()
		for i := range next {
			for left > 0 {
				j := slices.IndexFunc(next[i].Lines, func(l trade.Line) bool {
					return l.Goods() == bought.Goods() && l.OrdinaryQty() > 0
				})
				if j < 0 {
					break
				}
				n := min(left, next[i].Lines[j].OrdinaryQty())
				lines, ok := trade.TakeLine(next[i].Lines, trade.Line{
					Kind: bought.Kind, Building: bought.Building, Qty: n,
				})
				if !ok {
					return false
				}
				next[i].Lines = lines
				left -= n
			}
			if left == 0 {
				break
			}
		}
		if left != 0 {
			return false
		}
	}
	for _, sold := range res.Sell {
		if sold.Kind == item.Silver {
			next[traderIndex].Silver += sold.Qty
		} else {
			next[traderIndex].Lines = trade.AddLine(next[traderIndex].Lines, sold)
		}
	}
	if res.Silver < 0 {
		next[traderIndex].Silver -= res.Silver
	} else if res.Silver > 0 {
		left := res.Silver
		for i := range next {
			n := min(left, next[i].Silver)
			next[i].Silver -= n
			left -= n
			if left == 0 {
				break
			}
		}
		if left > 0 {
			return false
		}
	}
	copy(cargo, next)
	return true
}

// LoseCargo removes a lost carrier's exact quantities and physical
// item lots from the stock before the game drops those items on the map.
func LoseCargo(st *trade.Stock, cargo []Cargo, owner entity.EntityID) (remaining []Cargo, lines []trade.Line, silver int) {
	for i := range cargo {
		if cargo[i].Owner != owner {
			continue
		}
		remainingStock := trade.CloneLines(st.Lines)
		for _, l := range cargo[i].Lines {
			var ok bool
			remainingStock, ok = trade.TakeLine(remainingStock, l)
			if !ok {
				return cargo, nil, 0
			}
		}
		lines, silver = trade.CloneLines(cargo[i].Lines), cargo[i].Silver
		st.Lines = remainingStock
		st.Silver -= silver
		return slices.Delete(cargo, i, i+1), lines, silver
	}
	return cargo, nil, 0
}

// CargoOwners lists real carriers for an active trade group. The trader
// always owns its own portion; departed or missing pack members own none.
func CargoOwners(group Group, trader entity.EntityID, alive func(entity.EntityID) bool) []entity.EntityID {
	owners := []entity.EntityID{trader}
	for _, m := range group.Members {
		if m.Role == RolePackAnimal && !m.Departed && alive(m.ID) {
			owners = append(owners, m.ID)
		}
	}
	return owners
}

// CargoLossGroup returns the caravan owning an active pack member. An
// already departed carrier must never drop its manifest a second time.
func CargoLossGroup(groups []Group, id entity.EntityID) uint64 {
	for _, g := range groups {
		if g.Kind != KindCaravan {
			continue
		}
		for _, m := range g.Members {
			if m.ID == id && m.Role == RolePackAnimal && !m.Departed {
				return g.ID
			}
		}
	}
	return 0
}

// SmallStock caps goods carried by a small visiting trader.
func SmallStock(stock *trade.Stock) {
	stock.Silver = min(stock.Silver, 500)
	for i := range stock.Lines {
		stock.Lines[i].Qty = min(stock.Lines[i].Qty, 3)
	}
}

// CloneCargo detaches the complete physical manifest for a checkpoint.
func CloneCargo(cargo []Cargo) []Cargo {
	out := slices.Clone(cargo)
	for i := range out {
		out[i].Lines = trade.CloneLines(cargo[i].Lines)
	}
	return out
}
