package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func names(count int) []string {
	result := make([]string, count)
	for i := range result {
		result[i] = fmt.Sprintf("P%d", i+1)
	}
	return result
}

// testState builds a valid mid-match state for card tests. Seat 1 is active
// with two cards; the other seats have one card each.
func testState(t *testing.T, players int) State {
	t.Helper()
	state := State{
		Seed:        11,
		Version:     1,
		PlayerCount: players,
		Target:      targetFor(players),
		Phase:       PhasePlaying,
		Round:       1,
		Turn:        1,
		ActiveSeat:  1,
		Deck:        []Card{ReferenceCheck, ReferenceCheck, ReferenceCheck},
		Secret:      CounterOffer,
		Players:     make([]PlayerState, players),
	}
	for i := range state.Players {
		state.Players[i] = PlayerState{
			Seat: i + 1,
			Name: fmt.Sprintf("P%d", i+1),
			Hand: []Card{ReferenceCheck},
		}
	}
	state.Players[0].Hand = []Card{ReferenceCheck, ReferenceCheck}
	return state
}

func setHand(state *State, seat int, cards ...Card) {
	state.player(seat).Hand = append([]Card(nil), cards...)
}

func setDiscard(state *State, seat int, cards ...Card) {
	state.player(seat).Discard = append([]Card(nil), cards...)
}

func TestNewMatch(t *testing.T) {
	tests := []struct {
		players   int
		target    int
		drawCount int
		faceUp    int
	}{
		{players: 2, target: 7, drawCount: 9, faceUp: 3},
		{players: 3, target: 5, drawCount: 11, faceUp: 0},
		{players: 4, target: 4, drawCount: 10, faceUp: 0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d players", tt.players), func(t *testing.T) {
			state, err := NewMatch(1234, names(tt.players))
			if err != nil {
				t.Fatal(err)
			}
			if state.Version != 1 {
				t.Errorf("version = %d, want 1", state.Version)
			}
			if state.Target != tt.target {
				t.Errorf("target = %d, want %d", state.Target, tt.target)
			}
			if state.Round != 1 || state.Turn != 1 {
				t.Errorf("round/turn = %d/%d, want 1/1", state.Round, state.Turn)
			}
			if state.Secret == CardNone {
				t.Error("secret card is empty")
			}
			if len(state.Deck) != tt.drawCount {
				t.Errorf("draw deck = %d, want %d", len(state.Deck), tt.drawCount)
			}
			if len(state.FaceUp) != tt.faceUp {
				t.Errorf("face-up cards = %d, want %d", len(state.FaceUp), tt.faceUp)
			}
			if active := state.player(state.ActiveSeat); active == nil || len(active.Hand) != 2 {
				t.Errorf("active seat %d hand = %v, want 2 after the opening draw", state.ActiveSeat, active.Hand)
			}
			for _, player := range state.Players {
				want := 1
				if player.Seat == state.ActiveSeat {
					want = 2
				}
				if len(player.Hand) != want {
					t.Errorf("seat %d hand = %d, want %d", player.Seat, len(player.Hand), want)
				}
			}

			total := len(state.Deck) + 1 + len(state.FaceUp)
			for _, player := range state.Players {
				total += len(player.Hand) + len(player.Discard)
			}
			if total != 16 {
				t.Errorf("total cards = %d, want 16", total)
			}

			public := state.PublicView()
			if public.DrawCount != tt.drawCount {
				t.Errorf("public drawCount = %d, want %d", public.DrawCount, tt.drawCount)
			}
			view, err := state.View(1)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(view.Hand, state.Players[0].Hand) {
				t.Errorf("view hand = %v, want %v", view.Hand, state.Players[0].Hand)
			}
		})
	}
}

func TestNewMatchRejectsUnsupportedPlayerCounts(t *testing.T) {
	for _, count := range []int{0, 1, 5} {
		if _, err := NewMatch(1, names(count)); !errors.Is(err, ErrInvalidPlayerCount) {
			t.Errorf("NewMatch(%d players) error = %v, want ErrInvalidPlayerCount", count, err)
		}
	}
}

func TestNewMatchIsDeterministic(t *testing.T) {
	first, err := NewMatch(77, names(3))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewMatch(77, names(3))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Error("same seed produced different matches")
	}
}

