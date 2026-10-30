package game

import (
	"errors"
	"fmt"
	"math/rand"
)

// Phase is the coarse state of a match.
type Phase string

const (
	PhasePlaying   Phase = "playing"
	PhaseMatchOver Phase = "match_over"
)

// Errors returned by NewMatch and Apply.
var (
	ErrInvalidPlayerCount = errors.New("player count must be 2 to 4")
	ErrInvalidSeat        = errors.New("invalid seat")
	ErrStaleVersion       = errors.New("stale state version")
	ErrMatchOver          = errors.New("match is over")
	ErrNotYourTurn        = errors.New("not your turn")
	ErrCardNotInHand      = errors.New("card is not in hand")
	ErrMustPlayNonCompete = errors.New("must play Non-Compete")
	ErrInvalidTarget      = errors.New("invalid target")
	ErrInvalidGuess       = errors.New("invalid Reference Check guess")
	ErrDeckEmpty          = errors.New("draw deck is empty")
)

// State is a complete match. Every field is serializable, including the draw
// deck order and each player's private snapshots.
type State struct {
	Seed            int64         `json:"seed"`
	Version         int           `json:"version"`
	PlayerCount     int           `json:"playerCount"`
	Target          int           `json:"target"`
	Phase           Phase         `json:"phase"`
	Round           int           `json:"round"`
	Turn            int           `json:"turn"`
	ActiveSeat      int           `json:"activeSeat"`
	Deck            []Card        `json:"deck,omitempty"`
	Secret          Card          `json:"secret,omitempty"`
	SecretDrawn     bool          `json:"secretDrawn,omitempty"`
	FaceUp          []Card        `json:"faceUp,omitempty"`
	Players         []PlayerState `json:"players"`
	History         []Event       `json:"history,omitempty"`
	LastRound       *RoundResult  `json:"lastRound,omitempty"`
	MatchWinnerSeat int           `json:"matchWinnerSeat,omitempty"`
}

// PlayerState is one seat. Hand and Snapshots are private; everything else is
// public.
type PlayerState struct {
	Seat       int        `json:"seat"`
	Name       string     `json:"name"`
	Hand       []Card     `json:"hand,omitempty"`
	Discard    []Card     `json:"discard,omitempty"`
	Eliminated bool       `json:"eliminated"`
	Protected  bool       `json:"protected"`
	Callbacks  int        `json:"callbacks"`
	Snapshots  []Snapshot `json:"snapshots,omitempty"`
}

// Snapshot is private information one player acquired. It never updates when
// the target's hand changes.
type Snapshot struct {
	Round      int    `json:"round"`
	Turn       int    `json:"turn"`
	TargetSeat int    `json:"targetSeat"`
	Cards      []Card `json:"cards"`
}

// RoundResult records the public outcome of the most recent round.
type RoundResult struct {
	Round   int            `json:"round"`
	Winners []int          `json:"winners"`
	Secret  Card           `json:"secret,omitempty"`
	Hands   []RevealedHand `json:"hands,omitempty"`
}

// RevealedHand is a remaining hand shown at deck exhaustion.
type RevealedHand struct {
	Seat  int    `json:"seat"`
	Cards []Card `json:"cards"`
}

// Event is one entry in the public move history. It never contains a private
// snapshot or another player's current hand.
type Event struct {
	Turn        int    `json:"turn"`
	Round       int    `json:"round"`
	Kind        string `json:"kind"`
	Seat        int    `json:"seat,omitempty"`
	Card        Card   `json:"card,omitempty"`
	Target      int    `json:"target,omitempty"`
	Guess       Card   `json:"guess,omitempty"`
	Result      string `json:"result,omitempty"`
	Winners     []int  `json:"winners,omitempty"`
	Secret      Card   `json:"secret,omitempty"`
	MatchWinner int    `json:"matchWinner,omitempty"`
}

// NewMatch creates a match with one seat per name. Names may be empty; the
// caller is responsible for any fallback display text.
func NewMatch(seed int64, names []string) (State, error) {
	if len(names) < 2 || len(names) > 4 {
		return State{}, ErrInvalidPlayerCount
	}

	state := State{
		Seed:        seed,
		Version:     1,
		PlayerCount: len(names),
		Target:      targetFor(len(names)),
		Phase:       PhasePlaying,
		Players:     make([]PlayerState, len(names)),
	}
	for i, name := range names {
		state.Players[i] = PlayerState{Seat: i + 1, Name: name}
	}
	if err := state.startRound(0); err != nil {
		return State{}, err
	}
	return state, nil
}

func roundRNG(seed int64, round int) *rand.Rand {
	return rand.New(rand.NewSource(seed + int64(round)*0x9E3779B1))
}

