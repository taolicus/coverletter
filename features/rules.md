# Rules

Cover Letter adapts *Love Letter* for 2–4 players. This document is the
authoritative rule set for the game engine. It resolves the multi-round
ambiguity in [spec](spec.md) §5, §6, §12 and §13 and records the decisions the
engine needs.

The adaptation keeps the standard 16-card deck and its mechanics, renames the
cards around a job application, and adds two digital-play decisions: private
information is stored as per-player snapshots, and a match tie is broken by
another round rather than a shared win.

## Match

- A room seats 2–4 players. The seats are fixed when the game starts.
- A match is a series of rounds. Each round winner receives one **callback**.
  If several players tie for the round win, each tied player receives one.
- The match target depends on the number of seated players:

  | Players | Callbacks to win |
  | ------: | --------------: |
  |       2 |               7 |
  |       3 |               5 |
  |       4 |               4 |

- A player wins the match when, after a round, exactly one player has at least
  the target and strictly more callbacks than every other player. If several
  players reach the target in the same round, play another round. This is a
  deliberate adaptation: standard Love Letter allows a shared win, while Cover
  Letter has a single position to fill.
- The first round starts with a random seated player. The winner of the previous
  round starts the next; if several players tied, choose randomly among them.
  The engine derives these choices from the stored match seed.
- Callbacks never decrease. Eliminated players keep their callbacks and can
  still view the match.

## Cards

| Value | Name                  | Copies | Effect                                                                                                                                                                                                 |
| ----: | --------------------- | -----: | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
|     1 | Reference Check       |      5 | Choose another player and name a card other than Reference Check. If that player holds it, they are eliminated.                                                                                         |
|     2 | Informational Interview |    2 | Choose another player and privately look at their hand.                                                                                                                                                 |
|     3 | Salary Comparison     |      2 | Choose another player and privately compare hands. The lower value is eliminated; a tie eliminates neither player.                                                                                      |
|     4 | Personal Day          |      2 | Until the start of your next turn, other players cannot choose you for a card effect.                                                                                                                   |
|     5 | Reassignment          |      2 | Choose any player, including yourself. They discard their hand without resolving its effect and draw a replacement.                                                                                     |
|     6 | Counter-Offer         |      1 | Choose another player and trade hands.                                                                                                                                                                  |
|     7 | Non-Compete           |      1 | No effect when played. If you hold this card with Counter-Offer or Reassignment, you must play this card.                                                                                               |
|     8 | Dream Offer           |      1 | If you play or discard this card for any reason, you are eliminated.                                                                                                                                    |

Card values are also their ranks. The deck is exactly these 16 cards.

### Targeting and protection

- Reference Check, Informational Interview, Salary Comparison and Counter-Offer
  must choose **another** player. They cannot target the player who played them.
- Reassignment may choose **any** player, including the player who played it.
- Personal Day protects a player until the start of their next turn. While
  protected, another player cannot choose them.
- If every other player still in the round is protected when a card that must
  choose another player is played, that card is discarded with no effect.
- If every other player is protected when Reassignment is played, the player
  must choose themselves.

### Card details

- **Reference Check.** The named card must be a non-Reference Check card. A
  correct guess eliminates the target. A wrong guess has no effect. The named
  card is public; the target's hand is only revealed if the guess is correct.
