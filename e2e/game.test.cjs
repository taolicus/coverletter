'use strict';

const assert = require('node:assert/strict');

const { chromium } = require('playwright');

const {
  assertNoHorizontalScroll,
  inspectLayout,
  newPlayer,
  startServer,
  stopServer,
} = require('./support.cjs');

const MAX_TURNS = 160;

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function waitUntil(check, timeout = 10000, interval = 100) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    if (await check()) {
      return;
    }
    await sleep(interval);
  }
  throw new Error('timed out waiting for condition');
}

async function gameState(page) {
  return page.evaluate(() => {
    const root = document.querySelector('#game-view');
    if (!root) {
      return null;
    }
    return {
      version: Number(root.dataset.version),
      phase: root.dataset.phase,
      canMove: root.dataset.canMove === 'true',
      status: document.querySelector('#turn-status')?.textContent || '',
      cards: [...document.querySelectorAll('#your-hand button.hand-card')].map((button) => ({
        card: Number(button.dataset.card),
        disabled: button.disabled,
      })),
      matchResult: Boolean(document.querySelector('#match-result')),
      moveError: document.querySelector('#move-error')?.textContent || null,
    };
  });
}

async function waitForTurnOrEnd(pages, timeout = 12000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    for (const page of pages) {
      const state = await gameState(page);
      if (state && (state.phase === 'match_over' || state.status === 'Your turn')) {
        return { page, state };
      }
    }
    await sleep(100);
  }
  const states = await Promise.all(pages.map((page) => gameState(page)));
  throw new Error(`timed out waiting for a turn or the end: ${JSON.stringify(states)}`);
}

// The test already exercises the four-second poll and the visibility stop
// separately. For the long match loop, refresh remote boards directly instead
// of waiting a poll interval after every one of dozens of moves.
async function syncPage(page) {
  await page.evaluate(async () => {
    await refreshCurrent({ quiet: true, force: true });
  });
}

async function syncPages(pages) {
  await Promise.all(pages.map((page) => syncPage(page)));
}

async function choosePlayableCard(page) {
  // Playwright checks click stability across animation frames. Bring the
  // player being acted on to the foreground so a background tab cannot turn
  // every click into a long wait.
  await page.bringToFront();
  const cards = await page
    .locator('#your-hand button.hand-card:not([disabled])')
    .evaluateAll((buttons) => buttons.map((button) => Number(button.dataset.card)));
  assert.ok(cards.length > 0, 'the active player has no playable card button');

  const card = cards.find((value) => value !== 8) ?? cards[0];
  await page
    .locator(`#your-hand button.hand-card[data-card="${card}"]:not([disabled])`)
    .first()
    .click();
  await page.locator('#move-controls').waitFor();
}

async function fillMoveOptions(page) {
  for (const id of ['#move-target', '#move-guess']) {
    const select = page.locator(id);
    if ((await select.count()) === 0) {
      continue;
    }
    const values = await select
      .locator('option:not([value=""])')
      .evaluateAll((options) => options.map((option) => option.value));
    assert.ok(values.length > 0, `${id} has no selectable options`);
    await select.selectOption(values[0]);
  }
}

async function playTurn(page, debug = false) {
  const before = await gameState(page);
  assert.ok(before && before.status === 'Your turn', 'playTurn requires the active page');
  let started = Date.now();
  await choosePlayableCard(page);
  if (debug) {
    console.log(`e2e: choose ${Date.now() - started}ms`);
  }
  started = Date.now();
  await fillMoveOptions(page);
  if (debug) {
    console.log(`e2e: fill ${Date.now() - started}ms`);
  }
  started = Date.now();
  await page.locator('#play-card').click({ noWaitAfter: true });
  if (debug) {
    console.log(`e2e: click ${Date.now() - started}ms`);
  }
  started = Date.now();
  await waitUntil(async () => {
    const next = await gameState(page);
    return next && next.version !== before.version;
  }, 12000, 50);
  if (debug) {
    console.log(`e2e: version ${Date.now() - started}ms`);
  }
  const error = await page.locator('#move-error').textContent().catch(() => null);
  assert.equal(error, null, `move failed: ${error}`);
}