// startRound shuffles a new round and begins the first turn. A zero startSeat
// chooses a random seat, which is used for the first round.
func (s *State) startRound(startSeat int) error {
	s.Round++
	s.Secret = CardNone
	s.SecretDrawn = false
	s.FaceUp = nil
	for i := range s.Players {
		player := &s.Players[i]
		player.Hand = nil
		player.Discard = nil
		player.Eliminated = false
		player.Protected = false
		player.Snapshots = nil
	}

	rng := roundRNG(s.Seed, s.Round)
	deck := newDeck()
	rng.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	s.Secret = deck[0]
	next := 1
	if s.PlayerCount == 2 {
		s.FaceUp = append([]Card(nil), deck[1:4]...)
		next = 4
	}
	for i := range s.Players {
		s.Players[i].Hand = []Card{deck[next]}
		next++
	}
	s.Deck = append([]Card(nil), deck[next:]...)

	if startSeat == 0 {
		startSeat = 1 + rng.Intn(s.PlayerCount)
	}
	s.ActiveSeat = startSeat
	return s.beginTurn()
}

// beginTurn clears protection that expired at this player's turn, draws one
// card, and starts the turn clock.
func (s *State) beginTurn() error {
	player := s.player(s.ActiveSeat)
	if player == nil || player.Eliminated {
		return ErrInvalidSeat
	}
	player.Protected = false
	card, ok := s.draw()
	if !ok {
		return ErrDeckEmpty
	}
	player.Hand = append(player.Hand, card)
	s.Turn++
	return nil
}

func (s *State) draw() (Card, bool) {
	if len(s.Deck) == 0 {
		return CardNone, false
	}
	card := s.Deck[0]
	s.Deck = s.Deck[1:]
	if len(s.Deck) == 0 {
		s.Deck = nil
	}
	return card, true
}

func (s *State) drawSecret() (Card, bool) {
	if s.Secret == CardNone || s.SecretDrawn {
		return CardNone, false
	}
	card := s.Secret
	s.Secret = CardNone
	s.SecretDrawn = true
	return card, true
}

func (s *State) player(seat int) *PlayerState {
	if seat < 1 || seat > len(s.Players) {
		return nil
	}
	return &s.Players[seat-1]
}

func (s *State) allOthersProtected(seat int) bool {
	others := 0
	for _, player := range s.Players {
		if player.Seat == seat || player.Eliminated {
			continue
		}
		others++
		if !player.Protected {
			return false
		}
	}
	return others > 0
}

func (s *State) remainingSeats() []int {
	var seats []int
	for _, player := range s.Players {
		if !player.Eliminated {
			seats = append(seats, player.Seat)
		}
	}
	return seats
}

func (s *State) eliminate(player *PlayerState) {
	if player.Eliminated {
		return
	}
	player.Eliminated = true
	player.Protected = false
	player.Discard = append(player.Discard, player.Hand...)
	player.Hand = nil
}

func (s *State) addSnapshot(owner *PlayerState, targetSeat int, cards []Card) {
	owner.Snapshots = append(owner.Snapshots, Snapshot{
		Round:      s.Round,
		Turn:       s.Turn,
		TargetSeat: targetSeat,
		Cards:      append([]Card(nil), cards...),
	})
}

// cloneCards copies a card slice while preserving nil, so a clone compares
// equal to its original after a JSON round trip (omitempty decodes empty
// slices as nil).
func cloneCards(cards []Card) []Card {
	if cards == nil {
		return nil
	}
	return append([]Card{}, cards...)
}

func cloneInts(values []int) []int {
	if values == nil {
		return nil
	}
	return append([]int{}, values...)
}

func cloneEvents(events []Event) []Event {
	if events == nil {
		return nil
	}
	return append([]Event{}, events...)
}

// Clone returns a deep copy so an invalid or rejected move cannot alias the
// original state.
func (s State) Clone() State {
	clone := s
	clone.Deck = cloneCards(s.Deck)
	clone.FaceUp = cloneCards(s.FaceUp)
	clone.History = cloneEvents(s.History)
	clone.Players = make([]PlayerState, len(s.Players))
	for i, player := range s.Players {
		copyPlayer := player
		copyPlayer.Hand = cloneCards(player.Hand)
		copyPlayer.Discard = cloneCards(player.Discard)
		if player.Snapshots != nil {
			copyPlayer.Snapshots = make([]Snapshot, len(player.Snapshots))
			for j, snapshot := range player.Snapshots {
				copyPlayer.Snapshots[j] = snapshot
				copyPlayer.Snapshots[j].Cards = cloneCards(snapshot.Cards)
			}
		}
		clone.Players[i] = copyPlayer
	}
	if s.LastRound != nil {
		last := *s.LastRound
		last.Winners = cloneInts(s.LastRound.Winners)
		if s.LastRound.Hands != nil {
			last.Hands = make([]RevealedHand, len(s.LastRound.Hands))
			for i, hand := range s.LastRound.Hands {
				last.Hands[i] = hand
				last.Hands[i].Cards = cloneCards(hand.Cards)
			}
		}
		clone.LastRound = &last
	}
	return clone
}