func TestInvalidMovesLeaveStateUnchanged(t *testing.T) {
	state := testState(t, 3)
	setHand(&state, 1, ReferenceCheck, PersonalDay)
	setHand(&state, 2, DreamOffer)

	tests := []struct {
		name string
		move Move
		want error
	}{
		{
			name: "stale version",
			move: Move{Seat: 1, Version: 0, Card: ReferenceCheck, Target: 2, Guess: DreamOffer},
			want: ErrStaleVersion,
		},
		{
			name: "wrong player",
			move: Move{Seat: 2, Version: 1, Card: ReferenceCheck, Target: 3, Guess: DreamOffer},
			want: ErrNotYourTurn,
		},
		{
			name: "card not in hand",
			move: Move{Seat: 1, Version: 1, Card: CounterOffer, Target: 2},
			want: ErrCardNotInHand,
		},
		{
			name: "target self",
			move: Move{Seat: 1, Version: 1, Card: ReferenceCheck, Target: 1, Guess: DreamOffer},
			want: ErrInvalidTarget,
		},
		{
			name: "guess Reference Check",
			move: Move{Seat: 1, Version: 1, Card: ReferenceCheck, Target: 2, Guess: ReferenceCheck},
			want: ErrInvalidGuess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := state.Clone()
			_, err := state.Apply(tt.move)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			if !reflect.DeepEqual(state, before) {
				t.Error("state changed after a rejected move")
			}
		})
	}
}

func TestReferenceCheck(t *testing.T) {
	t.Run("correct guess eliminates the target", func(t *testing.T) {
		state := testState(t, 3)
		setHand(&state, 1, ReferenceCheck, PersonalDay)
		setHand(&state, 2, DreamOffer)

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: ReferenceCheck, Target: 2, Guess: DreamOffer})
		if err != nil {
			t.Fatal(err)
		}
		target := next.player(2)
		if !target.Eliminated {
			t.Error("target was not eliminated")
		}
		if !contains(target.Discard, DreamOffer) {
			t.Error("target's hand was not revealed in the discard pile")
		}
		if len(target.Hand) != 0 {
			t.Error("target still has a hand")
		}
		if next.ActiveSeat != 3 {
			t.Errorf("active seat = %d, want 3 after skipping the eliminated seat", next.ActiveSeat)
		}
		last := next.History[len(next.History)-1]
		if last.Kind != "play" || last.Result != "correct" || last.Guess != DreamOffer {
			t.Errorf("history = %+v, want a correct guess", last)
		}
	})

	t.Run("wrong guess reveals nothing", func(t *testing.T) {
		state := testState(t, 3)
		setHand(&state, 1, ReferenceCheck, PersonalDay)
		setHand(&state, 2, DreamOffer)

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: ReferenceCheck, Target: 2, Guess: CounterOffer})
		if err != nil {
			t.Fatal(err)
		}
		target := next.player(2)
		if target.Eliminated {
			t.Error("target was eliminated by a wrong guess")
		}
		if len(target.Discard) != 0 {
			t.Errorf("target discard = %v, want empty", target.Discard)
		}
		last := next.History[len(next.History)-1]
		if last.Result != "wrong" {
			t.Errorf("result = %q, want wrong", last.Result)
		}
	})
}

