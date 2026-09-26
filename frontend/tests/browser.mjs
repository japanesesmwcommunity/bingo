import test, { after } from 'node:test';
import assert from 'node:assert/strict';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { mkdir } from 'node:fs/promises';
import { EventEmitter, once } from 'node:events';

const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE
    ? pathToFileURL(resolve(process.env.PLAYWRIGHT_MODULE)).href
    : 'playwright'
);
const browser = await chromium.launch({
  headless: true,
  ...(process.env.BROWSER_EXECUTABLE ? { executablePath: process.env.BROWSER_EXECUTABLE } : {}),
});
const base = process.env.BINGO_URL || 'http://127.0.0.1:8080';
after(async () => {
  await browser.close();
});

async function newPlayer(t, mobile = false) {
  const context = await browser.newContext({
    viewport: mobile ? { width: 390, height: 844 } : { width: 1440, height: 1100 },
  });
  t.after(() => context.close());
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', (error) => errors.push(error.message));
  t.after(() => assert.deepEqual(errors, []));
  return page;
}
async function create(page, mode = 'race', rule = 'standard', participate = true) {
  await page.goto(base);
  assert.equal(await page.locator('#create-form').isVisible(), false);
  assert.equal(await page.locator('#join-form').isVisible(), false);
  await page.locator('#open-create').click();
  const form = page.locator('#create-form');
  await form.locator('[name=name]').fill('ブラウザ確認 <script>alert(1)</script>');
  assert.equal(await form.locator('[name=playerName], [name=color]').count(), 0);
  await form.locator('[name=passphrase]').fill('prototype-secret');
  await form.locator('[name=mode]').selectOption(mode);
  assert.equal(
    await form.locator('[name=mode] option:checked').textContent(),
    mode === 'race' ? 'ノーマル' : 'ロックアウト',
  );
  await form.locator('[name=rule]').selectOption(rule);
  assert.equal(await form.locator('input[type=number]').count(), 1);
  assert.equal(await form.locator('[name=baseRoute], [name=minTarget]').count(), 0);
  await form.locator('summary').click();
  await form.locator('[name=maxTime]').fill('60');
  const created = page.waitForResponse(
    (response) => response.url().endsWith('/api/rooms') && response.request().method() === 'POST',
  );
  await form.locator('button[type=submit]').click();
  const response = await created;
  const payload = response.request().postDataJSON();
  assert.equal(payload.maxTime, 60);
  assert.equal('baseRoute' in payload, false);
  assert.equal('minTarget' in payload, false);
  const result = await response.json();
  assert.equal(result.room.players.length, 0);
  assert.equal(result.room.options.maxTime, 60);
  assert.equal(result.room.options.minTarget, 0);
  assert.ok(result.room.estimatedMinutes <= 60);
  for (const goal of result.room.card.goals) {
    assert.equal(typeof goal.world, 'string');
    assert.equal(typeof goal.level, 'string');
  }
  await page.locator('#game').waitFor({ state: 'visible' });
  assert.equal(
    await page.locator('#mode').textContent(),
    mode === 'race' ? 'ノーマル' : 'ロックアウト',
  );
  assert.equal(await page.locator('.cell').count(), 25);
  const firstGoal = result.room.card.goals[0];
  const firstLabel = await page.locator('[data-cell="0"] .goal').textContent();
  assert.ok(firstLabel.endsWith(firstGoal.name));
  if (firstGoal.world) assert.ok(firstLabel.startsWith(firstGoal.world));
  if (firstGoal.level) assert.ok(firstLabel.includes(firstGoal.level));
  assert.equal(await page.locator('[data-cell="0"]').getAttribute('aria-label'), firstLabel);
  assert.equal(await page.locator('#sync-status').count(), 0);
  assert.doesNotMatch(await page.locator('#game').innerText(), /WebSocket|同期中|再接続中/);
  if (participate) {
    await page.locator('#owner-participation summary').click();
    await page.locator('#owner-join-form [name=playerName]').fill('主催者');
    await page.locator('#owner-join-form button[type=submit]').click();
    await page.locator('#owner-join-form').waitFor({ state: 'detached' });
  }
  return page.url();
}
async function join(page, invite, name, passphrase = 'prototype-secret', succeeds = true) {
  await page.goto(invite);
  const form = page.locator('#join-form');
  await form.locator('[name=playerName]').fill(name);
  await form.locator('[name=passphrase]').fill(passphrase);
  await form.locator('button[type=submit]').click();
  await page.locator(succeeds ? '#game' : '#notice').waitFor({ state: 'visible' });
}
async function claimed(page, index, value) {
  await page.waitForFunction(
    ({ index, value }) =>
      document.querySelector(`[data-cell="${index}"]`)?.getAttribute('aria-pressed') ===
      String(value),
    { index, value },
  );
}

