package game

// Card is a card value and rank. Zero means no card.
type Card int

const (
	CardNone Card = 0

	ReferenceCheck         Card = 1
	InformationalInterview Card = 2
	SalaryComparison       Card = 3
	PersonalDay            Card = 4
	Reassignment           Card = 5
	CounterOffer           Card = 6
	NonCompete             Card = 7
	DreamOffer             Card = 8
)

// Name returns the card's name as used in features/rules.md.
func (c Card) Name() string {
	switch c {
	case ReferenceCheck:
		return "Reference Check"
	case InformationalInterview:
		return "Informational Interview"
	case SalaryComparison:
		return "Salary Comparison"
	case PersonalDay:
		return "Personal Day"
	case Reassignment:
		return "Reassignment"
	case CounterOffer:
		return "Counter-Offer"
	case NonCompete:
		return "Non-Compete"
	case DreamOffer:
		return "Dream Offer"
	default:
		return "none"
	}
}

func (c Card) valid() bool {
	return c >= ReferenceCheck && c <= DreamOffer
}

// deckOrder is fixed so that a seed always produces the same shuffle.
var deckOrder = []Card{
	ReferenceCheck, ReferenceCheck, ReferenceCheck, ReferenceCheck, ReferenceCheck,
	InformationalInterview, InformationalInterview,
	SalaryComparison, SalaryComparison,
	PersonalDay, PersonalDay,
	Reassignment, Reassignment,
	CounterOffer,
	NonCompete,
	DreamOffer,
}

func newDeck() []Card {
	return append([]Card(nil), deckOrder...)
}

func contains(hand []Card, card Card) bool {
	for _, c := range hand {
		if c == card {
			return true
		}
	}
	return false
}

func removeCard(hand *[]Card, card Card) {
	cards := *hand
	for i, c := range cards {
		if c == card {
			cards = append(cards[:i], cards[i+1:]...)
			break
		}
	}
	if len(cards) == 0 {
		cards = nil
	}
	*hand = cards
}

func sumCards(cards []Card) int {
	total := 0
	for _, card := range cards {
		total += int(card)
	}
	return total
}

func targetFor(players int) int {
	switch players {
	case 2:
		return 7
	case 3:
		return 5
	case 4:
		return 4
	default:
		return 0
	}
}