func (s *State) matchWinner() int {
	best := -1
	winner := 0
	tied := false
	for _, player := range s.Players {
		switch {
		case player.Callbacks > best:
			best = player.Callbacks
			winner = player.Seat
			tied = false
		case player.Callbacks == best:
			tied = true
		}
	}
	if winner == 0 || tied || best < s.Target {
		return 0
	}
	return winner
}

func (s *State) endTurn() error {
	remaining := s.remainingSeats()
	if len(remaining) <= 1 {
		return s.finishRound(remaining, false)
	}
	if len(s.Deck) == 0 {
		return s.finishRoundByDeck()
	}
	s.advanceActive()
	return s.beginTurn()
}

func (s *State) advanceActive() {
	for i := 1; i <= s.PlayerCount; i++ {
		seat := ((s.ActiveSeat-1+i)%s.PlayerCount + s.PlayerCount) % s.PlayerCount
		seat++
		if !s.player(seat).Eliminated {
			s.ActiveSeat = seat
			return
		}
	}
}

func (s *State) finishRoundByDeck() error {
	remaining := s.remainingSeats()
	best := -1
	var tied []int
	for _, seat := range remaining {
		value := 0
		if hand := s.player(seat).Hand; len(hand) > 0 {
			value = int(hand[len(hand)-1])
		}
		switch {
		case value > best:
			best = value
			tied = []int{seat}
		case value == best:
			tied = append(tied, seat)
		}
	}

	if len(tied) > 1 {
		bestDiscard := -1
		var byDiscard []int
		for _, seat := range tied {
			total := sumCards(s.player(seat).Discard)
			switch {
			case total > bestDiscard:
				bestDiscard = total
				byDiscard = []int{seat}
			case total == bestDiscard:
				byDiscard = append(byDiscard, seat)
			}
		}
		tied = byDiscard
	}
	return s.finishRound(tied, true)
}

func (s *State) finishRound(winners []int, revealHands bool) error {
	for _, seat := range winners {
		if player := s.player(seat); player != nil {
			player.Callbacks++
		}
	}

	result := &RoundResult{
		Round:   s.Round,
		Winners: append([]int(nil), winners...),
	}
	if !s.SecretDrawn && s.Secret != CardNone {
		result.Secret = s.Secret
	}
	if revealHands {
		for _, player := range s.Players {
			if player.Eliminated {
				continue
			}
			result.Hands = append(result.Hands, RevealedHand{
				Seat:  player.Seat,
				Cards: append([]Card(nil), player.Hand...),
			})
		}
	}
	s.LastRound = result
	s.History = append(s.History, Event{
		Turn:    s.Turn,
		Round:   s.Round,
		Kind:    "round_end",
		Winners: append([]int(nil), winners...),
		Secret:  result.Secret,
	})

	if winner := s.matchWinner(); winner != 0 {
		s.Phase = PhaseMatchOver
		s.MatchWinnerSeat = winner
		s.History = append(s.History, Event{
			Turn:        s.Turn,
			Round:       s.Round,
			Kind:        "match_end",
			MatchWinner: winner,
		})
		return nil
	}

	start := 0
	if len(winners) == 1 {
		start = winners[0]
	} else if len(winners) > 1 {
		rng := roundRNG(s.Seed, s.Round+1)
		start = winners[rng.Intn(len(winners))]
	}
	return s.startRound(start)
}