test(
  'organizer can manage without participating and can enroll separately',
  { timeout: 60000 },
  async (t) => {
    const host = await newPlayer(t);
    const guest = await newPlayer(t);
    const viewer = await newPlayer(t);
    const remaining = await newPlayer(t);
    const invite = await create(host, 'race', 'standard', false);
    const id = new URL(invite).hash.slice('#room='.length);
    assert.equal(await host.locator('#player-count').textContent(), '0 / 4');
    assert.equal(await host.locator('#start').count(), 0);
    assert.equal(await host.locator('#timer').count(), 0);
    await host.locator('#owner-participation summary').click();
    await host.locator('#owner-join-form [name=playerName]').fill('管理者も参加');
    await host.locator('#owner-join-form button[type=submit]').click();
    await host.locator('#withdraw').waitFor({ state: 'visible' });
    assert.equal(await host.locator('#player-count').textContent(), '1 / 4');
    assert.equal(await host.locator('.kick-player').count(), 0);
    await host.locator('#withdraw').click();
    await host.locator('#owner-participation').waitFor();
    assert.equal(await host.locator('#player-count').textContent(), '0 / 4');
    await host.reload();
    await host.locator('#game').waitFor();
    assert.equal(await host.locator('#player-count').textContent(), '0 / 4');
    await join(guest, invite, '参加者');
    await join(remaining, invite, '残る参加者');
    assert.equal(await guest.locator('.kick-player').count(), 0);
    await viewer.goto(base);
    const row = viewer.locator(`[data-room-id="${id}"]`);
    await row.waitFor();
    assert.equal(await host.locator('#finish').isVisible(), true);
    await host.locator('#finish').waitFor({ state: 'visible' });
    await guest.locator('[data-cell="0"]:enabled').click();
    await claimed(guest, 0, true);
    await host.waitForFunction(() =>
      Boolean(document.querySelector('[data-cell="0"]')?.style.background),
    );
    assert.equal(await host.locator('.cell:enabled').count(), 0);
    assert.equal(await host.locator('#bowser').isVisible(), false);
    assert.equal(await guest.locator('#finish').isVisible(), false);

    await remaining.locator('[data-cell="1"]:enabled').click();
    await claimed(remaining, 1, true);
    await mkdir('tmp/browser-results', { recursive: true });
    await host.screenshot({ path: 'tmp/browser-results/kick-controls.png', fullPage: true });
    await host.route('**/players/*', (route) =>
      route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: '{"error":"再試行してください"}',
      }),
    );
    await host.getByRole('button', { name: '参加者をキック', exact: true }).click();
    await host.locator('#notice').waitFor({ state: 'visible' });
    assert.equal(await host.locator('#player-count').textContent(), '2 / 4');
    await host.unroute('**/players/*');
    await host.getByRole('button', { name: '参加者をキック', exact: true }).click();
    await guest.locator('#room-directory').waitFor();
    for (const page of [host, remaining]) {
      await page.waitForFunction(
        () => document.querySelector('#player-count')?.textContent === '1 / 4',
      );
      assert.equal(
        await page.locator('[data-cell="0"]').evaluate((cell) => cell.style.background),
        '',
      );
    }
    assert.equal(await host.locator('#phase').textContent(), 'プレイ中');

    // A failed finish request must leave the game playable and allow retrying.
    await host.route('**/finish', (route) =>
      route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: '{"error":"再試行してください"}',
      }),
    );
    await host.locator('#finish').click();
    await host.locator('#notice').waitFor({ state: 'visible' });
    assert.equal(await host.locator('#phase').textContent(), 'プレイ中');
    await host.unroute('**/finish');
    await host.locator('#finish:enabled').click();
    for (const page of [host, remaining]) {
      await page.waitForFunction(() => document.querySelector('#phase')?.textContent === '終了');
      assert.equal(
        await page.locator('#winner').textContent(),
        'ホストがゲームを終了しました（勝者なし）',
      );
      assert.equal(await page.locator('.cell:enabled').count(), 0);
      assert.equal(await page.locator('#bowser').isDisabled(), true);
      assert.equal(await page.locator('#finish').isVisible(), false);
      assert.equal(await page.locator('.kick-player').count(), 0);
    }
    await claimed(remaining, 1, true);
    await row.waitFor({ state: 'hidden' });
    await remaining.reload();
    await remaining.locator('#winner').waitFor({ state: 'visible' });
    assert.equal(await remaining.locator('#timer').count(), 0);
    await claimed(remaining, 1, true);
    await mkdir('tmp/browser-results', { recursive: true });
    await host.screenshot({ path: 'tmp/browser-results/manual-finish.png', fullPage: true });
    host.once('dialog', (dialog) => dialog.accept());
    await host.locator('#delete-room').click();
    await host.locator('#room-directory').waitFor();
    await remaining.locator('#room-directory').waitFor();
  },
);

