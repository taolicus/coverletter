---
area: game
---

# Implement the server-authoritative game engine

## Context

`features/spec.md` §5, §6, §9, §13. The engine owns all rules: deck, deal,
turns, card effects, elimination, round end, match scoring, and move
validation. It must be deterministic given a seed for tests and must never
expose another player's hand. The HTTP layer is out of scope; the engine is a
pure package with a state type and a move API.

The rules are documented in `features/rules.md`. Implement exactly that
document; do not guess at an effect or an edge case. If the document is
ambiguous, record the question and resolve it before implementing the affected
rule.

The engine state must be serializable without losing information so the HTTP
layer can persist it. State carries a monotonically increasing version; a move
supplies the version it expects, so a replayed move is rejected rather than
applied twice. The state also records an ordered public move history.

## Acceptance criteria

- `features/rules.md` defines the card table, deck setup per player count, turn
  procedure, round end, match end, and tie-breaks; the engine implements exactly
  that.
- The engine creates a match for the supported player counts from a seed, deals
  the documented hands and setup, and produces a view for one player that omits
  every other player's current hand.
- The 2-player setup (secret card plus three face-up set-aside cards), the
  3- and 4-player deck sizes, and the secret-card draw by Reassignment match
  `features/rules.md`.
- Every card effect is implemented, including the forced Countess discard
  (Non-Compete), Handmaid immunity (Personal Day), legal targeting, and
  elimination.
- Invalid moves (wrong player, wrong phase, illegal target, card not in hand,
  stale version) return an error and leave the state unchanged.
- Round end and match end are detected; the winner and scores are correct for
  last-player-standing, deck-exhaustion, the discard-total tie-break, shared
  round wins, and the strict-lead match end.
- Private snapshots from Informational Interview and Salary Comparison are
  returned only to the player who acquired them, do not update when the target's
  hand changes, and never appear in public views or history.
- The public view contains no current hand, no undrawn secret card, and no
  snapshot belonging to another player.
- The Non-Compete forced play is enforced, and the engine never advances a turn
  because a player is offline.
- The state serializes and deserializes to the same value, including the draw
  pile, hands, discards, secret card, 2-player face-up cards, snapshots,
  elimination status, active player, round, history, scores, and version.
- Unit tests cover every card, forced plays, immunity, elimination, deck
  exhaustion, tie-breaks, shared round wins, the strict-lead match end, 2-player
  setup, secret-card draws, private snapshots, stale moves, serialization, and a
  full seeded match. `go test ./...` passes.
- `features/game-engine.md` records the state model, the move API, the version
  rule, and the public/private view boundary.

## Verification

Run `go test ./...`, including `-race` if the engine is used concurrently. Walk
a seeded match in a test and assert both the public view and each player's
private view. Review the card table in `features/rules.md` against the
implementation.

## Outcome