func (s *State) validateMove(move Move) error {
	switch move.Card {
	case ReferenceCheck:
		if s.allOthersProtected(move.Seat) {
			return nil
		}
		if move.Guess == ReferenceCheck || !move.Guess.valid() {
			return fmt.Errorf("%w: %s", ErrInvalidGuess, move.Guess.Name())
		}
		return s.validateOtherTarget(move.Seat, move.Target)
	case InformationalInterview, SalaryComparison, CounterOffer:
		if s.allOthersProtected(move.Seat) {
			return nil
		}
		return s.validateOtherTarget(move.Seat, move.Target)
	case PersonalDay, NonCompete, DreamOffer:
		return nil
	case Reassignment:
		target := s.player(move.Target)
		if target == nil || target.Eliminated {
			return fmt.Errorf("%w: seat %d", ErrInvalidTarget, move.Target)
		}
		if move.Target != move.Seat && target.Protected {
			return fmt.Errorf("%w: seat %d is protected", ErrInvalidTarget, move.Target)
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown card", ErrCardNotInHand)
	}
}

func (s *State) validateOtherTarget(seat, target int) error {
	player := s.player(target)
	if player == nil || target == seat || player.Eliminated || player.Protected {
		return fmt.Errorf("%w: seat %d", ErrInvalidTarget, target)
	}
	return nil
}

// Move is one player's action. Version must equal the state version the client
// last saw. Target and Guess are zero when the card does not use them.
type Move struct {
	Seat    int  `json:"seat"`
	Version int  `json:"version"`
	Card    Card `json:"card"`
	Target  int  `json:"target,omitempty"`
	Guess   Card `json:"guess,omitempty"`
}

// Apply validates and applies a move. On any error the original state is
// returned unchanged.
func (s State) Apply(move Move) (State, error) {
	if move.Version != s.Version {
		return s, fmt.Errorf("%w: move has %d, state has %d", ErrStaleVersion, move.Version, s.Version)
	}
	if s.Phase != PhasePlaying {
		return s, ErrMatchOver
	}
	if move.Seat != s.ActiveSeat {
		return s, fmt.Errorf("%w: seat %d is active", ErrNotYourTurn, s.ActiveSeat)
	}
	player := s.player(move.Seat)
	if player == nil || player.Eliminated {
		return s, ErrInvalidSeat
	}
	if !contains(player.Hand, move.Card) {
		return s, fmt.Errorf("%w: %s", ErrCardNotInHand, move.Card.Name())
	}
	if contains(player.Hand, NonCompete) &&
		(contains(player.Hand, CounterOffer) || contains(player.Hand, Reassignment)) &&
		move.Card != NonCompete {
		return s, ErrMustPlayNonCompete
	}
	if err := s.validateMove(move); err != nil {
		return s, err
	}

	next := s.Clone()
	if err := next.applyMove(move); err != nil {
		return s, err
	}
	next.Version++
	return next, nil
}

func (s *State) applyMove(move Move) error {
	actor := s.player(move.Seat)
	removeCard(&actor.Hand, move.Card)
	actor.Discard = append(actor.Discard, move.Card)

	result := ""
	switch move.Card {
	case ReferenceCheck:
		if s.allOthersProtected(move.Seat) {
			result = "no_effect"
			break
		}
		target := s.player(move.Target)
		if contains(target.Hand, move.Guess) {
			s.eliminate(target)
			result = "correct"
		} else {
			result = "wrong"
		}
	case InformationalInterview:
		if s.allOthersProtected(move.Seat) {
			result = "no_effect"
			break
		}
		target := s.player(move.Target)
		s.addSnapshot(actor, target.Seat, target.Hand)
		result = "seen"
	case SalaryComparison:
		if s.allOthersProtected(move.Seat) {
			result = "no_effect"
			break
		}
		target := s.player(move.Target)
		s.addSnapshot(actor, target.Seat, target.Hand)
		s.addSnapshot(target, actor.Seat, actor.Hand)
		actorValue := handValue(actor.Hand)
		targetValue := handValue(target.Hand)
		switch {
		case actorValue < targetValue:
			s.eliminate(actor)
			result = "actor_eliminated"
		case targetValue < actorValue:
			s.eliminate(target)
			result = "target_eliminated"
		default:
			result = "tie"
		}
	case PersonalDay:
		actor.Protected = true
		result = "protected"
	case Reassignment:
		target := s.player(move.Target)
		discarded := target.Hand
		target.Hand = nil
		target.Discard = append(target.Discard, discarded...)
		if contains(discarded, DreamOffer) {
			s.eliminate(target)
			result = "eliminated"
			break
		}
		replacement, ok := s.draw()
		if !ok {
			replacement, ok = s.drawSecret()
		}
		if ok {
			target.Hand = []Card{replacement}
		}
		result = "reassigned"
	case CounterOffer:
		if s.allOthersProtected(move.Seat) {
			result = "no_effect"
			break
		}
		target := s.player(move.Target)
		actor.Hand, target.Hand = target.Hand, actor.Hand
		result = "swapped"
	case NonCompete:
		result = "none"
	case DreamOffer:
		s.eliminate(actor)
		result = "eliminated"
	}

	s.History = append(s.History, Event{
		Turn:   s.Turn,
		Round:  s.Round,
		Kind:   "play",
		Seat:   move.Seat,
		Card:   move.Card,
		Target: move.Target,
		Guess:  move.Guess,
		Result: result,
	})

	return s.endTurn()
}

func handValue(hand []Card) int {
	if len(hand) == 0 {
		return 0
	}
	return int(hand[len(hand)-1])
}