- **Informational Interview.** The player who played it sees the target's hand
  as a private snapshot (see [Private information](#private-information)).
- **Salary Comparison.** Both participants see the other's hand as a private
  snapshot. The lower value is eliminated. The eliminated player's hand is then
  discarded face up, so it becomes public; the winner's hand remains private.
- **Personal Day.** The protection ends at the start of the protected player's
  next turn, or when the round ends.
- **Reassignment.** The target discards their hand without resolving its effect
  and draws a replacement. If the discarded card is the Dream Offer, the target
  is eliminated and does **not** draw a replacement. If the draw deck is empty,
  the target draws the secret card (see [Round setup](#round-setup)). If the
  target is the player who played Reassignment, they discard their remaining
  card and draw a replacement.
- **Counter-Offer.** The two players exchange hands. The played Counter-Offer
  stays in the discard pile.
- **Non-Compete.** The forced play applies whenever the holder has Non-Compete
  and either Counter-Offer or Reassignment in hand, including the card just
  drawn. The engine rejects a move that plays Counter-Offer or Reassignment
  while the holder has Non-Compete.
- **Dream Offer.** Playing it or discarding it for any reason eliminates the
  holder. A Reassignment-forced discard eliminates the holder and prevents the
  replacement draw.

## Round setup

1. Shuffle all 16 cards.
2. Remove one card face down as the **secret card**. It stays hidden until a
   Reassignment draws it or the round ends.
3. For a 2-player round, remove three more cards and place them face up. They
   are public, unused, and never drawn.
4. Deal one card to each player. The remaining cards form the face-down draw
   deck.
5. The first round's starting player is chosen randomly from the seated
   players. Later rounds start with the previous round's winner, or a random
   choice among tied winners.

The draw deck order is hidden; only its size is public. The secret card is not
part of the draw deck and cannot be drawn by a normal draw. The 2-player
face-up cards are not part of the draw deck.

Deck sizes after setup:

| Players | Secret | Face up | Dealt | Draw deck |
| ------: | -----: | ------: | ----: | --------: |
|       2 |      1 |       3 |     2 |        10 |
|       3 |      1 |       0 |     3 |        12 |
|       4 |      1 |       0 |     4 |        11 |

## Turn

1. The active player draws the top card of the draw deck.
2. The active player now has two cards and must play one. The Non-Compete
   forced play applies.
3. Resolve the played card's effect.
4. If the active player was eliminated, discard their remaining hand face up
   without resolving its effect and end their turn.
5. Check the round-end conditions in [End of a round](#end-of-a-round).
6. If the round continues, the next player in seat order who is still in the
   round becomes active. Eliminated players are skipped.

The engine never advances a turn because a player is offline. It waits for the
active player's move indefinitely, as required by [spec](spec.md) §5.

## Elimination

A player is eliminated by a correct Reference Check, a lost Salary Comparison,
playing or discarding the Dream Offer, or a Reassignment that forces the Dream
Offer to be discarded.

On elimination:

- The player discards their remaining hand face up without resolving its
  effect. Discarding the Dream Offer still eliminates its holder; if they are
  already eliminated, the result is unchanged.
- The discarded hand becomes public.
- The player takes no further turns this round and is skipped in turn order.
- Their callbacks are unchanged.

If only one player remains in the round, the round ends immediately and that
player wins.

## End of a round

After each turn, check in this order:

1. **One player left.** If only one player remains in the round, that player
   wins the round immediately.
2. **Deck empty.** If the draw deck is empty, all remaining players reveal their
   hands:
   - The highest card value wins.
   - If tied, the player with the highest total value of cards in their discard
     pile wins. The total includes the card played on the final turn.
   - If still tied, all tied players win the round and each receives one
     callback.
3. Otherwise, the next active player takes a turn.

The round does not end merely because a player is offline. It ends only by the
conditions above.

The secret card is revealed at round end for transparency; it does not affect
scoring. The 2-player face-up cards were already public.

## Match end

After a round's callbacks are awarded:

- If exactly one player has at least the target and strictly more callbacks
  than every other player, that player wins the match.
- Otherwise, set up another round. The previous round's winner starts; if
  several players tied, choose randomly among them.

A match can therefore continue past the target when several players reach it in
the same round. The first player to hold a strict lead at or above the target
wins.

## Private information

The engine's public view contains:

- seat numbers, display names, elimination status and callbacks;
- the active player and the round number;
- the draw deck size;
- all discard piles and eliminated hands;
- the 2-player face-up set-aside cards;
- the secret card after the round ends;
- the public move history.

The engine's per-player view additionally contains:

- the viewer's own hand;
- private snapshots the viewer acquired.

A **snapshot** records the cards a player saw through Informational Interview or
Salary Comparison, plus the round and turn in which they saw them. A snapshot
does not update when the target's hand changes. It is returned only to the
player who acquired it and never appears in the public view, another player's
view, or the public history. This preserves the tabletop memory advantage while
allowing a player to close the browser and return later.

In a Salary Comparison, both participants acquire a snapshot of the other's
hand. The eliminated player's hand then becomes public through the discard pile;
the winner's hand stays private.

The public history records public events: the player, the card played, the
target, the public result, eliminations, round end, callbacks and match end. It
does not record the contents of a private snapshot. A correct Reference Check
reveals the target's hand through the public discard; a wrong guess does not.

## Async play

- A game never advances automatically.
- A player may close the browser and return to the same seat and game state.
- The engine state is persisted after every successful move and includes the
  draw deck order, hands, discards, secret card, snapshots, callbacks, active
  player and state version.
- A move carries the state version it expects. A stale or replayed move is
  rejected rather than applied twice.
- The MVP has no turn timers, reminders or auto-play. Room abandonment is a
  separate open question; see `issues/abandoned-rooms-undefined.md`.

## Deliberate adaptations

- **Multi-round match play is in the MVP.** This resolves the contradiction
  between [spec](spec.md) §6 (round and game scores) and §12 (multiple rounds
  listed as a future enhancement).
- **Strict-lead match end.** Standard Love Letter permits a shared win; Cover
  Letter plays another round until one player has a strict lead at the target.
- **Private snapshots.** Standard play relies on memory. Cover Letter stores
  what a player was shown so async play does not punish a return visit.
- **Revealed secret card at round end.** This is for transparency; it has no
  effect on scoring.
