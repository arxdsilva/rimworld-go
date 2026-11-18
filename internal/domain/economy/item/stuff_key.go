package item

// stuffKeys are the material families' stable names in definition files
// (PLG-025: a recipe input's "stuff" list), one per bit from Woody up.
var stuffKeys = [...]string{"wood", "metal", "stone", "fabric", "leather"}

// AllStuff is every material family.
const AllStuff = Woody | Metallic | Stony | Fabric | Leathery

// Keys lists the families in c by stable name, in bit order (nil: none).
func (c StuffCategory) Keys() []string {
	var out []string
	for i, k := range stuffKeys {
		if c&(1<<i) != 0 {
			out = append(out, k)
		}
	}
	return out
}

// ParseStuff finds one material family by its key.
func ParseStuff(key string) (StuffCategory, bool) {
	for i, k := range stuffKeys {
		if k == key {
			return StuffCategory(1 << i), true
		}
	}
	return 0, false
}
