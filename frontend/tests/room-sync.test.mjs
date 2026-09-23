import test from 'node:test';
import assert from 'node:assert/strict';
import { RoomConnection } from '../src/room-sync.ts';
import { SYNC_INTERVAL_MS, SOCKET_TIMEOUT_MS, RECONNECT_DELAY_MS } from '../src/sync-config.ts';

function fixture(options = {}) {
  let clock = 100;
  const sockets = [];
  const snapshots = [];
  const errors = [];

  class FakeSocket {
    constructor(url) {
      this.url = url;
      this.readyState = 0;
      sockets.push(this);
    }

    open() {
      this.readyState = 1;
      this.onopen?.();
    }

    receive(value) {
      this.onmessage?.({ data: JSON.stringify(value) });
    }

    close() {
      this.readyState = 3;
      this.onclose?.();
    }
  }

  const connection = new RoomConnection({
    WebSocketImpl: FakeSocket,
    baseURL: 'http://localhost:8080/',
    now: () => clock,
    onSnapshot: (snapshot) => snapshots.push(snapshot),
    onUnavailable: (error) => errors.push(error),
    ...options,
  });

  return {
    connection,
    sockets,
    snapshots,
    errors,
    advance(milliseconds) {
      clock += milliseconds;
    },
  };
}

test('sync cadence is one second and snapshots are delivered over the socket', () => {
  assert.equal(SYNC_INTERVAL_MS, 1000);
  const f = fixture();
  f.connection.connect('room-a');
  assert.equal(f.sockets[0].url, 'ws://localhost:8080/api/rooms/room-a/events');
  assert.equal(f.connection.isLive(), false);
  f.connection.connect('room-a');
  assert.equal(f.sockets.length, 1);
  f.sockets[0].open();
  assert.equal(f.connection.isLive(), true);
  const message = { type: 'room', room: { id: 'room-a', version: 2 }, playerId: 'player' };
  f.sockets[0].receive(message);
  assert.deepEqual(f.snapshots, [message]);
  f.connection.stop();
  assert.equal(f.connection.isLive(), false);
  assert.equal(f.sockets[0].readyState, 3);
});

test('disconnects enable fallback and retry after one second', () => {
  const f = fixture();
  f.connection.connect('room');
  f.sockets[0].open();
  f.sockets[0].onerror();
  assert.equal(f.connection.isLive(), false);
  f.connection.connect('room');
  assert.equal(f.sockets.length, 1);
  f.advance(RECONNECT_DELAY_MS);
  f.connection.connect('room');
  assert.equal(f.sockets.length, 2);
  f.sockets[1].open();
  assert.equal(f.connection.isLive(), true);
});

test('stalled handshakes and silent open connections are replaced', () => {
  for (const open of [false, true]) {
    const f = fixture();
    f.connection.connect('room');
    if (open) f.sockets[0].open();
    f.advance(SOCKET_TIMEOUT_MS);
    assert.equal(f.connection.isLive(), false);
    f.connection.connect('room');
    assert.equal(f.sockets.length, 2);
    assert.equal(f.sockets[0].readyState, 3);
  }
});

test('messages renew liveness and unrelated data does not', () => {
  const f = fixture();
  f.connection.connect('room');
  f.sockets[0].open();
  f.advance(SOCKET_TIMEOUT_MS - 1);
  f.sockets[0].receive({ type: 'room', room: { id: 'room' }, playerId: 'p' });
  f.advance(100);
  assert.equal(f.connection.isLive(), true);
  for (const message of [
    null,
    {},
    { type: 'room', room: { id: 'other' }, playerId: 'p' },
    { type: 'room', room: { id: 'room' } },
  ]) {
    f.sockets[0].receive(message);
  }
  assert.equal(f.snapshots.length, 1);
  f.advance(SOCKET_TIMEOUT_MS);
  assert.equal(f.connection.isLive(), false);
});

test('callbacks from a previous room cannot alter the new connection', () => {
  const f = fixture({ baseURL: 'https://bingo.example/' });
  f.connection.connect('old');
  const old = f.sockets[0];
  assert.equal(old.url, 'wss://bingo.example/api/rooms/old/events');
  f.connection.connect('new');
  const current = f.sockets[1];
  current.open();
  old.open();
  old.receive({ type: 'room', room: { id: 'old' }, playerId: 'p' });
  old.onerror();
  old.close();
  assert.equal(f.connection.isLive(), true);
  assert.equal(f.connection.socket, current);
  assert.equal(f.snapshots.length, 0);
});

test('invalid data falls back and revoked access stops the connection', () => {
  const f = fixture();
  f.connection.connect('room');
  f.sockets[0].open();
  f.sockets[0].onmessage({ data: '{' });
  assert.equal(f.connection.isLive(), false);
  f.advance(RECONNECT_DELAY_MS);
  f.connection.connect('room');
  f.sockets[1].open();
  f.sockets[1].receive({ type: 'error', error: 'session expired' });
  assert.equal(f.connection.isLive(), false);
  assert.equal(f.connection.roomId, '');
  assert.deepEqual(f.errors, ['session expired']);
});

test('an unavailable websocket implementation keeps HTTP fallback usable', () => {
  const f = fixture({ WebSocketImpl: null });
  f.connection.connect('room');
  assert.equal(f.connection.isLive(), false);
  assert.equal(f.sockets.length, 0);
  f.connection.connect('room');
  f.connection.stop();
});
