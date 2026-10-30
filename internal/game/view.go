package game

// PublicView is the information every player and spectator may see. It never
// contains a current hand, the undrawn secret card, or another player's
// snapshots.
type PublicView struct {
	Version         int            `json:"version"`
	Phase           Phase          `json:"phase"`
	Round           int            `json:"round"`
	Turn            int            `json:"turn"`
	PlayerCount     int            `json:"playerCount"`
	Target          int            `json:"target"`
	ActiveSeat      int            `json:"activeSeat"`
	DrawCount       int            `json:"drawCount"`
	FaceUp          []Card         `json:"faceUp,omitempty"`
	Players         []PublicPlayer `json:"players"`
	History         []Event        `json:"history,omitempty"`
	LastRound       *RoundResult   `json:"lastRound,omitempty"`
	MatchWinnerSeat int            `json:"matchWinnerSeat,omitempty"`
}

// PublicPlayer is one seat's public state.
type PublicPlayer struct {
	Seat       int    `json:"seat"`
	Name       string `json:"name"`
	Eliminated bool   `json:"eliminated"`
	Protected  bool   `json:"protected"`
	Callbacks  int    `json:"callbacks"`
	HandCount  int    `json:"handCount"`
	Discard    []Card `json:"discard,omitempty"`
}

// PlayerView is the public view plus one player's private information.
type PlayerView struct {
	PublicView
	ViewerSeat int        `json:"viewerSeat"`
	Hand       []Card     `json:"hand,omitempty"`
	Snapshots  []Snapshot `json:"snapshots,omitempty"`
}

// PublicView returns the information shared with everyone.
func (s *State) PublicView() PublicView {
	view := PublicView{
		Version:         s.Version,
		Phase:           s.Phase,
		Round:           s.Round,
		Turn:            s.Turn,
		PlayerCount:     s.PlayerCount,
		Target:          s.Target,
		ActiveSeat:      s.ActiveSeat,
		DrawCount:       len(s.Deck),
		FaceUp:          cloneCards(s.FaceUp),
		Players:         make([]PublicPlayer, len(s.Players)),
		History:         cloneEvents(s.History),
		MatchWinnerSeat: s.MatchWinnerSeat,
	}
	for i, player := range s.Players {
		view.Players[i] = PublicPlayer{
			Seat:       player.Seat,
			Name:       player.Name,
			Eliminated: player.Eliminated,
			Protected:  player.Protected,
			Callbacks:  player.Callbacks,
			HandCount:  len(player.Hand),
			Discard:    cloneCards(player.Discard),
		}
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
		view.LastRound = &last
	}
	return view
}

// View returns the public view plus the viewer's hand and snapshots. The seat
// must exist, but eliminated players can still view the match.
func (s *State) View(seat int) (PlayerView, error) {
	player := s.player(seat)
	if player == nil {
		return PlayerView{}, ErrInvalidSeat
	}

	view := PlayerView{
		PublicView: s.PublicView(),
		ViewerSeat: seat,
		Hand:       cloneCards(player.Hand),
	}
	if player.Snapshots != nil {
		view.Snapshots = make([]Snapshot, len(player.Snapshots))
		for i, snapshot := range player.Snapshots {
			view.Snapshots[i] = snapshot
			view.Snapshots[i].Cards = cloneCards(snapshot.Cards)
		}
	}
	return view, nil
}
