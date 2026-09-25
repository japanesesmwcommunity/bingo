import assert from 'node:assert/strict';
import { readFile, mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { BingoBoard } from '../src/components/BingoBoard.tsx';

const { chromium } = await import(pathToFileURL(resolve(process.env.PLAYWRIGHT_MODULE)).href);
const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.BROWSER_EXECUTABLE,
});
try {
  const page = await browser.newPage({ viewport: { width: 900, height: 780 } });
  const room = {
    mode: 'race',
    startedAt: 'now',
    players: ['#ed5b64', '#59b7e8', '#f3ca52', '#70cc88'].map((color, index) => ({
      id: String(index),
      name: `プレイヤー${index + 1}`,
      color,
      progress: Array.from({ length: 25 }, (_, cell) => index < cell % 5),
    })),
    card: {
      goals: Array.from({ length: 25 }, (_, index) => ({
        name: `${index % 5}人が達成したマス`,
        world: '',
        level: '',
        timeMin: 1,
        exec: 1,
        risk: 1,
      })),
    },
  };
  const css = await readFile(new URL('../style.css', import.meta.url), 'utf8');
  await page.setContent(
    `<style>${css}</style><main>${renderToStaticMarkup(React.createElement(BingoBoard, { room, playerId: 'spectator', pending: false, onToggle() {} }))}</main>`,
  );
  const styles = await page.locator('.cell').evaluateAll((cells) =>
    cells.slice(0, 5).map((cell) => ({
      background: getComputedStyle(cell).backgroundImage,
      color: getComputedStyle(cell).backgroundColor,
    })),
  );
  assert.equal(styles[0].background, 'none');
  assert.equal(styles[1].color, 'rgb(237, 91, 100)');
  assert.match(styles[2].background, /linear-gradient\(135deg/);
  assert.match(styles[3].background, /conic-gradient/);
  assert.match(styles[4].background, /conic-gradient/);
  await page.locator('[data-cell="2"]').hover();
  assert.equal(
    await page
      .locator('[data-cell="2"]')
      .evaluate((cell) => getComputedStyle(cell).backgroundImage),
    styles[2].background,
  );
  await mkdir('tmp/browser-results', { recursive: true });
  await page.screenshot({ path: 'tmp/browser-results/player-cell-colors.png' });
  await page.setViewportSize({ width: 390, height: 844 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
  console.log('0〜4人の色分け・ホバー維持・モバイル表示を確認しました。');
} finally {
  await browser.close();
}
