import test, { after } from 'node:test';
import assert from 'node:assert/strict';
import { readFile, mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { ROUTE_SEGMENTS, RETIRED_ROUTE_AREAS, routeSegmentLabel } from '../src/route-config.ts';
const segmentIds = ROUTE_SEGMENTS.map((segment) => segment.id);
const completeTimes = Object.fromEntries(segmentIds.map((name) => [name, 1]));

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
after(() => browser.close());

async function adminPage(t) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1100 } });
  t.after(() => context.close());
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', (error) => errors.push(error.message));
  t.after(() => assert.deepEqual(errors, []));
  return page;
}

test('admin page stays locked when Discord is not configured', async (t) => {
  const page = await adminPage(t);
  await page.goto(base + '/admin');
  await page
    .getByText('管理者ログインは未設定です。サーバーのDiscord認証設定を追加してください。')
    .waitFor();
  assert.equal(await page.locator('#goal-editor').count(), 0);
  assert.equal((await page.request.get(base + '/api/admin/goals')).status(), 401);
  await page.route('**/api/admin/session', (route) =>
    route.fulfill({ json: { configured: true, user: null } }),
  );
  await page.goto(base + '/admin?error=denied');
  await page.getByRole('alert').waitFor();
  assert.match(await page.getByRole('alert').textContent(), /管理権限がありません/);
  assert.equal(
    await page.getByRole('link', { name: 'Discordでログイン', exact: true }).getAttribute('href'),
    '/api/admin/login',
  );
});

