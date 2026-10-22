package pawngen

// Original name pools (NG-1 §3.8): common real-world names from many
// cultures plus invented ones; nothing taken from another game's lists.
// Order matters for determinism: append only.

var femaleNames = []string{
	"Ada", "Adaeze", "Aiko", "Alma", "Amara", "Anika", "Aoife", "Asha", "Astrid", "Averine",
	"Beatrix", "Bianca", "Brisa", "Bronwen", "Carmen", "Celeste", "Dalia", "Daria", "Delphine", "Dessa",
	"Dora", "Edda", "Elif", "Elsie", "Esme", "Fenna", "Fenwyn", "Fiora", "Freya", "Gaia",
	"Greta", "Halla", "Hana", "Hedda", "Ilka", "Ilse", "Ilvy", "Imani", "Ines", "Ingrid",
	"Isolde", "Jana", "Jessamy", "Juno", "Kaia", "Kalinda", "Kestra", "Keziah", "Kirra", "Lale",
	"Leda", "Liesel", "Lina", "Liora", "Lorna", "Lucia", "Lumi", "Maeve", "Mara", "Maren",
	"Marisol", "Merrin", "Mina", "Mira", "Nadia", "Nessa", "Neve", "Nia", "Noor", "Odile",
	"Olena", "Oona", "Paloma", "Pella", "Petra", "Priya", "Quilla", "Rhea", "Rilla", "Rosa",
	"Runa", "Sabine", "Saoirse", "Selma", "Senna", "Signe", "Sola", "Sorrel", "Talia", "Tamsin",
	"Tavi", "Tova", "Una", "Valka", "Veda", "Vera", "Vessa", "Wren", "Xiomara", "Yara",
	"Ysolde", "Zara", "Zelie", "Zofia",
}

var maleNames = []string{
	"Aarav", "Abel", "Adem", "Alaric", "Amadou", "Anders", "Arlo", "Arvid", "Bastian", "Benedek",
	"Bertil", "Brannoc", "Cadwin", "Caius", "Callum", "Cormac", "Corvin", "Dallan", "Dario", "Desmond",
	"Dmitri", "Dorran", "Eamon", "Elias", "Emeka", "Enzo", "Eskil", "Evrin", "Ewan", "Fabian",
	"Felix", "Fenrick", "Finnegan", "Florin", "Garro", "Gideon", "Goran", "Halvar", "Hamish", "Haskel",
	"Hector", "Hugo", "Ibrahim", "Idris", "Ilya", "Isak", "Ithel", "Jago", "Jarek", "Joaquin",
	"Jonas", "Jory", "Kaito", "Kasimir", "Kellan", "Kenji", "Kofi", "Korvin", "Lars", "Leandro",
	"Lev", "Linus", "Lorcan", "Ludo", "Magnus", "Malik", "Matteo", "Merek", "Milo", "Mosi",
	"Niall", "Nikolai", "Oisin", "Omar", "Orrin", "Osric", "Ottar", "Pavel", "Piet", "Pim",
	"Quentin", "Rafael", "Ravi", "Remy", "Rolf", "Rune", "Santiago", "Silas", "Soren", "Stellan",
	"Tamas", "Teodor", "Thabo", "Tobias", "Toller", "Ugo", "Ulric", "Varro", "Vasco", "Viggo",
	"Wendell", "Wystan", "Yannick", "Yusuf", "Zane", "Zoltan",
}

var surnames = []string{
	"Abernathy", "Achterberg", "Adeyemi", "Albescu", "Alder", "Amsel", "Ashdown", "Ashgrove", "Bakker", "Balogun",
	"Barrow", "Beckett", "Bellweather", "Birch", "Blackwood", "Bramley", "Brennan", "Brightwater", "Calder", "Carrow",
	"Castellan", "Cheng", "Cindermoor", "Collis", "Corrigan", "Dagher", "Dalca", "Delacroix", "Dunholt", "Dunmore",
	"Eastwick", "Ekwueme", "Elling", "Emberly", "Esposito", "Falk", "Farhollow", "Farrow", "Fenwick", "Ferreira",
	"Flint", "Fosse", "Gallo", "Garrick", "Greaves", "Gunnarsen", "Haddad", "Hallowmere", "Halloran", "Harrow",
	"Hartigan", "Holm", "Ibarra", "Ingram", "Jaramillo", "Jessop", "Juniper", "Kaminski", "Kesterton", "Kettleby",
	"Kovac", "Krall", "Lachance", "Lark", "Lindqvist", "Lowell", "Lund", "Maddox", "Marchetti", "Mendel",
	"Merriweather", "Moreau", "Mossgard", "Nakamura", "Navarro", "Nkemelu", "Northcott", "Novak", "Oakes", "Okafor",
	"Olsen", "Ortega", "Pallister", "Pemberton", "Quinlan", "Rainsford", "Ramsay", "Ravel", "Reyes", "Rook",
	"Rowntree", "Salazar", "Sandoval", "Selwyn", "Sixsmith", "Soto", "Stonecrop", "Strand", "Szabo", "Tanaka",
	"Tarrant", "Thistlewood", "Thorne", "Torvik", "Underhill", "Valdez", "Vance", "Varga", "Wainwright", "Wexley",
	"Whitlock", "Winter", "Wyse", "Yilmaz", "Yoshida", "Zamora", "Zielinski",
}

var nicknames = []string{
	"Ace", "Ash", "Badger", "Biscuit", "Bolt", "Bramble", "Brick", "Buttons", "Cinder", "Clover",
	"Cog", "Crow", "Dash", "Dusty", "Spark", "Fig", "Flicker", "Gadget", "Gravel", "Hatch",
	"Hex", "Jinx", "Kettle", "Knuckles", "Lantern", "Lucky", "Mags", "Moth", "Mouse", "Nettle",
	"Nib", "Patch", "Pepper", "Pip", "Quill", "Rivet", "Rusty", "Sable", "Scout", "Socks",
	"Sparrow", "Sprocket", "Tack", "Thimble", "Tinker", "Tumble", "Twig", "Whistle", "Wick", "Wisp",
	"Yarrow", "Zigzag",
}

// blocklist are names and titles from other games (and this game's own
// storyteller and studio names) that generated text must never contain.
// Matching is case-insensitive on whole entries.
var blocklist = []string{
	// storytellers and studio of the game this project is inspired by
	"Tynan", "Ludeon", "Cassandra", "Phoebe", "Randy", "Igor",
	// well-known backstory titles from that game
	"Vatgrown soldier", "Urbworld urchin", "Glitterworld surgeon", "Midworld student",
	"Medical student", "Cave child", "Tribe child", "Farm kid", "Ship janitor",
	"Space marine", "Bounty hunter", "Mercenary", "Pyromaniac", "Coder",
	"Abandoned child", "Colonial child", "Escape pod scavenger", "Cult child",
	"Vatgrown", "Glitterworld", "Urbworld", "Midworld", "Rimworld",
	// our own storyteller names must not be reused for a colonist
	"Orla", "Vane", "Bram", "Holloway", "Kestrel", "Quay",
}

// Blocklist returns the names and titles generated text must never use
// (for cmd/contentlint).
func Blocklist() []string { return append([]string(nil), blocklist...) }
