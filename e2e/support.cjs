'use strict';

const assert = require('node:assert/strict');
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const readline = require('node:readline');

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

    const controls = [...document.querySelectorAll('button, input, select, a.button')]
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
      gameColumns: columns('.game-layout'),
      gameSeatColumns: columns('#game-seats'),
    };
  });
}

function assertNoHorizontalScroll(layout, label) {
  assert.ok(
    layout.scrollWidth <= layout.innerWidth + 1,
    `${label}: content is ${layout.scrollWidth}px wide in a ${layout.innerWidth}px viewport`,
  );
}

module.exports = {
  assertNoHorizontalScroll,
  freezeRoomUpdates,
  inspectLayout,
  newPlayer,
  startServer,
  stopServer,
};