test('admin editor searches edits adds removes saves and logs out', async (t) => {
  const page = await adminPage(t);
  let user = { id: '234567890123456789', username: '管理者' };
  const catalog = JSON.parse(
    await readFile(new URL('../../backend/bingo.json', import.meta.url), 'utf8'),
  );
  catalog.goals = catalog.goals.map((goal) => ({
    ...goal,
    routeAreas: (goal.routeAreas ?? []).filter((area) => !RETIRED_ROUTE_AREAS.includes(area)),
  }));
  let document = {
    ...catalog,
    routeAreaTimes: { ...completeTimes, ...catalog.routeAreaTimes },
    revision: 'first-version',
  };
  const count = document.goals.length;
  const writes = [];
  await page.route('**/api/admin/session', (route) =>
    route.fulfill({ json: { configured: true, user } }),
  );
  await page.route('**/api/admin/logout', (route) => {
    user = null;
    return route.fulfill({ status: 204 });
  });
  await page.route('**/api/admin/goals', async (route) => {
    if (route.request().method() === 'PUT') {
      const submitted = route.request().postDataJSON();
      assert.equal(submitted.revision, document.revision);
      assert.equal('key' in submitted.goals[0], false);
      writes.push(submitted);
      document = { ...submitted, revision: `saved-${writes.length}` };
    }
    await route.fulfill({ json: document });
  });
  await page.goto(base + '/admin');
  await page.locator('[name=goal-name]').waitFor();
  const save = page.getByRole('button', { name: '変更を保存', exact: true });
  assert.equal(await save.isDisabled(), true);
  await page.getByRole('button', { name: 'お題を追加', exact: true }).click();
  assert.equal(await save.isDisabled(), true);
  await page.locator('[name=goal-name]').fill('編集テストのお題');
  await page.locator('[name=goal-world]').fill('ヨースターとう');
  await page.locator('[name=goal-level]').fill('コース4');
  await page.locator('[name=goal-time]').fill('7');
  await page.locator('[name=goal-exec]').selectOption('2');
  await page.locator('[name=goal-risk]').selectOption('3');
  await page.getByText('配置バランスのタグ', { exact: true }).click();
  await page.locator('[name=goal-areas]').fill('yoster');
  await page.locator('[name=goal-kinds]').fill('clear, coin');
  await page.getByRole('searchbox').fill('編集テスト');
  assert.equal(await page.locator('.admin-goal-list li').count(), 1);
  await save.click();
  await page.getByRole('status').waitFor();
  assert.equal(writes[0].goals.length, count + 1);
  assert.deepEqual(writes[0].goals.at(-1), {
    name: '編集テストのお題',
    world: 'ヨースターとう',
    level: 'コース4',
    timeMin: 7,
    exec: 2,
    risk: 3,
    tags: ['area:yoster', 'type:clear', 'type:coin'],
  });
  assert.equal(await save.isDisabled(), true);
  await mkdir('tmp/browser-results', { recursive: true });
  await page.screenshot({ path: 'tmp/browser-results/admin-editor.png', fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
  await page.screenshot({ path: 'tmp/browser-results/admin-editor-mobile.png', fullPage: true });
  await page.getByRole('button', { name: 'このお題を削除', exact: true }).click();
  await save.click();
  await page.waitForFunction(
    () => document.querySelector('.admin-toolbar').textContent.includes('未保存') === false,
  );
  assert.equal(document.goals.length, count);
  await page.reload();
  await page.locator('[name=goal-name]').waitFor();
  assert.equal(await page.locator('.admin-goal-list li').count(), count);
  await page.getByRole('button', { name: 'ログアウト', exact: true }).click();
  await page.getByRole('link', { name: 'Discordでログイン', exact: true }).waitFor();
});

test('all area times are shown upfront and goal routes remain selectable', async (t) => {
  const page = await adminPage(t);
  let document = {
    ...JSON.parse(await readFile(new URL('../../backend/bingo.json', import.meta.url), 'utf8')),
    routeAreaTimes: { 独自区間: 2 },
    revision: 'routes-0',
  };
  document.goals = document.goals.map((goal) => ({ ...goal, routeAreas: [] }));
  let saves = 0;
  await page.route('**/api/admin/session', (route) =>
    route.fulfill({
      json: {
        configured: true,
        user: { id: '234567890123456789', username: 'admin' },
      },
    }),
  );
  await page.route('**/api/admin/goals', (route) => {
    if (route.request().method() === 'PUT') {
      const data = route.request().postDataJSON();
      assert.equal(data.revision, document.revision);
      document = { ...data, revision: `routes-${++saves}` };
    }
    return route.fulfill({ json: document });
  });
  await page.goto(base + '/admin');
  const save = page.getByRole('button', { name: '変更を保存', exact: true });
  const timeFor = (name) =>
    page.getByRole('spinbutton', {
      name: `${routeSegmentLabel(name)}の基準時間（分）`,
      exact: true,
    });
  const routes = page.locator('[name=goal-route-add]');
  await routes.waitFor();
  assert.equal(await page.locator('.route-area-row').count(), segmentIds.length + 1);
  assert.equal(await page.locator('[name=route-area-add], .route-area-editor button').count(), 0);
  assert.equal(await timeFor('独自区間').inputValue(), '2');
  assert.equal(await timeFor('ヨースターとう').inputValue(), '');
  assert.equal(await save.isDisabled(), true);
  assert.equal(await page.getByRole('alert').count(), 0);
  assert.equal(await routes.locator('option[value="vanilla:plateau"]').count(), 1);
  await routes.selectOption('ヨースターとう');
  await routes.selectOption('donut:main');
  assert.equal(await routes.locator('option[value="ヨースターとう"]').count(), 0);
  assert.equal(await save.isDisabled(), true);
  assert.match(await page.getByRole('alert').textContent(), /未設定/);
  await page.locator('[name=goal-time]').fill('10');
  for (const name of segmentIds) await timeFor(name).fill('1');
  await timeFor('ヨースターとう').fill('4');
  await timeFor('donut:main').fill('3');
  await save.click();
  await page.getByRole('status').waitFor();
  assert.deepEqual(document.routeAreaTimes, {
    ...completeTimes,
    ヨースターとう: 4,
    'donut:main': 3,
    独自区間: 2,
  });
  assert.deepEqual(document.goals[0].routeAreas, ['ヨースターとう', 'donut:main']);
  assert.equal(await save.isDisabled(), true);
  await timeFor('vanilla:plateau').fill('');
  assert.equal(await timeFor('vanilla:plateau').inputValue(), '');
  assert.equal(await save.isDisabled(), false);
  assert.equal(await page.getByRole('alert').count(), 0);
  await timeFor('vanilla:plateau').fill('1');
  assert.equal(await save.isDisabled(), true);
  assert.equal(await page.getByRole('alert').count(), 0);
  await timeFor('ヨースターとう').fill('8');
  assert.equal(await save.isDisabled(), true);
  assert.match(await page.getByRole('alert').textContent(), /超えています/);
  await timeFor('ヨースターとう').fill('5');
  await save.click();
  await page.getByRole('status').waitFor();
  await page.reload();
  await routes.waitFor();
  assert.equal(await save.isDisabled(), true);
  assert.equal(await timeFor('ヨースターとう').inputValue(), '5');
  assert.deepEqual(await page.locator('.selected-route-areas li span').allTextContents(), [
    'ヨースターとう',
    routeSegmentLabel('donut:main'),
  ]);
  await page.setViewportSize({ width: 390, height: 844 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
  await mkdir('tmp/browser-results', { recursive: true });
  await page.screenshot({
    path: 'tmp/browser-results/admin-fixed-areas-mobile.png',
    fullPage: true,
  });
  await page
    .getByRole('button', {
      name: `${routeSegmentLabel('donut:main')}を経由区間から外す`,
      exact: true,
    })
    .click();
  await save.click();
  await page.getByRole('status').waitFor();
  assert.equal(document.routeAreaTimes['donut:main'], 3);
  assert.deepEqual(document.goals[0].routeAreas, ['ヨースターとう']);
  await routes.selectOption('donut:main');
  await page
    .getByRole('button', {
      name: `${routeSegmentLabel('donut:main')}を経由区間から外す`,
      exact: true,
    })
    .click();
  assert.equal(await save.isDisabled(), true);
});
test('conflicts and expired login preserve drafts and failed loads explain the problem', async (t) => {
  const page = await adminPage(t);
  const catalog = JSON.parse(
    await readFile(new URL('../../backend/bingo.json', import.meta.url), 'utf8'),
  );
  catalog.goals = catalog.goals.map((goal) => ({
    ...goal,
    routeAreas: (goal.routeAreas ?? []).filter((area) => !RETIRED_ROUTE_AREAS.includes(area)),
  }));
  const document = {
    ...catalog,
    routeAreaTimes: { ...completeTimes, ...catalog.routeAreaTimes },
    revision: 'base',
  };
  let status = 409;
  await page.route('**/api/admin/session', (route) =>
    route.fulfill({
      json: { configured: true, user: { id: '234567890123456789', username: 'admin' } },
    }),
  );
  await page.route('**/api/admin/goals', (route) =>
    route.request().method() === 'GET'
      ? route.fulfill({ json: document })
      : route.fulfill({
          status,
          json: {
            error:
              status === 409
                ? '別の編集が保存されています。'
                : '管理者としてDiscordでログインしてください',
          },
        }),
  );
  await page.goto(base + '/admin');
  await page.locator('[name=goal-name]').fill('保存に失敗しても残す内容');
  await page.getByRole('button', { name: '変更を保存', exact: true }).click();
  await page.getByRole('alert').waitFor();
  assert.equal(await page.locator('[name=goal-name]').inputValue(), '保存に失敗しても残す内容');
  assert.equal(
    await page.getByRole('button', { name: '変更を保存', exact: true }).isEnabled(),
    true,
  );
  status = 401;
  await page.getByRole('button', { name: '変更を保存', exact: true }).click();
  await page.getByRole('link', { name: 'Discordで再ログイン' }).waitFor();
  assert.equal(await page.locator('[name=goal-name]').inputValue(), '保存に失敗しても残す内容');
  assert.equal(
    await page.getByRole('button', { name: '変更を保存', exact: true }).isDisabled(),
    true,
  );
  page.once('dialog', (dialog) => dialog.dismiss());
  await page.getByRole('link', { name: 'SMW Bingo', exact: true }).click();
  assert.equal(new URL(page.url()).pathname, '/admin');
  await page.unroute('**/api/admin/goals');
  await page.route('**/api/admin/goals', (route) =>
    route.fulfill({ status: 500, json: { error: 'お題ファイルを読み込めません。' } }),
  );
  page.once('dialog', (dialog) => dialog.accept());
  await page.reload();
  await page.getByRole('alert').waitFor();
  assert.equal(await page.locator('[name=goal-name]').count(), 0);
});

test('legacy area totals remain as references while branch routes can be selected independently', async (t) => {
  const page = await adminPage(t);
  let document = {
    ...JSON.parse(await readFile(new URL('../../backend/bingo.json', import.meta.url), 'utf8')),
    routeAreaTimes: { ...completeTimes, バニラドーム: 10, バニラだいち: 5 },
    revision: 'branch-0',
  };
  document.goals = document.goals.map((goal) => ({ ...goal, routeAreas: [] }));
  document.goals[0].disabledReason = '';
  document.goals[0].timeMin = 60;
  document.goals[0].routeAreas = ['バニラドーム', 'バニラだいち'];
  await page.route('**/api/admin/session', (route) =>
    route.fulfill({
      json: {
        configured: true,
        user: { id: '234567890123456789', username: 'admin' },
      },
    }),
  );
  await page.route('**/api/admin/goals', (route) => {
    if (route.request().method() === 'PUT')
      document = { ...route.request().postDataJSON(), revision: 'branch-1' };
    return route.fulfill({ json: document });
  });
  await page.goto(base + '/admin');
  const routes = page.locator('[name=goal-route-add]');
  const save = page.getByRole('button', { name: '変更を保存', exact: true });
  await routes.waitFor();
  assert.match(await page.locator('.legacy-route-times').textContent(), /バニラドーム：10分/);
  assert.match(await page.locator('.legacy-route-times').textContent(), /バニラだいち：5分/);
  assert.equal(await page.locator('.legacy-route-times input').count(), 0);
  assert.equal(await routes.locator('option[value="バニラドーム"]').count(), 0);
  assert.match(await page.locator('.selected-route-areas').textContent(), /旧エリア・要再設定/);
  await routes.selectOption('vanilla:common');
  await routes.selectOption('vanilla:cheese');
  assert.equal(await save.isDisabled(), true);
  assert.match(await page.getByRole('alert').textContent(), /選び直して/);
  for (const name of ['バニラドーム', 'バニラだいち'])
    await page.getByRole('button', { name: `${name}を経由区間から外す`, exact: true }).click();
  await routes.selectOption('vanilla:plateau');
  await routes.selectOption('plateau:butter');
  await save.click();
  await page.getByRole('status').waitFor();
  assert.deepEqual(document.goals[0].routeAreas, [
    'vanilla:common',
    'vanilla:cheese',
    'vanilla:plateau',
    'plateau:butter',
  ]);
  assert.equal(document.routeAreaTimes.バニラドーム, 10);
  assert.equal(document.routeAreaTimes.バニラだいち, 5);
  await page.reload();
  await routes.waitFor();
  assert.equal(await save.isDisabled(), true);
  assert.deepEqual(
    await page.locator('.selected-route-areas li span').allTextContents(),
    document.goals[0].routeAreas.map(routeSegmentLabel),
  );
  await page.screenshot({ path: 'tmp/browser-results/admin-branch-segments.png', fullPage: true });
});