func TestInformationalInterviewIsPrivate(t *testing.T) {
	state := testState(t, 3)
	setHand(&state, 1, InformationalInterview, SalaryComparison)
	setHand(&state, 2, DreamOffer)
	setHand(&state, 3, PersonalDay)

	next, err := state.Apply(Move{Seat: 1, Version: 1, Card: InformationalInterview, Target: 2})
	if err != nil {
		t.Fatal(err)
	}

	owner := next.player(1)
	if len(owner.Snapshots) != 1 {
		t.Fatalf("snapshots = %d, want 1", len(owner.Snapshots))
	}
	snapshot := owner.Snapshots[0]
	if snapshot.TargetSeat != 2 || snapshot.Round != 1 || snapshot.Turn != 1 {
		t.Errorf("snapshot = %+v, want target 2, round 1, turn 1", snapshot)
	}
	if !reflect.DeepEqual(snapshot.Cards, []Card{DreamOffer}) {
		t.Errorf("snapshot cards = %v, want [Dream Offer]", snapshot.Cards)
	}

	// The target drew on their own turn; the snapshot must not change.
	if len(next.player(2).Hand) != 2 {
		t.Fatalf("target hand = %v, want the original card plus a draw", next.player(2).Hand)
	}
	if !reflect.DeepEqual(next.player(1).Snapshots[0].Cards, []Card{DreamOffer}) {
		t.Error("snapshot changed when the target's hand changed")
	}

	ownerView, err := next.View(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(ownerView.Snapshots) != 1 {
		t.Error("owner view lost the snapshot")
	}
	otherView, err := next.View(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherView.Snapshots) != 0 {
		t.Error("another player can see the snapshot")
	}
	if next.PublicView().Players[1].HandCount != len(next.player(2).Hand) {
		t.Error("public hand count is wrong")
	}
	for _, event := range next.PublicView().History {
		if event.Kind == "play" {
			if event.Card != InformationalInterview || event.Target != 2 || event.Result != "seen" {
				t.Errorf("public history = %+v, want the play without snapshot cards", event)
			}
		}
	}
}

func TestSalaryComparison(t *testing.T) {
	t.Run("target loses", func(t *testing.T) {
		state := testState(t, 3)
		setHand(&state, 1, SalaryComparison, PersonalDay)
		setHand(&state, 2, ReferenceCheck)
		setHand(&state, 3, CounterOffer)

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: SalaryComparison, Target: 2})
		if err != nil {
			t.Fatal(err)
		}
		if !next.player(2).Eliminated {
			t.Error("lower target was not eliminated")
		}
		if next.player(1).Eliminated {
			t.Error("higher actor was eliminated")
		}
		if !reflect.DeepEqual(next.player(1).Snapshots[0].Cards, []Card{ReferenceCheck}) {
			t.Errorf("actor snapshot = %v, want [Reference Check]", next.player(1).Snapshots[0].Cards)
		}
		if !reflect.DeepEqual(next.player(2).Snapshots[0].Cards, []Card{PersonalDay}) {
			t.Errorf("target snapshot = %v, want [Personal Day]", next.player(2).Snapshots[0].Cards)
		}
		if !contains(next.player(2).Discard, ReferenceCheck) {
			t.Error("losing hand was not discarded publicly")
		}
	})

	t.Run("actor loses", func(t *testing.T) {
		state := testState(t, 3)
		setHand(&state, 1, SalaryComparison, ReferenceCheck)
		setHand(&state, 2, DreamOffer)
		setHand(&state, 3, CounterOffer)

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: SalaryComparison, Target: 2})
		if err != nil {
			t.Fatal(err)
		}
		if !next.player(1).Eliminated {
			t.Error("lower actor was not eliminated")
		}
		if next.player(2).Eliminated {
			t.Error("higher target was eliminated")
		}
		if len(next.player(1).Hand) != 0 {
			t.Error("eliminated actor still has a hand")
		}
		if next.ActiveSeat != 2 {
			t.Errorf("active seat = %d, want 2", next.ActiveSeat)
		}
	})

	t.Run("tie eliminates nobody", func(t *testing.T) {
		state := testState(t, 3)
		setHand(&state, 1, SalaryComparison, PersonalDay)
		setHand(&state, 2, PersonalDay)
		setHand(&state, 3, CounterOffer)

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: SalaryComparison, Target: 2})
		if err != nil {
			t.Fatal(err)
		}
		if next.player(1).Eliminated || next.player(2).Eliminated {
			t.Error("a tie eliminated a player")
		}
		last := next.History[len(next.History)-1]
		if last.Result != "tie" {
			t.Errorf("result = %q, want tie", last.Result)
		}
	})
}

