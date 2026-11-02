package power

import "rimworld-go/internal/domain/building/construction"

// axWant is the AX-0 contract kinds' electrical definitions (ax.go), merged
// into TestSpecOf's table.
var axWant = map[construction.Kind]Spec{
	construction.WallLamp:            {Role: Consumer, Class: ClassLamp, Draw: 30, Idle: 30, Toggle: true},
	construction.FloodLight:          {Role: Consumer, Class: ClassLamp, Draw: 100, Idle: 100, Toggle: true},
	construction.WaterproofConduit:   {Role: Transmitter},
	construction.ChemfuelGenerator:   {Role: Producer, Peak: 1000, Toggle: true},
	construction.Watermill:           {Role: Producer, Peak: 1100, Toggle: true},
	construction.DeepDrill:           {Role: Consumer, Class: ClassBench, Draw: 200, Idle: 20, Toggle: true},
	construction.GroundScanner:       {Role: Consumer, Class: ClassBench, Draw: 700, Idle: 70, Toggle: true},
	construction.MineralScanner:      {Role: Consumer, Class: ClassBench, Draw: 700, Idle: 70, Toggle: true},
	construction.MoisturePump:        {Role: Consumer, Class: ClassBench, Draw: 150, Idle: 150, Toggle: true},
	construction.ElectricCrematorium: {Role: Consumer, Class: ClassBench, Draw: 250, Idle: 25, Toggle: true},
	construction.FoamTurret:          {Role: Consumer, Class: ClassDefense, Draw: 80, Idle: 80, Toggle: true},
	construction.RocketBattery:       {Role: Consumer, Class: ClassDefense, Draw: 150, Idle: 150, Toggle: true},
	construction.TubeTV:              {Role: Consumer, Class: ClassBench, Draw: 200, Idle: 20, Toggle: true},
	construction.FlatscreenTV:        {Role: Consumer, Class: ClassBench, Draw: 330, Idle: 33, Toggle: true},
	construction.BigScreenTV:         {Role: Consumer, Class: ClassBench, Draw: 450, Idle: 45, Toggle: true},
}
