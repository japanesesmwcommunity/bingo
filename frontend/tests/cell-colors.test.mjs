import test from 'node:test';
import assert from 'node:assert/strict';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { cellFill } from '../src/cell-colors.ts';
import { BingoBoard } from '../src/components/BingoBoard.tsx';

const players = ['#ff0000', '#00ff00', '#0000ff', '#ffff00'].map((color, index) => ({
  id: String(index),
  name: `Player ${index}`,
  color,
  progress: [false],
}));

test('fills use player colors for one, diagonal halves for two and cross quarters for four', () => {
  assert.equal(cellFill([]), undefined);
  assert.equal(cellFill(players.slice(0, 1)), '#ff0000');
  assert.equal(
    cellFill(players.slice(0, 2)),
    'linear-gradient(135deg, #ff0000 0% 50%, #00ff00 50% 100%)',
  );
  assert.equal(
    cellFill(players),
    'conic-gradient(#ff0000 0deg 90deg, #00ff00 90deg 180deg, #0000ff 180deg 270deg, #ffff00 270deg 360deg)',
  );
});

test('three conic regions occupy equal square area, rather than equal angles', () => {
  const fill = cellFill(players.slice(0, 3));
  const stops = [...fill.matchAll(/([\d.]+)deg/g)].map((match) => Number(match[1]));
  assert.equal(stops.length, 6);
  assert.equal(stops[0], 0);
  assert.equal(stops[5], 360);
  assert.equal(stops[1], stops[2]);
  assert.equal(stops[3], stops[4]);
  const rightY = 0.5 - 0.5 / Math.tan((stops[1] * Math.PI) / 180);
  const leftY = 0.5 + 0.5 / Math.tan((stops[3] * Math.PI) / 180);
  // Shoelace areas for the actual square-boundary intersections.
  const polygons = [
    [
      [0.5, 0.5],
      [0.5, 0],
      [1, 0],
      [1, rightY],
    ],
    [
      [0.5, 0.5],
      [1, rightY],
      [1, 1],
      [0, 1],
      [0, leftY],
    ],
    [
      [0.5, 0.5],
      [0, leftY],
      [0, 0],
      [0.5, 0],
    ],
  ];
  for (const polygon of polygons) {
    let twiceArea = 0;
    for (let i = 0; i < polygon.length; i++) {
      const [x, y] = polygon[i];
      const [nextX, nextY] = polygon[(i + 1) % polygon.length];
      twiceArea += x * nextY - nextX * y;
    }
    assert.ok(Math.abs(Math.abs(twiceArea) / 2 - 1 / 3) < 1e-12);
  }
});

test('players see only their own fills while spectators see all owners on marking and undo', () => {
  const room = {
    players: structuredClone(players),
    startedAt: 'now',
    mode: 'race',
    card: { goals: [{ name: 'test goal', world: '', level: '', timeMin: 1, exec: 1, risk: 1 }] },
  };
  for (const count of [0, 1, 2, 3, 4, 3, 2, 1, 0]) {
    room.players.forEach((player, index) => {
      player.progress[0] = index < count;
    });
    for (const playerId of ['0', '3', 'spectator']) {
      let toggled;
      const tree = BingoBoard({
        room,
        playerId,
        pending: false,
        onToggle: (index) => {
          toggled = index;
        },
      });
      const button = tree.props.children.props.children[0];
      const mine = room.players.find((player) => player.id === playerId);
      assert.equal(
        button.props.style.background,
        mine ? (mine.progress[0] ? mine.color : undefined) : cellFill(room.players.slice(0, count)),
      );
      assert.equal(button.props['aria-pressed'], Boolean(mine?.progress[0]));
      if (mine && !mine.progress[0]) assert.equal(button.props.className, 'cell');
      button.props.onClick();
      assert.equal(toggled, 0);
      assert.equal(button.props.disabled, playerId === 'spectator');
    }
  }
  room.players[1].progress[0] = true;
  room.mode = 'lockout';
  const markup = renderToStaticMarkup(
    React.createElement(BingoBoard, { room, playerId: '0', pending: false, onToggle() {} }),
  );
  assert.doesNotMatch(markup, /background:/);
  assert.doesNotMatch(markup, /Player 1|class="markers"|class="marker"/);
  assert.match(markup, /disabled/);
});