func TestPersonalDayProtection(t *testing.T) {
	t.Run("blocks another player's target", func(t *testing.T) {
		state := testState(t, 3)
		setHand(&state, 1, PersonalDay, ReferenceCheck)
		setHand(&state, 2, DreamOffer)
		setHand(&state, 3, CounterOffer)
		state.Deck = []Card{ReferenceCheck, ReferenceCheck}

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: PersonalDay})
		if err != nil {
			t.Fatal(err)
		}
		if !next.player(1).Protected {
			t.Error("player was not protected")
		}
		if next.ActiveSeat != 2 {
			t.Fatalf("active seat = %d, want 2", next.ActiveSeat)
		}
		if !contains(next.player(2).Hand, ReferenceCheck) {
			t.Fatalf("seat 2 hand = %v, want a drawn Reference Check", next.player(2).Hand)
		}

		before := next.Clone()
		_, err = next.Apply(Move{Seat: 2, Version: next.Version, Card: ReferenceCheck, Target: 1, Guess: DreamOffer})
		if !errors.Is(err, ErrInvalidTarget) {
			t.Fatalf("targeting a protected player error = %v, want ErrInvalidTarget", err)
		}
		if !reflect.DeepEqual(next, before) {
			t.Error("rejected protected target changed the state")
		}

		// An unprotected player is still a legal target.
		_, err = next.Apply(Move{Seat: 2, Version: next.Version, Card: ReferenceCheck, Target: 3, Guess: CounterOffer})
		if err != nil {
			t.Fatalf("targeting an unprotected player: %v", err)
		}
	})

	t.Run("expires at the protected player's next turn", func(t *testing.T) {
		state := testState(t, 2)
		setHand(&state, 1, PersonalDay, ReferenceCheck)
		setHand(&state, 2, NonCompete)
		state.Deck = []Card{CounterOffer, ReferenceCheck}

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: PersonalDay})
		if err != nil {
			t.Fatal(err)
		}
		if !next.player(1).Protected {
			t.Fatal("player was not protected")
		}
		if next.ActiveSeat != 2 {
			t.Fatalf("active seat = %d, want 2", next.ActiveSeat)
		}
		if !contains(next.player(2).Hand, CounterOffer) {
			t.Fatalf("seat 2 hand = %v, want the drawn Counter-Offer", next.player(2).Hand)
		}

		// The forced Non-Compete play ends seat 2's turn without targeting.
		next, err = next.Apply(Move{Seat: 2, Version: next.Version, Card: NonCompete})
		if err != nil {
			t.Fatal(err)
		}
		if next.ActiveSeat != 1 {
			t.Fatalf("active seat = %d, want 1", next.ActiveSeat)
		}
		if next.player(1).Protected {
			t.Error("protection did not expire at the start of the next turn")
		}
	})
}

func TestAllOthersProtected(t *testing.T) {
	state := testState(t, 2)
	setHand(&state, 1, ReferenceCheck, PersonalDay)
	setHand(&state, 2, DreamOffer)
	state.player(2).Protected = true

	next, err := state.Apply(Move{Seat: 1, Version: 1, Card: ReferenceCheck, Target: 0})
	if err != nil {
		t.Fatal(err)
	}
	last := next.History[len(next.History)-1]
	if last.Result != "no_effect" {
		t.Errorf("result = %q, want no_effect", last.Result)
	}
	if next.player(2).Eliminated {
		t.Error("protected player was eliminated")
	}
}

func TestReassignment(t *testing.T) {
	t.Run("self discards and draws a replacement", func(t *testing.T) {
		state := testState(t, 3)
		setHand(&state, 1, Reassignment, PersonalDay)
		setHand(&state, 2, ReferenceCheck)
		setHand(&state, 3, CounterOffer)
		state.Deck = []Card{SalaryComparison, ReferenceCheck}

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: Reassignment, Target: 1})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(next.player(1).Hand, []Card{SalaryComparison}) {
			t.Errorf("hand = %v, want the replacement", next.player(1).Hand)
		}
		if !contains(next.player(1).Discard, PersonalDay) {
			t.Error("discarded hand is not public")
		}
	})

	t.Run("discarding Dream Offer eliminates without a replacement", func(t *testing.T) {
		state := testState(t, 3)
		setHand(&state, 1, Reassignment, PersonalDay)
		setHand(&state, 2, DreamOffer)
		setHand(&state, 3, CounterOffer)

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: Reassignment, Target: 2})
		if err != nil {
			t.Fatal(err)
		}
		if !next.player(2).Eliminated {
			t.Error("target was not eliminated")
		}
		if len(next.player(2).Hand) != 0 {
			t.Error("eliminated target drew a replacement")
		}
		if !contains(next.player(2).Discard, DreamOffer) {
			t.Error("Dream Offer is not in the discard pile")
		}
	})

	t.Run("draws the secret card when the deck is empty", func(t *testing.T) {
		state := testState(t, 2)
		setHand(&state, 1, Reassignment, PersonalDay)
		setHand(&state, 2, ReferenceCheck)
		state.Deck = nil
		state.Secret = CounterOffer

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: Reassignment, Target: 1})
		if err != nil {
			t.Fatal(err)
		}
		if next.LastRound == nil {
			t.Fatal("round did not end after the deck was exhausted")
		}
		var seat1 []Card
		for _, hand := range next.LastRound.Hands {
			if hand.Seat == 1 {
				seat1 = hand.Cards
			}
		}
		if !reflect.DeepEqual(seat1, []Card{CounterOffer}) {
			t.Errorf("revealed seat 1 hand = %v, want the secret Counter-Offer", seat1)
		}
	})

	t.Run("cannot target a protected other player", func(t *testing.T) {
		state := testState(t, 3)
		setHand(&state, 1, Reassignment, PersonalDay)
		setHand(&state, 2, ReferenceCheck)
		state.player(2).Protected = true

		_, err := state.Apply(Move{Seat: 1, Version: 1, Card: Reassignment, Target: 2})
		if !errors.Is(err, ErrInvalidTarget) {
			t.Fatalf("error = %v, want ErrInvalidTarget", err)
		}
	})
}

