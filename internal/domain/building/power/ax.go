package power

import "rimworld-go/internal/domain/building/construction"

// axSpec is the electrical definition of an AX-0 contract kind (inert
// while the kind is Planned; the zero Spec for anything else). wiki: Wall
// lamp (30 W), Flood light (100 W), Waterproof conduit, Chemfuel powered
// generator (1000 W), Watermill generator (1100 W), Deep drill (200 W),
// Ground-penetrating scanner (700 W), Moisture pump (150 W), Electric
// crematorium (250 W), Foam turret (80 W), Rocketswarm launcher (150 W,
// [MEM]), Tube/Flatscreen/Megascreen television (200/330/450 W),
// Long-range mineral scanner (700 W). A bench idles at a tenth (our rule);
// the drill, the scanners and the screens are benches here (worked or
// watched); the pump draws all the time. The foam popper, the stasis
// casket, the pods and the ship's other parts need no power; the ship
// reactor (wiki: Ship reactor) gives 1000 W at all times, started or not.
func axSpec(k construction.Kind) Spec {
	switch k {
	case construction.WallLamp:
		return consumer(ClassLamp, 30, 30)
	case construction.FloodLight:
		return consumer(ClassLamp, 100, 100)
	case construction.WaterproofConduit:
		return Spec{Role: Transmitter}
	case construction.ChemfuelGenerator:
		return producer(1000)
	case construction.Watermill:
		return producer(1100)
	case construction.DeepDrill:
		return bench(200)
	case construction.GroundScanner, construction.MineralScanner:
		return bench(700)
	case construction.MoisturePump:
		return consumer(ClassBench, 150, 150)
	case construction.ElectricCrematorium:
		return bench(250)
	case construction.FoamTurret:
		return consumer(ClassDefense, 80, 80)
	case construction.RocketBattery:
		return consumer(ClassDefense, 150, 150)
	case construction.TubeTV:
		return bench(200)
	case construction.FlatscreenTV:
		return bench(330)
	case construction.BigScreenTV:
		return bench(450)
	case construction.ShipReactor:
		return producer(1000)
	}
	return Spec{}
}