async function fetchGame(page, roomID) {
  return page.evaluate(async (id) => {
    const response = await fetch(`/api/rooms/${id}/game`, {
      credentials: 'same-origin',
      headers: { Accept: 'application/json' },
    });
    return { status: response.status, data: await response.json() };
  }, roomID);
}

function chooseServerMove(data) {
  const game = data.game;
  const hand = game.hand || [];
  const forced = hand.includes(7) && (hand.includes(5) || hand.includes(6));
  const card = forced ? 7 : hand.find((value) => value !== 8) ?? hand[0];
  const seat = game.viewerSeat;
  const others = game.players.filter(
    (player) => player.seat !== seat && !player.eliminated && !player.protected,
  );

  let target = 0;
  if (card === 5) {
    const options = game.players.filter(
      (player) => !player.eliminated && (!player.protected || player.seat === seat),
    );
    target = options[0] ? options[0].seat : seat;
  } else if ([1, 2, 3, 6].includes(card) && others.length) {
    target = others[0].seat;
  }

  return {
    version: game.version,
    card,
    target,
    guess: card === 1 && target ? 2 : 0,
  };
}

async function main() {
  const server = startServer();
  let browser;

  try {
    const baseURL = await server.url;
    browser = await chromium.launch({ headless: true });

    const ada = await newPlayer(browser, baseURL, 'Ada');
    await ada.page.getByRole('button', { name: 'Create a room' }).click();
    await ada.page.waitForURL(/\/room\/[a-z0-9]+$/);

    const roomURL = ada.page.url();
    const roomID = roomURL.split('/').pop();
    const shareURL = await ada.page.locator('#share-url').inputValue();

    const grace = await newPlayer(browser, baseURL, 'Grace');
    await grace.page.goto(shareURL);
    await grace.page.getByRole('button', { name: /Take seat/ }).click();
    await grace.page.locator('#room-notice').waitFor();

    await ada.page.locator('#start-game').waitFor({ timeout: 10000 });
    assert.equal(await grace.page.locator('#start-game').count(), 0);

    await ada.page.locator('#start-game').click();
    await Promise.all([
      ada.page.locator('#game-view').waitFor({ timeout: 12000 }),
      grace.page.locator('#game-view').waitFor({ timeout: 12000 }),
    ]);

    const pages = [ada.page, grace.page];
    assert.ok((await ada.page.locator('#game-seats').textContent()).includes('Ada'));
    assert.ok((await grace.page.locator('#game-seats').textContent()).includes('Grace'));

    const desktopLayout = await inspectLayout(ada.page);
    assert.equal(desktopLayout.gameColumns, 2);
    assert.equal(desktopLayout.gameSeatColumns, 2);
    assertNoHorizontalScroll(desktopLayout, 'desktop game');

    const initial = await waitForTurnOrEnd(pages);
    const waiting = initial.page === ada.page ? grace.page : ada.page;
    const waitingState = await gameState(waiting);
    assert.equal(waitingState.canMove, false);
    assert.match(waitingState.status, /^Waiting for /);
    assert.equal(await waiting.locator('#your-hand button.hand-card').count(), 0);
    assert.equal(await waiting.locator('#play-card').count(), 0);

    await initial.page.setViewportSize({ width: 390, height: 844 });
    await initial.page.locator('#game-view').waitFor();
    await choosePlayableCard(initial.page);
    const phoneLayout = await inspectLayout(initial.page);
    assertNoHorizontalScroll(phoneLayout, 'phone game');
    assert.deepEqual(phoneLayout.undersized, []);
    assert.equal(phoneLayout.gameColumns, 1);
    assert.equal(phoneLayout.gameSeatColumns, 1);
    assert.ok(phoneLayout.bodyFontSize >= 16, `phone body font is ${phoneLayout.bodyFontSize}px`);
    await initial.page.setViewportSize({ width: 1280, height: 900 });

    const beforeStale = await fetchGame(initial.page, roomID);
    assert.equal(beforeStale.status, 200);
    const directMove = chooseServerMove(beforeStale.data);

    await initial.page.route('**/api/rooms/*/game', async (route) => {
      if (route.request().method() === 'GET') {
        await route.abort('failed');
      } else {
        await route.continue();
      }
    });

    const direct = await initial.page.context().request.post(
      `${baseURL}/api/rooms/${roomID}/move`,
      { data: directMove },
    );
    assert.equal(direct.status(), 200, await direct.text());

    await fillMoveOptions(initial.page);
    await initial.page.locator('#play-card').click({ noWaitAfter: true });
    await initial.page.locator('#move-error').waitFor({ timeout: 10000 });
    const staleError = await initial.page.locator('#move-error').textContent();
    assert.match(staleError, /board changed|refreshed/i);

    await initial.page.unroute('**/api/rooms/*/game');
    await syncPage(initial.page);
    await waitUntil(async () => {
      const next = await gameState(initial.page);
      return next && next.version !== beforeStale.data.game.version;
    }, 10000, 50);

    await syncPages(pages);
    const pollingTurn = await waitForTurnOrEnd(pages);
    const pollingOther = pollingTurn.page === ada.page ? grace.page : ada.page;
    const pollingHandled = playTurn(pollingTurn.page);
    await pollingHandled;
    const activeVersion = (await gameState(pollingTurn.page)).version;
    await waitUntil(async () => {
      const next = await gameState(pollingOther);
      return next && next.version === activeVersion;
    }, 9000, 100);

    const requests = [];
    const counter = async (route) => {
      if (route.request().method() === 'GET') {
        requests.push(Date.now());
      }
      await route.continue();
    };
    await pollingOther.route('**/api/rooms/*/game', counter);
    await sleep(300);
    requests.length = 0;
    await pollingOther.evaluate(() => {
      Object.defineProperty(document, 'visibilityState', {
        configurable: true,
        get: () => 'hidden',
      });
    });
    await sleep(5200);
    assert.equal(requests.length, 0, 'the hidden tab kept polling');
    await pollingOther.evaluate(() => {
      delete document.visibilityState;
      document.dispatchEvent(new Event('visibilitychange'));
    });
    await waitUntil(() => requests.length > 0, 3000);
    await pollingOther.unroute('**/api/rooms/*/game', counter);

    let turns = 0;
    for (; turns < MAX_TURNS; turns++) {
      const waitStarted = Date.now();
      const turn = await waitForTurnOrEnd(pages);
      if (turns < 3) {
        console.log(`e2e: turn ${turns + 1} wait ${Date.now() - waitStarted}ms`);
      }
      if (turn.state.phase === 'match_over') {
        break;
      }
      if (turns % 10 === 0) {
        console.log(`e2e: playing turn ${turns + 1}`);
      }
      const moveStarted = Date.now();
      await playTurn(turn.page, turns < 3);
      if (turns < 3) {
        console.log(`e2e: turn ${turns + 1} move ${Date.now() - moveStarted}ms`);
      }
      const syncStarted = Date.now();
      await syncPages(pages.filter((page) => page !== turn.page));
      if (turns < 3) {
        console.log(`e2e: turn ${turns + 1} sync ${Date.now() - syncStarted}ms`);
      }
    }
    assert.ok(turns < MAX_TURNS, `match did not finish within ${MAX_TURNS} turns`);

    await syncPages(pages);
    await Promise.all(
      pages.map((page) => page.locator('#match-result').waitFor({ timeout: 12000 })),
    );

    const final = await ada.context.request.get(`${baseURL}/api/rooms/${roomID}/game`);
    assert.equal(final.status(), 200);
    const finalData = await final.json();
    const game = finalData.game;
    assert.equal(game.phase, 'match_over');
    assert.ok(game.matchWinnerSeat > 0);
    const winner = game.players.find((player) => player.seat === game.matchWinnerSeat);
    const others = game.players.filter((player) => player.seat !== game.matchWinnerSeat);
    assert.ok(winner.callbacks >= game.target);
    assert.ok(others.every((player) => winner.callbacks > player.callbacks));
    const roundEnds = (game.history || []).filter((event) => event.kind === 'round_end');
    assert.ok(roundEnds.length >= 7, `only ${roundEnds.length} rounds were played`);

    console.log(`e2e: game flow passed in ${turns + 1} turns`);
  } finally {
    if (browser) {
      await browser.close();
    }
    await stopServer(server);
  }
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
