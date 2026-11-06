package infocard

import (
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/domain/economy/item"
)

// The card's short descriptions are original prose, one per category (never
// a game's text); each is a literal at its T call so the catalog lists it.
// describe is the description for kind k ("" ok: an unknown name).
func describe(tr lang.Tr, k item.Kind, ok bool) string {
	if !ok {
		return tr.T("Nothing is known about this item.")
	}
	switch {
	case k.IsDrug():
		return tr.T("Taken for its effect rather than its nutrition. Doses wear off, and some leave the user wanting more.")
	case k.IsMedicine():
		return tr.T("Used by a doctor to tend wounds and illness. Better stock lowers the chance an injury turns bad.")
	}
	switch k.Category() {
	case "Food":
		return tr.T("Eaten to ease hunger. Meals made from several ingredients keep better and satisfy more than raw scraps.")
	case "Apparel":
		return tr.T("Worn on the body. What it covers, how many layers it takes and what it is made of decide how much it shields and how warm it keeps you.")
	case "Weapons":
		return tr.T("Carried in the hands. Damage, how fast it fires and how far it reaches set how it trades blows with armour and cover.")
	case "Materials":
		return tr.T("A raw or refined material. Builders and crafters turn it into walls, tools, clothing and more.")
	case "Manufactured":
		return tr.T("A made good. It is hauled, stored and installed rather than used up as raw material.")
	case "Corpses":
		return tr.T("What is left of the dead. It rots, draws flies and should be buried or burned before it spoils the room.")
	}
	return tr.T("A thing of some use. It can be stored on a shelf or carried by a colonist.")
}