test(
  'home lists active rooms and creation and joining use separate screens',
  { timeout: 60000 },
  async (t) => {
    const host = await newPlayer(t);
    const viewer = await newPlayer(t);
    const third = await newPlayer(t);
    const fourth = await newPlayer(t);
    await viewer.route('**/api/rooms', (route) =>
      route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({ rooms: [] }),
      }),
    );
    await viewer.goto(base);
    await viewer.locator('.directory-empty').waitFor();
    assert.equal(await viewer.locator('#create-form').isVisible(), false);
    assert.equal(await viewer.locator('#join-form').isVisible(), false);
    await viewer.locator('#open-create').click();
    await viewer.locator('#create-form').waitFor({ state: 'visible' });
    assert.equal(await viewer.locator('#room-directory').count(), 0);
    await viewer.locator('.back-link').click();
    await viewer.locator('.directory-empty').waitFor();
    await viewer.unroute('**/api/rooms');
    await viewer.route('**/api/rooms', (route) =>
      route.fulfill({ status: 503, contentType: 'application/json', body: '{}' }),
    );
    await viewer.locator('.directory-error').waitFor();
    await viewer.unroute('**/api/rooms');
    await viewer.locator('.directory-error').waitFor({ state: 'hidden' });

    const invite = await create(host, 'race', 'line');
    const id = new URL(invite).hash.slice('#room='.length);
    const row = viewer.locator(`[data-room-id="${id}"]`);
    await row.waitFor();
    assert.match(await row.locator('h2').textContent(), /<script>/);
    assert.equal(await row.locator('.room-description .muted').textContent(), 'ノーマル');
    assert.equal(await row.locator('.room-capacity').textContent(), '1 / 4 人');
    assert.equal(await row.locator('.room-phase').textContent(), '参加受付中');
    await row.locator('a').click();
    await viewer.locator('#join-form').waitFor({ state: 'visible' });
    assert.equal(await viewer.locator('#join-form [name=roomId]').inputValue(), id);
    await viewer.locator('#join-form [name=playerName]').fill('一覧から参加');
    await viewer.locator('#join-form [name=passphrase]').fill('prototype-secret');
    await viewer.locator('#join-form button[type=submit]').click();
    await viewer.locator('#game').waitFor({ state: 'visible' });
    await viewer.goto(base);
    await join(third, invite, '3人目');
    await join(fourth, invite, '4人目');
    await viewer.waitForFunction(
      (id) => document.querySelector(`[data-room-id="${id}"] .room-phase`)?.textContent === '満員',
      id,
    );
    await mkdir('tmp/browser-results', { recursive: true });
    await viewer.screenshot({ path: 'tmp/browser-results/room-directory.png', fullPage: true });
    await viewer.setViewportSize({ width: 390, height: 844 });
    assert.equal(
      await viewer.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      true,
    );
    await viewer.screenshot({
      path: 'tmp/browser-results/room-directory-mobile.png',
      fullPage: true,
    });

    await viewer.waitForFunction(
      (id) => document.querySelector(`[data-room-id="${id}"] .room-phase`)?.textContent === '満員',
      id,
    );
    await row.locator('a').click();
    await viewer.locator('#game').waitFor({ state: 'visible' });
    await viewer.goto(base);
    for (let index = 0; index < 5; index++) {
      await host.locator(`[data-cell="${index}"]:enabled`).click();
      await claimed(host, index, true);
    }
    await host.locator('#winner').waitFor({ state: 'visible' });
    await row.waitFor({ state: 'hidden' });

    const deletable = await create(host);
    const deletedId = new URL(deletable).hash.slice('#room='.length);
    const deletedRow = viewer.locator(`[data-room-id="${deletedId}"]`);
    await deletedRow.waitFor();
    host.once('dialog', (dialog) => dialog.accept());
    await host.locator('#delete-room').click();
    await host.locator('#room-directory').waitFor();
    await deletedRow.waitFor({ state: 'hidden' });
  },
);

