'use strict';

const assert = require('node:assert/strict');
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const readline = require('node:readline');

const { chromium } = require('playwright');

const SERVER_BIN =
  process.env.COVERLETTER_BIN || path.join(__dirname, '..', 'coverletter-e2e');

function startServer() {
  if (!fs.existsSync(SERVER_BIN)) {
    throw new Error(
      `missing server binary at ${SERVER_BIN}; build it with ` +
        '`go build -o /tmp/coverletter-e2e ./cmd/server` and set COVERLETTER_BIN',
    );
  }

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'coverletter-e2e-'));
  const child = spawn(
    SERVER_BIN,
    ['-addr', '127.0.0.1:0', '-db', path.join(dir, 'test.db'), '-open=false'],
    { stdio: ['ignore', 'pipe', 'pipe'] },
  );

  let stderr = '';
  child.stderr.setEncoding('utf8');
  child.stderr.on('data', (chunk) => {
    stderr += chunk;
  });

  const url = new Promise((resolve, reject) => {
    const lines = readline.createInterface({ input: child.stdout });
    const timer = setTimeout(() => {
      reject(new Error(`server did not print a URL; stderr: ${stderr}`));
    }, 10000);

    child.once('error', (err) => {
      clearTimeout(timer);
      reject(err);
    });
    child.once('exit', (code, signal) => {
      clearTimeout(timer);
      reject(new Error(`server exited before ready (${code || signal}); stderr: ${stderr}`));
    });
    lines.on('line', (line) => {
      const value = line.trim();
      if (value.startsWith('http://') || value.startsWith('https://')) {
        clearTimeout(timer);
        resolve(value);
      }
    });
  });

  return { child, dir, url, stderr: () => stderr };
}

async function stopServer(server) {
  if (server.child.exitCode === null) {
    server.child.kill('SIGTERM');
    await new Promise((resolve) => {
      const timer = setTimeout(() => {
        server.child.kill('SIGKILL');
        resolve();
      }, 3000);
      server.child.once('exit', () => {
        clearTimeout(timer);
        resolve();
      });
    });
  }
  fs.rmSync(server.dir, { recursive: true, force: true });
}

async function newPlayer(browser, baseURL, name) {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  await context.grantPermissions(['clipboard-read', 'clipboard-write'], { origin: baseURL });
  const page = await context.newPage();
  page.on('pageerror', (err) => {
    console.error(`page error for ${name}:`, err);
  });

  await page.goto(`${baseURL}/`);
  await page.getByLabel(/display name/i).fill(name);
  await page.getByRole('button', { name: 'Save name' }).click();
  await page.getByRole('button', { name: 'Change name' }).waitFor();

  return { context, page };
}

// Let the first room fetch through, then freeze later polling so the page keeps
// the state it saw. This is how a real stale tab behaves when a seat is taken or
// the host starts the game.
async function freezeRoomUpdates(page) {
  let roomGets = 0;
  await page.route('**/api/rooms/*', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.continue();
      return;
    }
    roomGets += 1;
    if (roomGets === 1) {
      await route.continue();
    } else {
      await route.abort('failed');
    }
  });
}

async function inspectLayout(page) {
  return page.evaluate(() => {
    const visible = (element) => {
      const style = getComputedStyle(element);
      const rect = element.getBoundingClientRect();
      return style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0 && rect.height > 0;
    };

    const controls = [...document.querySelectorAll('button, input, a.button')]
      .filter(visible)
      .map((element) => {
        const rect = element.getBoundingClientRect();
        return {
          tag: element.tagName,
          label: (element.textContent || element.getAttribute('aria-label') || '').trim().slice(0, 40),
          width: Math.round(rect.width),
          height: Math.round(rect.height),
        };
      });

    const columns = (selector) => {
      const element = document.querySelector(selector);
      return element
        ? getComputedStyle(element).gridTemplateColumns.split(' ').filter(Boolean).length
        : null;
    };

    return {
      innerWidth: window.innerWidth,
      scrollWidth: Math.max(document.documentElement.scrollWidth, document.body.scrollWidth),
      bodyFontSize: parseFloat(getComputedStyle(document.body).fontSize),
      controls,
      undersized: controls.filter((control) => control.height < 44 || control.width < 44),
      seatColumns: columns('#seats'),
      roomColumns: columns('.room-layout'),
    };
  });
}

function assertNoHorizontalScroll(layout, label) {
  assert.ok(
    layout.scrollWidth <= layout.innerWidth + 1,
    `${label}: content is ${layout.scrollWidth}px wide in a ${layout.innerWidth}px viewport`,
  );
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

    await ada.page.locator('#room-state').filter({ hasText: 'In progress' }).waitFor({ timeout: 10000 });

    await grace.page.reload();
    await grace.page.locator('#room-state').filter({ hasText: 'In progress' }).waitFor();
    assert.match(await grace.page.locator('#room-actions').textContent(), /You're seated/i);

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

    await bad.page.goto(roomURL);
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
