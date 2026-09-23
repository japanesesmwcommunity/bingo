import test from 'node:test';
import assert from 'node:assert/strict';
import { api, ApiError } from '../src/api.ts';

test('API requests send JSON and the same-origin request header', async (t) => {
  const fetch = t.mock.method(globalThis, 'fetch', async () => Response.json({ version: 3 }));
  assert.deepEqual(await api('/api/rooms/r/progress', 'PUT', { index: 0, completed: true }), {
    version: 3,
  });
  assert.deepEqual(fetch.mock.calls[0].arguments, [
    '/api/rooms/r/progress',
    {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'bingo' },
      body: '{"index":0,"completed":true}',
    },
  ]);
  await api('/api/rooms/r');
  assert.equal(fetch.mock.calls[1].arguments[1].method, 'GET');
  assert.equal('body' in fetch.mock.calls[1].arguments[1], false);
});

test('API handles empty success, server errors and network failures', async (t) => {
  const fetch = t.mock.method(globalThis, 'fetch', async () => new Response(null, { status: 204 }));
  assert.equal(await api('/api/rooms/r', 'DELETE'), undefined);
  fetch.mock.mockImplementation(async () =>
    Response.json({ error: '参加してください' }, { status: 401 }),
  );
  await assert.rejects(
    api('/api/rooms/r'),
    (error) =>
      error instanceof ApiError && error.status === 401 && error.message === '参加してください',
  );
  fetch.mock.mockImplementation(async () => Response.json({}, { status: 503 }));
  await assert.rejects(api('/api/rooms/r'), /HTTP 503/);
  fetch.mock.mockImplementation(async () => {
    throw new TypeError('network unavailable');
  });
  await assert.rejects(api('/api/rooms/r'), /network unavailable/);
});