test(
  'four browsers can join, recover, play and finish the standard race',
  { timeout: 90000 },
  async (t) => {
    const host = await newPlayer(t),
      guest = await newPlayer(t),
      third = await newPlayer(t),
      fourth = await newPlayer(t),
      fifth = await newPlayer(t);
    const invite = await create(host);
    assert.equal(await host.locator('.brand img').evaluate((image) => image.naturalWidth), 1024);
    const cellStyle = await host.locator('[data-cell="0"]').evaluate((cell) => {
      const rect = cell.getBoundingClientRect();
      const style = getComputedStyle(cell);
      return {
        width: Math.round(rect.width),
        height: Math.round(rect.height),
        fontSize: style.fontSize,
        radius: style.borderRadius,
      };
    });
    assert.deepEqual(cellStyle, { width: 120, height: 120, fontSize: '14px', radius: '0px' });
    assert.match(await host.locator('#room-name').textContent(), /<script>/);
    const firstSeed = await host.locator('#seed-info').textContent();
    await host.locator('#regenerate').click();
    await host.waitForFunction(
      (seed) => document.querySelector('#seed-info').textContent !== seed,
      firstSeed,
    );
    await join(guest, invite, 'ゲスト', 'wrong', false);
    await guest.locator('#join-form [name=passphrase]').fill('prototype-secret');
    await guest.locator('#join-form button[type=submit]').click();
    await guest.locator('#game').waitFor({ state: 'visible' });
    await join(third, invite, '3人目');
    await join(fourth, invite, '4人目');
    await join(fifth, invite, '5人目', 'prototype-secret', false);
    await host.waitForFunction(
      () => document.querySelector('#player-count').textContent === '4 / 4',
    );
    assert.equal(await guest.locator('#start').isVisible(), false);
    await guest.reload();
    await guest.locator('#game').waitFor({ state: 'visible' });
    assert.equal(await guest.locator('#player-count').textContent(), '4 / 4');
    await host.context().grantPermissions(['clipboard-read', 'clipboard-write']);
    await host.locator('#invite').click();
    await host.waitForFunction(
      async (url) => (await navigator.clipboard.readText()) === url,
      invite,
    );
    assert.equal(await host.evaluate(() => navigator.clipboard.readText()), invite);
    assert.equal(await host.locator('#invite').textContent(), 'コピーしました');
    await host.locator('[data-cell="0"]:enabled').waitFor();
    await host.locator('[data-cell="0"]').focus();
    assert.equal(
      await host.locator('[data-cell="0"]').evaluate((cell) => document.activeElement === cell),
      true,
    );
    await host.locator('[data-cell="0"]').click();
    await claimed(host, 0, true);
    await host.locator('[data-cell="0"]').click();
    await claimed(host, 0, false);
    for (let i = 0; i < 5; i++) {
      await host.locator(`[data-cell="${i}"]`).click();
      await claimed(host, i, true);
    }
    assert.equal(await host.locator('#winner').isVisible(), false);
    assert.equal(
      await guest.locator('[data-cell="0"]').evaluate((cell) => cell.style.background),
      '',
    );
    assert.equal(await guest.locator('[data-cell="0"]').isEnabled(), true);
    await guest.locator('[data-cell="0"]').click();
    await claimed(guest, 0, true);
    await host.locator('#bowser').click();
    await host.locator('#winner').waitFor({ state: 'visible' });
    assert.match(await host.locator('#winner').textContent(), /主催者 の勝利/);
    await guest.locator('#winner').waitFor({ state: 'visible' });
    assert.equal(await guest.locator('[data-cell="6"]').isEnabled(), false);
    await mkdir('tmp/browser-results', { recursive: true });
    await host.screenshot({ path: 'tmp/browser-results/race-finished.png', fullPage: true });
    host.once('dialog', (dialog) => dialog.accept());
    await host.locator('#delete-room').click();
    await host.locator('#lobby').waitFor({ state: 'visible' });
    await guest.locator('#lobby').waitFor({ state: 'visible' });
  },
);