func TestCounterOffer(t *testing.T) {
	state := testState(t, 3)
	setHand(&state, 1, CounterOffer, PersonalDay)
	setHand(&state, 2, DreamOffer)
	setHand(&state, 3, ReferenceCheck)
	state.Deck = []Card{ReferenceCheck, ReferenceCheck}

	next, err := state.Apply(Move{Seat: 1, Version: 1, Card: CounterOffer, Target: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(next.player(1).Hand, []Card{DreamOffer}) {
		t.Errorf("actor hand = %v, want the target's Dream Offer", next.player(1).Hand)
	}
	if !contains(next.player(2).Hand, PersonalDay) {
		t.Errorf("target hand = %v, want the actor's Personal Day", next.player(2).Hand)
	}
	if !contains(next.player(1).Discard, CounterOffer) {
		t.Error("played Counter-Offer is not in the discard pile")
	}

	before := state.Clone()
	_, err = state.Apply(Move{Seat: 1, Version: 1, Card: CounterOffer, Target: 1})
	if !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("self target error = %v, want ErrInvalidTarget", err)
	}
	if !reflect.DeepEqual(state, before) {
		t.Error("rejected Counter-Offer changed the state")
	}
}

func TestForcedNonCompete(t *testing.T) {
	for _, held := range []Card{CounterOffer, Reassignment} {
		t.Run(held.Name(), func(t *testing.T) {
			state := testState(t, 3)
			setHand(&state, 1, NonCompete, held)
			setHand(&state, 2, DreamOffer)
			setHand(&state, 3, ReferenceCheck)

			target := 2
			if held == Reassignment {
				target = 1
			}
			before := state.Clone()
			_, err := state.Apply(Move{Seat: 1, Version: 1, Card: held, Target: target})
			if !errors.Is(err, ErrMustPlayNonCompete) {
				t.Fatalf("error = %v, want ErrMustPlayNonCompete", err)
			}
			if !reflect.DeepEqual(state, before) {
				t.Error("rejected forced play changed the state")
			}

			next, err := state.Apply(Move{Seat: 1, Version: 1, Card: NonCompete})
			if err != nil {
				t.Fatalf("playing Non-Compete: %v", err)
			}
			if !contains(next.player(1).Discard, NonCompete) {
				t.Error("Non-Compete is not in the discard pile")
			}
			last := next.History[len(next.History)-1]
			if last.Result != "none" {
				t.Errorf("result = %q, want none", last.Result)
			}
		})
	}

	t.Run("not forced with an unrelated card", func(t *testing.T) {
		state := testState(t, 3)
		setHand(&state, 1, NonCompete, PersonalDay)
		setHand(&state, 2, DreamOffer)
		setHand(&state, 3, ReferenceCheck)

		if _, err := state.Apply(Move{Seat: 1, Version: 1, Card: PersonalDay}); err != nil {
			t.Fatalf("playing Personal Day: %v", err)
		}
	})
}

func TestDreamOffer(t *testing.T) {
	state := testState(t, 3)
	setHand(&state, 1, DreamOffer, PersonalDay)
	setHand(&state, 2, ReferenceCheck)
	setHand(&state, 3, CounterOffer)

	next, err := state.Apply(Move{Seat: 1, Version: 1, Card: DreamOffer})
	if err != nil {
		t.Fatal(err)
	}
	actor := next.player(1)
	if !actor.Eliminated {
		t.Error("Dream Offer did not eliminate its holder")
	}
	if len(actor.Hand) != 0 {
		t.Error("eliminated holder still has a hand")
	}
	if !contains(actor.Discard, DreamOffer) || !contains(actor.Discard, PersonalDay) {
		t.Errorf("discard = %v, want both cards", actor.Discard)
	}
	if next.ActiveSeat != 2 {
		t.Errorf("active seat = %d, want 2", next.ActiveSeat)
	}
}

func TestLastPlayerStandingWinsRound(t *testing.T) {
	state := testState(t, 2)
	setHand(&state, 1, ReferenceCheck, PersonalDay)
	setHand(&state, 2, DreamOffer)
	state.Deck = []Card{SalaryComparison, ReferenceCheck}

	next, err := state.Apply(Move{Seat: 1, Version: 1, Card: ReferenceCheck, Target: 2, Guess: DreamOffer})
	if err != nil {
		t.Fatal(err)
	}
	if next.LastRound == nil {
		t.Fatal("round did not end")
	}
	if !reflect.DeepEqual(next.LastRound.Winners, []int{1}) {
		t.Errorf("winners = %v, want [1]", next.LastRound.Winners)
	}
	if next.player(1).Callbacks != 1 || next.player(2).Callbacks != 0 {
		t.Errorf("callbacks = %d/%d, want 1/0", next.player(1).Callbacks, next.player(2).Callbacks)
	}
	if next.Round != 2 {
		t.Errorf("round = %d, want 2", next.Round)
	}
	if next.ActiveSeat != 1 {
		t.Errorf("active seat = %d, want the previous winner 1", next.ActiveSeat)
	}
	if next.Phase != PhasePlaying {
		t.Errorf("phase = %q, want playing", next.Phase)
	}
	if len(next.player(1).Discard) != 0 {
		t.Errorf("discard = %v, want a fresh round", next.player(1).Discard)
	}
}

func TestDeckExhaustion(t *testing.T) {
	t.Run("highest hand wins", func(t *testing.T) {
		state := testState(t, 3)
		setHand(&state, 1, PersonalDay, ReferenceCheck)
		setHand(&state, 2, DreamOffer)
		setHand(&state, 3, SalaryComparison)
		state.Deck = nil

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: PersonalDay})
		if err != nil {
			t.Fatal(err)
		}
		if next.LastRound == nil {
			t.Fatal("round did not end")
		}
		if !reflect.DeepEqual(next.LastRound.Winners, []int{2}) {
			t.Errorf("winners = %v, want [2]", next.LastRound.Winners)
		}
		if next.LastRound.Secret != CounterOffer {
			t.Errorf("revealed secret = %v, want the original secret", next.LastRound.Secret)
		}
		if len(next.LastRound.Hands) != 3 {
			t.Errorf("revealed hands = %d, want 3", len(next.LastRound.Hands))
		}
	})

	t.Run("discard total breaks a hand tie", func(t *testing.T) {
		state := testState(t, 3)
		setHand(&state, 1, PersonalDay, ReferenceCheck)
		setHand(&state, 2, ReferenceCheck)
		setHand(&state, 3, ReferenceCheck)
		setDiscard(&state, 1)
		setDiscard(&state, 2, CounterOffer)
		setDiscard(&state, 3, ReferenceCheck)
		state.Deck = nil

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: PersonalDay})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(next.LastRound.Winners, []int{2}) {
			t.Errorf("winners = %v, want [2] from the higher discard total", next.LastRound.Winners)
		}
	})

	t.Run("a full tie shares the round win", func(t *testing.T) {
		state := testState(t, 3)
		setHand(&state, 1, PersonalDay, ReferenceCheck)
		setHand(&state, 2, ReferenceCheck)
		setHand(&state, 3, ReferenceCheck)
		setDiscard(&state, 1)
		setDiscard(&state, 2, PersonalDay)
		setDiscard(&state, 3, PersonalDay)
		state.Deck = nil

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: PersonalDay})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(next.LastRound.Winners, []int{1, 2, 3}) {
			t.Errorf("winners = %v, want all three", next.LastRound.Winners)
		}
		for _, player := range next.Players {
			if player.Callbacks != 1 {
				t.Errorf("seat %d callbacks = %d, want 1", player.Seat, player.Callbacks)
			}
		}
	})
}

