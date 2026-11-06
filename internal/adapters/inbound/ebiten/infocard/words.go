package infocard

import "rimworld-go/internal/domain/economy/item"

// The card's short descriptions are original prose, one per category (never
// a game's text).
const (
	wordsApparel  = "Worn on the body. What it covers, how many layers it takes and what it is made of decide how much it shields and how warm it keeps you."
	wordsWeapon   = "Carried in the hands. Damage, how fast it fires and how far it reaches set how it trades blows with armour and cover."
	wordsFood     = "Eaten to ease hunger. Meals made from several ingredients keep better and satisfy more than raw scraps."
	wordsDrug     = "Taken for its effect rather than its nutrition. Doses wear off, and some leave the user wanting more."
	wordsMedicine = "Used by a doctor to tend wounds and illness. Better stock lowers the chance an injury turns bad."
	wordsMaterial = "A raw or refined material. Builders and crafters turn it into walls, tools, clothing and more."
	wordsMade     = "A made good. It is hauled, stored and installed rather than used up as raw material."
	wordsCorpse   = "What is left of the dead. It rots, draws flies and should be buried or burned before it spoils the room."
	wordsOther    = "A thing of some use. It can be stored on a shelf or carried by a colonist."
	wordsUnknown  = "Nothing is known about this item."
	wordsGone     = "This item is no longer there."
)

// describe is the description for kind k ("" ok: an unknown name).
func describe(k item.Kind, ok bool) string {
	if !ok {
		return wordsUnknown
	}
	switch {
	case k.IsDrug():
		return wordsDrug
	case k.IsMedicine():
		return wordsMedicine
	}
	switch k.Category() {
	case "Food":
		return wordsFood
	case "Apparel":
		return wordsApparel
	case "Weapons":
		return wordsWeapon
	case "Materials":
		return wordsMaterial
	case "Manufactured":
		return wordsMade
	case "Corpses":
		return wordsCorpse
	}
	return wordsOther
}