test(
  'failed updates unlock controls and late responses cannot reopen a departed view',
  { timeout: 30000 },
  async (t) => {
    const host = await newPlayer(t);
    const invite = await create(host);
    assert.equal(await host.locator('#create-form [name=passphrase]').inputValue(), '');
    await host.locator('[data-cell="0"]:enabled').waitFor();
    await host.route('**/progress', (route) =>
      route.fulfill({
        status: 409,
        contentType: 'application/json',
        body: JSON.stringify({ error: '更新を確認してください' }),
      }),
    );
    await host.locator('[data-cell="0"]').click();
    await host.waitForFunction(
      () => document.querySelector('#notice').textContent === '更新を確認してください',
    );
    assert.equal(await host.locator('[data-cell="0"]').isEnabled(), true);
    await claimed(host, 0, false);
    await host.unroute('**/progress');

    let release;
    let received;
    const held = new Promise((resolve) => {
      release = resolve;
    });
    const intercepted = new Promise((resolve) => {
      received = resolve;
    });
    await host.route('**/progress', async (route) => {
      const response = await route.fetch();
      received();
      await held;
      await route.fulfill({ response });
    });
    await host.locator('[data-cell="0"]').click();
    await intercepted;
    await host.evaluate(() => {
      location.hash = '';
    });
    await host.locator('#lobby').waitFor({ state: 'visible' });
    const completed = host.waitForResponse((response) => response.url().endsWith('/progress'));
    release();
    await completed;
    await host.waitForFunction(
      () => !document.querySelector('#create-form button[type=submit]').disabled,
    );
    assert.equal(await host.locator('#game').count(), 0);
    await host.unroute('**/progress');
    await host.evaluate((url) => {
      location.hash = new URL(url).hash;
    }, invite);
    await host.locator('#game').waitFor({ state: 'visible' });
    await claimed(host, 0, true);
  },
);