func TestMatchEnd(t *testing.T) {
	t.Run("a strict lead at the target wins", func(t *testing.T) {
		state := testState(t, 4)
		setHand(&state, 1, PersonalDay, DreamOffer)
		setHand(&state, 2, ReferenceCheck)
		setHand(&state, 3, ReferenceCheck)
		setHand(&state, 4, ReferenceCheck)
		state.Deck = nil
		for i := range state.Players {
			state.Players[i].Callbacks = 3
		}

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: PersonalDay})
		if err != nil {
			t.Fatal(err)
		}
		if next.Phase != PhaseMatchOver {
			t.Errorf("phase = %q, want match_over", next.Phase)
		}
		if next.MatchWinnerSeat != 1 {
			t.Errorf("match winner = %d, want 1", next.MatchWinnerSeat)
		}
		if next.player(1).Callbacks != 4 {
			t.Errorf("winner callbacks = %d, want 4", next.player(1).Callbacks)
		}
		last := next.History[len(next.History)-1]
		if last.Kind != "match_end" || last.MatchWinner != 1 {
			t.Errorf("last history = %+v, want match_end for seat 1", last)
		}
		if _, err := next.Apply(Move{Seat: 1, Version: next.Version, Card: PersonalDay}); !errors.Is(err, ErrMatchOver) {
			t.Errorf("move after match end error = %v, want ErrMatchOver", err)
		}
	})

	t.Run("two players at the target play another round", func(t *testing.T) {
		state := testState(t, 4)
		setHand(&state, 1, PersonalDay, DreamOffer)
		setHand(&state, 2, DreamOffer)
		setHand(&state, 3, ReferenceCheck)
		setHand(&state, 4, ReferenceCheck)
		setDiscard(&state, 2, PersonalDay)
		state.Deck = nil
		state.Players[0].Callbacks = 3
		state.Players[1].Callbacks = 3

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: PersonalDay})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(next.LastRound.Winners, []int{1, 2}) {
			t.Errorf("winners = %v, want [1 2]", next.LastRound.Winners)
		}
		if next.Phase != PhasePlaying {
			t.Errorf("phase = %q, want playing", next.Phase)
		}
		if next.MatchWinnerSeat != 0 {
			t.Errorf("match winner = %d, want none", next.MatchWinnerSeat)
		}
		if next.player(1).Callbacks != 4 || next.player(2).Callbacks != 4 {
			t.Errorf("callbacks = %d/%d, want 4/4", next.player(1).Callbacks, next.player(2).Callbacks)
		}
		if next.Round != 2 {
			t.Errorf("round = %d, want 2", next.Round)
		}
	})

	t.Run("a strict lead after shared target wins", func(t *testing.T) {
		state := testState(t, 4)
		setHand(&state, 1, PersonalDay, DreamOffer)
		setHand(&state, 2, ReferenceCheck)
		setHand(&state, 3, ReferenceCheck)
		setHand(&state, 4, ReferenceCheck)
		state.Deck = nil
		state.Players[0].Callbacks = 4
		state.Players[1].Callbacks = 4
		state.Players[2].Callbacks = 3
		state.Players[3].Callbacks = 3

		next, err := state.Apply(Move{Seat: 1, Version: 1, Card: PersonalDay})
		if err != nil {
			t.Fatal(err)
		}
		if next.Phase != PhaseMatchOver || next.MatchWinnerSeat != 1 {
			t.Errorf("phase/winner = %q/%d, want match_over/1", next.Phase, next.MatchWinnerSeat)
		}
		if next.player(1).Callbacks != 5 {
			t.Errorf("winner callbacks = %d, want 5", next.player(1).Callbacks)
		}
	})
}

