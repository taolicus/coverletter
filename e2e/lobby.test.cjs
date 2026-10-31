'use strict';

const assert = require('node:assert/strict');

const { chromium } = require('playwright');

const {
  assertNoHorizontalScroll,
  freezeRoomUpdates,
  inspectLayout,
  newPlayer,
  startServer,
  stopServer,
} = require('./support.cjs');

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

    assert.equal(shareURL, roomURL);
    assert.equal(await ada.page.locator('#room-state').textContent(), 'Waiting');
    assert.match(await ada.page.locator('#seats').textContent(), /Ada/);
    assert.match(await ada.page.locator('#seats').textContent(), /Open seat/);

    await ada.page.getByRole('button', { name: 'Copy link' }).click();
    await ada.page.getByText('Room link copied.').waitFor();
    assert.equal(await ada.page.evaluate(() => navigator.clipboard.readText()), shareURL);

    const grace = await newPlayer(browser, baseURL, 'Grace');
    await grace.page.goto(`${baseURL}/`);
    await grace.page.getByLabel('Open a room link').fill(shareURL);
    await grace.page.getByRole('button', { name: 'Open room' }).click();
    await grace.page.waitForURL(roomURL);
    await grace.page.getByRole('button', { name: /Take seat/ }).click();
    await grace.page.locator('#room-notice').waitFor();
    assert.match(await grace.page.locator('#room-notice').textContent(), /You're in seat 2/);
    assert.equal(await grace.page.locator('#room-state').textContent(), 'Ready');

    await ada.page
      .locator('#seats .seat')
      .nth(1)
      .getByText('Grace', { exact: true })
      .waitFor({ timeout: 10000 });
    assert.equal(await ada.page.locator('#room-state').textContent(), 'Ready');

    const desktopLayout = await inspectLayout(ada.page);
    assert.equal(desktopLayout.seatColumns, 2);
    assert.equal(desktopLayout.roomColumns, 2);
    assertNoHorizontalScroll(desktopLayout, 'desktop room');

    const noah = await newPlayer(browser, baseURL, 'Noah');
    await freezeRoomUpdates(noah.page);
    await noah.page.goto(shareURL);
    await noah.page.getByRole('button', { name: 'Take seat 3' }).waitFor();

    const mia = await newPlayer(browser, baseURL, 'Mia');
    await freezeRoomUpdates(mia.page);
    await mia.page.goto(shareURL);
    await mia.page.getByRole('button', { name: 'Take seat 3' }).waitFor();

    const lin = await newPlayer(browser, baseURL, 'Lin');
    await lin.page.goto(shareURL);
    await lin.page.getByRole('button', { name: /Take seat/ }).click();
    await lin.page.locator('#room-notice').waitFor();

    const sam = await newPlayer(browser, baseURL, 'Sam');
    await sam.page.goto(shareURL);
    await sam.page.getByRole('button', { name: /Take seat/ }).click();
    await sam.page.locator('#room-notice').waitFor();

    await ada.page
      .locator('#seats .seat')
      .nth(3)
      .getByText('Sam', { exact: true })
      .waitFor({ timeout: 10000 });

    await noah.page.getByRole('button', { name: /Take seat/ }).click();
    await noah.page.locator('#room-notice').waitFor();
    assert.match(await noah.page.locator('#room-notice').textContent(), /full/i);

    const start = await ada.context.request.post(`${baseURL}/api/rooms/${roomID}/start`);
    assert.equal(start.status(), 200);
    assert.equal((await start.json()).state, 'in_progress');

    await mia.page.getByRole('button', { name: /Take seat/ }).click();
    await mia.page.locator('#room-notice').waitFor();
    assert.match(await mia.page.locator('#room-notice').textContent(), /already started/i);

    await ada.page.locator('#game-view').waitFor({ timeout: 10000 });

    await grace.page.reload();
    await grace.page.locator('#game-view').waitFor({ timeout: 10000 });
    assert.match(await grace.page.locator('#game-seats').textContent(), /Grace/);

    const bad = await newPlayer(browser, baseURL, 'Bad');
    await bad.page.goto(`${baseURL}/room/aaaaaaaaaa`);
    await bad.page.getByRole('heading', { name: /couldn.t open that room/i }).waitFor();
    assert.match(await bad.page.locator('.error-panel').textContent(), /couldn't find that room/i);

    await bad.page.setViewportSize({ width: 390, height: 844 });
    await bad.page.goto(`${baseURL}/`);
    await bad.page.getByRole('button', { name: 'Create a room' }).waitFor();
    const phoneLanding = await inspectLayout(bad.page);
    assertNoHorizontalScroll(phoneLanding, 'phone landing');
    assert.deepEqual(phoneLanding.undersized, []);
    assert.ok(phoneLanding.bodyFontSize >= 16, `phone body font is ${phoneLanding.bodyFontSize}px`);

    await bad.page.getByRole('button', { name: 'Create a room' }).click();
    await bad.page.waitForURL(/\/room\/[a-z0-9]+$/);
    await bad.page.locator('#room-state').waitFor();
    const phoneRoom = await inspectLayout(bad.page);
    assertNoHorizontalScroll(phoneRoom, 'phone room');
    assert.deepEqual(phoneRoom.undersized, []);
    assert.equal(phoneRoom.seatColumns, 1);
    assert.equal(phoneRoom.roomColumns, 1);

    console.log('e2e: lobby flow passed');
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