test(
  'lockout excludes other players and the line variant ends at one line',
  { timeout: 60000 },
  async (t) => {
    const host = await newPlayer(t),
      guest = await newPlayer(t, true);
    const invite = await create(host, 'lockout', 'line');
    await join(guest, invite, 'モバイル');
    assert.equal(await guest.locator('#bowser').isVisible(), false);
    assert.equal(
      await guest.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      true,
    );
    await guest.locator('#leave').click();
    await guest.locator('#lobby').waitFor({ state: 'visible' });
    await join(guest, invite, 'モバイル');
    await host.locator('[data-cell="0"]:enabled').waitFor();
    await host.locator('[data-cell="0"]').click();
    await claimed(host, 0, true);
    await guest.locator('[data-cell="0"]:disabled').waitFor();
    assert.equal(
      await guest.locator('[data-cell="0"]').evaluate((cell) => cell.style.background),
      '',
    );
    assert.equal(await guest.locator('[data-cell="0"]').isEnabled(), false);
    await host.locator('[data-cell="0"]').click();
    await claimed(host, 0, false);
    await guest.locator('[data-cell="0"]:enabled').waitFor();
    for (let i = 0; i < 5; i++) {
      await guest.locator(`[data-cell="${i}"]`).click();
      await claimed(guest, i, true);
    }
    await guest.locator('#winner').waitFor({ state: 'visible' });
    assert.match(await guest.locator('#winner').textContent(), /モバイル の勝利/);
    await mkdir('tmp/browser-results', { recursive: true });
    await guest.screenshot({ path: 'tmp/browser-results/mobile-lockout.png', fullPage: true });
  },
);

test(
  'connection failures recover and bowser can be recorded before a line',
  { timeout: 60000 },
  async (t) => {
    const host = await newPlayer(t);
    let blockSockets = false;
    let activeSocket;
    const frames = new EventEmitter();
    await host.routeWebSocket('**/api/rooms/*/events', (socket) => {
      if (blockSockets) {
        void socket.close({ code: 1013, reason: 'connection unavailable' });
      } else {
        activeSocket = socket;
        socket.connectToServer().onMessage((message) => {
          socket.send(message);
          frames.emit('room');
        });
      }
    });
    await create(host);
    const polls = [];
    const isStatusRequest = (request) =>
      request.method() === 'GET' && /\/api\/rooms\/[^/]+$/.test(new URL(request.url()).pathname);
    host.on('request', (request) => {
      if (isStatusRequest(request)) polls.push(Date.now());
    });
    await new Promise((resolve) => setTimeout(resolve, 2200));
    assert.equal(polls.length, 0, 'a healthy websocket must not poll HTTP');
    await host.locator('#bowser:enabled').waitFor();
    await host.locator('#bowser').click();
    await host.waitForFunction(
      () => document.querySelector('#bowser').getAttribute('aria-pressed') === 'true',
    );
    assert.equal(await host.locator('#winner').isVisible(), false);
    await host.locator('#bowser').click();
    await host.waitForFunction(
      () => document.querySelector('#bowser').getAttribute('aria-pressed') === 'false',
    );

    blockSockets = true;
    await activeSocket.close({ code: 1013, reason: 'test disconnect' });
    while (polls.length < 3) await host.waitForRequest(isStatusRequest);
    for (let i = 1; i < 3; i++) {
      const interval = polls[i] - polls[i - 1];
      assert.ok(interval >= 600 && interval <= 1800, `fallback interval: ${interval}ms`);
    }

    const failedPoll = host.waitForEvent('requestfailed', isStatusRequest);
    await host.route('**/api/rooms/*/session', (route) =>
      route.request().method() === 'GET' ? route.abort() : route.continue(),
    );
    await failedPoll;
    assert.doesNotMatch(await host.locator('#game').innerText(), /WebSocket|同期中|再接続中/);
    await host.unroute('**/api/rooms/*/session');
    const reconnected = once(frames, 'room');
    blockSockets = false;
    await reconnected;
    await host.context().clearCookies();
    await host.reload();
    await host.locator('#lobby').waitFor({ state: 'visible' });
  },
);
