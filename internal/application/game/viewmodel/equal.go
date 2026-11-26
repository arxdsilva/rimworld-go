// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package viewmodel

import "slices"

// EqualBuilding reports two views that read the same: every field
// equal, Bills element by element and MovingTo by the cell it points at.
// TestBuildingViewEqual proves every field is compared (a field added to
// BuildingView must be added here).
func EqualBuilding(a, b *BuildingView) bool {
	if (a.MovingTo == nil) != (b.MovingTo == nil) || (a.MovingTo != nil && *a.MovingTo != *b.MovingTo) || !a.Tank.Equal(b.Tank) {
		return false
	}
	return a.ID == b.ID && a.Kind == b.Kind && a.X == b.X && a.Y == b.Y &&
		a.Claimant == b.Claimant && a.Claimant2 == b.Claimant2 && a.Facing == b.Facing &&
		a.Floor == b.Floor && a.Deconstruct == b.Deconstruct && a.Stuff == b.Stuff && a.Quality == b.Quality &&
		a.SwapTo == b.SwapTo && a.SwapPct == b.SwapPct && a.Ruin == b.Ruin && a.Hostile == b.Hostile && a.OffGrid == b.OffGrid && a.Sparking == b.Sparking && a.Medical == b.Medical &&
		a.W == b.W && a.H == b.H && a.HasSpot == b.HasSpot && a.SpotX == b.SpotX && a.SpotY == b.SpotY &&
		a.Bench == b.Bench && slices.Equal(a.Bills, b.Bills) &&
		a.DryKind == b.DryKind && a.DryQty == b.DryQty && a.DryPct == b.DryPct &&
		a.Fuel == b.Fuel && a.FuelPct == b.FuelPct && a.FuelLogs == b.FuelLogs && a.FuelCapLogs == b.FuelCapLogs && a.Burning == b.Burning && a.Refuelable == b.Refuelable && a.AutoRefuel == b.AutoRefuel &&
		a.HPPct == b.HPPct && a.DeconWork == b.DeconWork && a.Catch == b.Catch &&
		a.Occupant == b.Occupant && a.Epitaph == b.Epitaph && a.Headstone == b.Headstone &&
		a.TroughFill == b.TroughFill && a.TroughCap == b.TroughCap && a.TroughFilter == b.TroughFilter &&
		a.Sealed == b.Sealed && a.OpenReason == b.OpenReason &&
		a.PowerRole == b.PowerRole && a.PowerNet == b.PowerNet && a.Powered == b.Powered &&
		a.PowerOn == b.PowerOn && a.PowerWantOn == b.PowerWantOn &&
		a.PowerUse == b.PowerUse && a.PowerOut == b.PowerOut &&
		a.StoredWh == b.StoredWh && a.CapWh == b.CapWh && a.NoPowerReason == b.NoPowerReason &&
		a.LinkX == b.LinkX && a.LinkY == b.LinkY && a.LinkFromX == b.LinkFromX && a.LinkFromY == b.LinkFromY &&
		a.HasLink == b.HasLink && a.PowerRelink == b.PowerRelink && a.Glow == b.Glow && a.GlowRadius == b.GlowRadius &&
		a.ThermoTarget == b.ThermoTarget && a.ThermoActive == b.ThermoActive && a.PowerMax == b.PowerMax &&
		a.AimDir == b.AimDir && a.ShotAge == b.ShotAge &&
		a.Comfort == b.Comfort && a.RestEffect == b.RestEffect && a.Beauty == b.Beauty &&
		a.Facilities == b.Facilities && a.Forbidden == b.Forbidden && a.HoldOpen == b.HoldOpen && a.HasTarget == b.HasTarget &&
		a.HoldFire == b.HoldFire && a.Barrel == b.Barrel && a.Shells == b.Shells && a.TargetX == b.TargetX &&
		a.TargetY == b.TargetY && a.WorkSpeed == b.WorkSpeed && slices.Equal(a.Linked, b.Linked) &&
		a.Group == b.Group && a.Stored == b.Stored && a.Capacity == b.Capacity && a.Open == b.Open && a.HeatRate == b.HeatRate && a.ResearchSpeed == b.ResearchSpeed && a.Analyzer == b.Analyzer && a.AX == b.AX && a.CleanOff == b.CleanOff
}

// EqualFrame reports two frame views that read the same (Haulers
// element by element). TestFrameViewEqual proves every field is compared.
func EqualFrame(a, b *FrameView) bool {
	return a.ID == b.ID && a.Kind == b.Kind && a.X == b.X && a.Y == b.Y && a.Wood == b.Wood &&
		a.Need == b.Need && a.WorkLeft == b.WorkLeft && a.Facing == b.Facing && a.Floor == b.Floor &&
		a.Stuff == b.Stuff && a.W == b.W && a.H == b.H && a.Moving == b.Moving && a.MovingFrom == b.MovingFrom &&
		a.WorkTotal == b.WorkTotal && a.Incoming == b.Incoming && a.InStock == b.InStock &&
		a.Reserved == b.Reserved && a.Builder == b.Builder && slices.Equal(a.Haulers, b.Haulers) &&
		a.Status == b.Status && a.Comp == b.Comp && a.CompNeed == b.CompNeed && a.CompIncoming == b.CompIncoming && a.AX == b.AX
}

// EqualItem is a == b field by field (Ingredients keeps ItemView from
// ==); TestSameItemViewSeesEveryField fails on a field left out.
func EqualItem(a, b ItemView) bool {
	return a.ID == b.ID && a.Kind == b.Kind && a.Qty == b.Qty && a.X == b.X && a.Y == b.Y && a.Carried == b.Carried &&
		a.FreshDays == b.FreshDays && a.Rotten == b.Rotten && a.RotRate == b.RotRate && a.Dried == b.Dried && a.Of == b.Of &&
		a.OfName == b.OfName && a.Animal == b.Animal && a.Wreck == b.Wreck && a.Catch == b.Catch && a.Forbidden == b.Forbidden &&
		a.Unclaimed == b.Unclaimed && a.HaulMarked == b.HaulMarked && a.PoisonPermille == b.PoisonPermille &&
		a.Quality == b.Quality && a.Stuff == b.Stuff && a.HP == b.HP && a.MaxHP == b.MaxHP && a.Tainted == b.Tainted &&
		a.Urgent == b.Urgent && slices.Equal(a.Ingredients, b.Ingredients) && a.Meat == b.Meat && a.AX == b.AX && a.Store == b.Store && a.Slot == b.Slot
}
