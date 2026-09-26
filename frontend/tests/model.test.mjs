import test from 'node:test';
import assert from 'node:assert/strict';
import {
  roomIdFromHash,
  cellState,
  createPayload,
  shouldApplySnapshot,
  lobbyViewFromHash,
  formatGoalName,
} from '../src/model.ts';

test('goal labels combine stage metadata using LEVEL_LIST conventions', () => {
  for (const [world, level, expected] of [
    ['', '', 'クリアする'],
    ['ヨースターとう', 'コース4', 'ヨースターとう コース4 クリアする'],
    ['ソーダのみずうみ', '', 'ソーダのみずうみ クリアする'],
    ['ドーナツへいや', 'ひみつのコース 1', 'ドーナツへいや ひみつのコース 1 クリアする'],
    ['まよいのもり', 'しろ', 'まよいのもりのしろ クリアする'],
    ['まよいのもり', 'とりで', 'まよいのもりのとりで クリアする'],
    ['かっぱやま', 'きいろスイッチ', 'かっぱやまきいろスイッチ クリアする'],
    ['ドーナツへいや', 'みどりスイッチ', 'ドーナツへいや みどりスイッチ クリアする'],
    ['バニラドーム', 'あかスイッチ', 'バニラドーム あかスイッチ クリアする'],
    ['まよいのもり', 'あおスイッチ', 'まよいのもり あおスイッチ クリアする'],
  ]) {
    assert.equal(formatGoalName({ world, level, name: 'クリアする' }), expected);
  }
});

test('invites accept room UUIDs only', () => {
  const id = '12345678-1234-1234-1234-123456789abc';
  assert.equal(roomIdFromHash(`#room=${id}`), id);
  for (const value of ['', '#room=../x', '#room=<script>', '#other=test'])
    assert.equal(roomIdFromHash(value), '');
});

test('the home page lists rooms and forms have separate routes', () => {
  assert.equal(lobbyViewFromHash(''), 'list');
  assert.equal(lobbyViewFromHash('#'), 'list');
  assert.equal(lobbyViewFromHash('#create'), 'create');
  assert.equal(lobbyViewFromHash('#room=12345678-1234-1234-1234-123456789abc'), 'join');
  assert.equal(lobbyViewFromHash('#room=invalid'), 'list');
});
test('participants can mark immediately; lockout and finished rooms restrict actions', () => {
  const a = { id: 'a', progress: Array(25).fill(false) },
    b = { id: 'b', progress: Array(25).fill(false) };
  const room = { players: [a, b], mode: 'race', finishedAt: null };
  assert.equal(cellState(room, 'a', 0).disabled, false);
  b.progress[0] = true;
  assert.equal(cellState(room, 'a', 0).disabled, false);
  room.mode = 'lockout';
  assert.equal(cellState(room, 'a', 0).disabled, true);
  assert.deepEqual(cellState(room, 'b', 0), { disabled: false, completed: true, owners: [b] });
  assert.equal(cellState(room, 'missing', 0).disabled, true);
  room.finishedAt = 'done';
  assert.equal(cellState(room, 'b', 0).disabled, true);
});
test('form sends one overall time limit without extra time settings', () => {
  const payload = createPayload([
    ['maxTime', '90'],
    ['passphrase', ' secret '],
  ]);
  assert.deepEqual(payload, { maxTime: 90, passphrase: ' secret ' });
  assert.deepEqual(JSON.parse(JSON.stringify(payload)), payload);
});

test('late HTTP responses cannot overwrite newer websocket state', () => {
  const current = { id: 'room-a', version: 8 };
  assert.equal(shouldApplySnapshot(null, current), true);
  assert.equal(shouldApplySnapshot(current, { id: 'room-a', version: 7 }), false);
  assert.equal(shouldApplySnapshot(current, { id: 'room-a', version: 8 }), true);
  assert.equal(shouldApplySnapshot(current, { id: 'room-a', version: 9 }), true);
  assert.equal(shouldApplySnapshot(current, { id: 'room-b', version: 1 }), true);
});
