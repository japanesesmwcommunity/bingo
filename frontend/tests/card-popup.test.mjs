import test from 'node:test';
import assert from 'node:assert/strict';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { openCardPopup, CARD_POPUP_FEATURES } from '../src/card-popup.ts';
import { PlayerCard } from '../src/components/PlayerCard.tsx';
import { GameRoom } from '../src/components/GameRoom.tsx';

const room = {
  id: 'room',
  ownerId: 'owner',
  name: 'test',
  mode: 'race',
  options: { maxTime: 90 },
  estimatedMinutes: 30,
  card: { seed: 'test', goals: Array.from({ length: 25 }, () => ({ name: 'お題' })) },
  players: [{ id: 'player', name: 'player', color: '#e71e07', progress: Array(25).fill(false) }],
};

test('opens a reusable room-specific popup and reports blocking', () => {
  let focused = false;
  assert.equal(
    openCardPopup('room', {
      open(...args) {
        assert.deepEqual(args, ['/?view=card#room=room', 'bingo-card-room', CARD_POPUP_FEATURES]);
        return {
          focus() {
            focused = true;
          },
        };
      },
    }),
    true,
  );
  assert.equal(focused, true);
  assert.equal(
    openCardPopup('room', {
      open() {
        return null;
      },
    }),
    false,
  );
});

test('player-only popup renders an editable card without room controls', () => {
  const controller = { snapshot: { room, playerId: 'player' }, pending: false, notice: '' };
  const html = renderToStaticMarkup(React.createElement(PlayerCard, { controller }));
  assert.equal((html.match(/data-cell=/g) ?? []).length, 25);
  assert.doesNotMatch(html, /disabled|<header|<footer|id="players"|id="finish"/);
  const finished = renderToStaticMarkup(
    React.createElement(PlayerCard, {
      controller: {
        ...controller,
        snapshot: { room: { ...room, finishedAt: 'done' }, playerId: 'player' },
      },
    }),
  );
  assert.equal((finished.match(/disabled=/g) ?? []).length, 25);
  for (const snapshot of [null, { room, playerId: 'owner' }]) {
    const unavailable = renderToStaticMarkup(
      React.createElement(PlayerCard, {
        controller: { ...controller, snapshot, notice: '参加権限がありません' },
      }),
    );
    assert.doesNotMatch(unavailable, /data-cell=/);
    assert.match(unavailable, /role="alert"/);
  }
});

test('only enrolled players see the popup button', () => {
  for (const playerId of ['owner', 'player']) {
    const html = renderToStaticMarkup(
      React.createElement(GameRoom, {
        snapshot: { room, playerId },
        controller: { pending: false },
      }),
    );
    assert.equal(html.includes('id="open-card"'), playerId === 'player');
  }
});
