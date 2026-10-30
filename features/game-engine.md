# Game engine

`internal/game` is a pure Go package that owns every Cover Letter rule. It has
no HTTP, database, or clock dependency. The rules themselves live in
[rules.md](rules.md); this document records the engine's data model and API.

## State model

`game.State` is a complete, JSON-serializable match:

- match metadata: `Seed`, `Version`, `PlayerCount`, `Target`, `Phase`, `Round`,
  `Turn`, `ActiveSeat`, `MatchWinnerSeat`;
- the draw pile `Deck`, the undrawn `Secret` card, `SecretDrawn`, and the
  two-player `FaceUp` set-aside cards;
- one `PlayerState` per seat: `Hand`, `Discard`, `Eliminated`, `Protected`,
  `Callbacks`, and private `Snapshots`;
- the ordered public `History` of `Event`s;
- `LastRound`, a `RoundResult` with the round winners, the revealed secret, and
  the hands revealed at deck exhaustion.

`Snapshot` records the round, turn, target seat, and cards one player was shown.
It is a copy, so later changes to the target's hand do not alter it.

Nil slices are preserved across copies and JSON round trips. `Clone` deep-copies
every slice so a rejected move cannot alias the original state.

## Creating a match

```go
state, err := game.NewMatch(seed int64, names []string)
```

`names` supplies 2–4 display names, one per seat. Seat numbers are 1-based and
fixed for the match. The seed determines the shuffle and every random choice
(initial player, next starter after a tie), so the same seed and moves always
produce the same match.

## Move API

```go
next, err := state.Apply(game.Move{
    Seat:    1,
    Version: state.Version,
    Card:    game.ReferenceCheck,
    Target:  2,
    Guess:   game.DreamOffer,
})
```

`Move` fields:

- `Seat`: the acting seat. It must be the active seat.
- `Version`: the state version the client last saw.
- `Card`: the card played. It must be in the acting player's hand.
- `Target`: the chosen seat for cards that target; zero otherwise.
- `Guess`: the named card for Reference Check; zero otherwise.

`Apply` returns a new `State`; the receiver is never modified. On any validation
error it returns the original state unchanged, wrapped with one of the exported
sentinels (`ErrStaleVersion`, `ErrNotYourTurn`, `ErrCardNotInHand`,
`ErrInvalidTarget`, `ErrInvalidGuess`, `ErrMustPlayNonCompete`, `ErrMatchOver`,
and so on) for `errors.Is`.

Validation covers the version, phase, active seat, hand membership, the forced
Non-Compete play, targeting rules, and protection. The engine never advances a
turn because a player is offline.

## Version rule

`Version` starts at 1 and increments by exactly one for each accepted move.
Because a move carries the version it expects, replaying the same move after it
has been applied fails with `ErrStaleVersion` instead of applying it twice. The
HTTP layer can therefore persist the returned state and retry safely.

## Round and match transitions

A move resolves its card effect, appends a `play` event, and then ends the turn.
The turn ends the round when one player remains or the draw pile is empty. A
`round_end` event records the winners and any revealed secret. If the match
continues, the next round reshuffles, resets hands, discards, protection, and
snapshots, and starts with the previous winner (or a seeded choice among tied
winners).

After callbacks are awarded, the match ends only when one player has at least
the target and strictly more callbacks than every other player. A tie at the top
at or above the target starts another round; callbacks never decrease. A
`match_end` event records the winner and `Phase` becomes `match_over`.

## View boundary

The engine exposes two read-only views:

- `PublicView()`: match metadata, draw count, face-up cards, each seat's name,
  elimination, protection, callback count, hand count, and discard pile, plus
  public history and the last round result. It never contains a current hand,
  the undrawn secret, or any snapshot.
- `View(seat)`: the public view plus that seat's own hand and snapshots.
  Eliminated players can still view the match. Another player's snapshots are
  never included.

The HTTP layer must serialize `PublicView` for spectators and `View(seat)` for
the seated identity; it must not marshal `State` to a client.

## History

`History` is append-only and public. Event kinds are:

- `play`: turn, round, seat, card, target, guess, and a result such as
  `correct`, `wrong`, `seen`, `protected`, `swapped`, `eliminated`, or
  `no_effect`;
- `round_end`: winners and the revealed secret, if it was not drawn;
- `match_end`: the winning seat.

History never contains snapshot cards or another player's current hand.

## Tests

`internal/game/game_test.go` covers the card table, targeting, protection,
forced Non-Compete, elimination, snapshots, deck exhaustion, tie-breaks, shared
round wins, the strict-lead match end, serialization, and full seeded matches
for 2, 3, and 4 players. The seeded tests apply deterministic legal moves until
`PhaseMatchOver` and assert a JSON round trip after every move.
