import test from 'node:test';
import assert from 'node:assert/strict';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { GameRoom } from '../src/components/GameRoom.tsx';
import { Lobby } from '../src/components/Lobby.tsx';
import { cellState } from '../src/model.ts';

const room = {
  id: 'room',
  ownerId: 'owner',
  name: 'test',
  mode: 'race',
  finishedAt: null,
  options: { rule: 'line', maxTime: 90 },
  estimatedMinutes: 30,
  card: {
    seed: 'test',
    goals: Array.from({ length: 25 }, () => ({ name: 'お題', world: '', level: '' })),
  },
  players: [{ id: 'owner', name: '主催者', color: '#123456', progress: Array(25).fill(true) }],
};

test('completed cards remain editable without victory or Bowser controls', () => {
  const html = renderToStaticMarkup(
    React.createElement(GameRoom, {
      snapshot: { room, playerId: 'owner' },
      controller: { pending: false },
    }),
  );
  assert.doesNotMatch(html, /id="bowser"|クッパ撃破|勝利|ビンゴ！/);
  assert.match(html, /25 \/ 25 マス/);
  assert.equal(cellState(room, 'owner', 0).disabled, false);
  assert.equal(cellState({ ...room, finishedAt: 'done' }, 'owner', 0).disabled, true);
});

test('creation form has no victory condition selector', () => {
  const html = renderToStaticMarkup(
    React.createElement(Lobby, {
      lobbyView: 'create',
      inviteRoomId: '',
      pending: false,
      hidden: false,
    }),
  );
  assert.doesNotMatch(html, /勝利条件|<select name="rule"/);
  assert.match(html, /name="rule" value="line"/);
});

test('guest and organizer join forms use automatic colors', () => {
  const lobby = renderToStaticMarkup(
    React.createElement(Lobby, {
      lobbyView: 'join',
      inviteRoomId: 'room',
      pending: false,
      hidden: false,
    }),
  );
  const organizer = renderToStaticMarkup(
    React.createElement(GameRoom, {
      snapshot: { room: { ...room, players: [] }, playerId: 'owner' },
      controller: { pending: false },
    }),
  );
  for (const html of [lobby, organizer]) {
    assert.doesNotMatch(html, /name="color"|type="color"/);
    assert.match(html, /定員4人/);
    assert.match(html, /赤・青・黄・緑/);
  }
});