func TestSerializationRoundTrip(t *testing.T) {
	state, err := NewMatch(4242, names(3))
	if err != nil {
		t.Fatal(err)
	}
	assertRoundTrip(t, state)

	move := firstLegalMove(t, state)
	state, err = state.Apply(move)
	if err != nil {
		t.Fatal(err)
	}
	assertRoundTrip(t, state)
}

func assertRoundTrip(t *testing.T, state State) {
	t.Helper()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var decoded State
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state, decoded) {
		t.Errorf("state did not survive a JSON round trip\noriginal: %+v\ndecoded:  %+v", state, decoded)
	}
}

func TestPublicViewHasNoPrivateFields(t *testing.T) {
	publicType := reflect.TypeOf(PublicView{})
	if _, ok := publicType.FieldByName("Hand"); ok {
		t.Error("PublicView exposes a hand field")
	}
	if _, ok := publicType.FieldByName("Snapshots"); ok {
		t.Error("PublicView exposes snapshots")
	}
	playerType := reflect.TypeOf(PublicPlayer{})
	if _, ok := playerType.FieldByName("Hand"); ok {
		t.Error("PublicPlayer exposes a hand field")
	}
	if _, ok := playerType.FieldByName("Snapshots"); ok {
		t.Error("PublicPlayer exposes snapshots")
	}
}

func TestFullSeededMatches(t *testing.T) {
	for _, players := range []int{2, 3, 4} {
		for _, seed := range []int64{1, 42, 99} {
			t.Run(fmt.Sprintf("%d players seed %d", players, seed), func(t *testing.T) {
				state, err := NewMatch(seed, names(players))
				if err != nil {
					t.Fatal(err)
				}
				assertRoundTrip(t, state)

				for turn := 0; turn < 20000; turn++ {
					if state.Phase == PhaseMatchOver {
						break
					}
					move := firstLegalMove(t, state)
					next, err := state.Apply(move)
					if err != nil {
						t.Fatalf("turn %d: applying %+v: %v", turn, move, err)
					}
					if next.Version != state.Version+1 {
						t.Fatalf("version = %d, want %d", next.Version, state.Version+1)
					}
					assertRoundTrip(t, next)
					state = next
				}

				if state.Phase != PhaseMatchOver {
					t.Fatalf("match did not finish: phase %q, round %d, callbacks %+v", state.Phase, state.Round, callbacks(state))
				}
				winner := state.MatchWinnerSeat
				if winner < 1 || winner > players {
					t.Fatalf("match winner = %d", winner)
				}
				winnerCallbacks := state.player(winner).Callbacks
				for _, player := range state.Players {
					if player.Seat == winner {
						if player.Callbacks < state.Target {
							t.Errorf("winner callbacks = %d, target %d", player.Callbacks, state.Target)
						}
						continue
					}
					if player.Callbacks >= winnerCallbacks {
						t.Errorf("seat %d has %d callbacks, winner %d has %d",
							player.Seat, player.Callbacks, winner, winnerCallbacks)
					}
				}
			})
		}
	}
}

func callbacks(state State) []int {
	result := make([]int, len(state.Players))
	for i, player := range state.Players {
		result[i] = player.Callbacks
	}
	return result
}

// firstLegalMove chooses a deterministic legal move for the seeded-match test.
// It may peek at hands; it is a test driver, not a player strategy.
func firstLegalMove(t *testing.T, state State) Move {
	t.Helper()
	player := state.player(state.ActiveSeat)
	if player == nil {
		t.Fatal("no active player")
	}

	if contains(player.Hand, NonCompete) &&
		(contains(player.Hand, CounterOffer) || contains(player.Hand, Reassignment)) {
		return Move{Seat: player.Seat, Version: state.Version, Card: NonCompete}
	}

	for _, card := range player.Hand {
		move := Move{Seat: player.Seat, Version: state.Version, Card: card}
		switch card {
		case ReferenceCheck:
			target := firstOtherTarget(state, player.Seat)
			if target == nil {
				return move
			}
			move.Target = target.Seat
			move.Guess = DreamOffer
			if len(target.Hand) > 0 && target.Hand[0] != ReferenceCheck {
				move.Guess = target.Hand[0]
			}
			return move
		case InformationalInterview, SalaryComparison, CounterOffer:
			target := firstOtherTarget(state, player.Seat)
			if target == nil {
				return move
			}
			move.Target = target.Seat
			return move
		case Reassignment:
			move.Target = player.Seat
			return move
		default:
			return move
		}
	}
	t.Fatalf("active seat %d has no card", player.Seat)
	return Move{}
}

func firstOtherTarget(state State, seat int) *PlayerState {
	for i := range state.Players {
		player := &state.Players[i]
		if player.Seat == seat || player.Eliminated || player.Protected {
			continue
		}
		return player
	}
	return nil
}
