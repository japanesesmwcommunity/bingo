import { useCallback, useEffect, useRef, useState } from 'react';
import { api, ApiError } from './api';
import {
  cellState,
  createPayload,
  lobbyViewFromHash,
  roomIdFromHash,
  shouldApplySnapshot,
} from './model';
import { RoomConnection } from './room-sync';
import { SYNC_INTERVAL_MS } from './sync-config';
import type { LobbyView, Room, RoomSnapshot } from './types';

export function useRoom() {
  const [snapshot, setSnapshot] = useState<RoomSnapshot | null>(null);
  const [pending, setPending] = useState(false);
  const [notice, setNotice] = useState('');
  const [inviteCopied, setInviteCopied] = useState(false);
  const [inviteRoomId, setInviteRoomId] = useState('');
  const [lobbyView, setLobbyView] = useState<LobbyView>('list');
  const connection = useRef<RoomConnection | null>(null);

  // Async callbacks read the latest room and revision, including before React
  // commits the next render. This prevents stale requests from restoring a room.
  const active = useRef({
    roomId: '',
    snapshot: null as RoomSnapshot | null,
    revision: 0,
    pending: false,
    polling: false,
  });

  const acceptSnapshot = useCallback((data: RoomSnapshot) => {
    const session = active.current;
    if (session.roomId !== data.room.id) return;
    if (!shouldApplySnapshot(session.snapshot?.room ?? null, data.room)) return;
    session.snapshot = data;
    setSnapshot(data);
  }, []);

  const reset = useCallback((clearHash = true) => {
    connection.current?.stop();
    active.current.revision++;
    active.current.roomId = '';
    active.current.snapshot = null;
    setSnapshot(null);
    setInviteCopied(false);
    if (clearHash) {
      history.replaceState(null, '', '/');
      setInviteRoomId('');
      setLobbyView('list');
    }
  }, []);

  const enter = useCallback(
    (data: RoomSnapshot) => {
      active.current.roomId = data.room.id;
      acceptSnapshot(data);
      history.replaceState(null, '', `#room=${data.room.id}`);
      connection.current?.connect(data.room.id);
    },
    [acceptSnapshot],
  );

  const refresh = useCallback(async () => {
    const session = active.current;
    if (!session.roomId || session.pending || session.polling) return;
    const requestedRoom = session.roomId;
    const requestedRevision = session.revision;
    session.polling = true;
    try {
      const data = await api<RoomSnapshot>(`/api/rooms/${requestedRoom}/session`);
      if (session.roomId !== requestedRoom || session.revision !== requestedRevision) return;
      enter(data);
    } catch (error) {
      if (session.roomId !== requestedRoom || session.revision !== requestedRevision) return;
      if (error instanceof ApiError && [401, 403, 404].includes(error.status)) {
        const wasInGame = Boolean(session.snapshot);
        reset(false);
        if (wasInGame || error.status !== 401) setNotice(error.message);
      }
    } finally {
      session.polling = false;
    }
  }, [enter, reset]);

  useEffect(() => {
    const socket = new RoomConnection({
      onSnapshot: acceptSnapshot,
      onUnavailable(message) {
        reset();
        setNotice(message);
      },
    });
    connection.current = socket;

    function restore() {
      reset(false);
      setNotice('');
      const id = roomIdFromHash(location.hash);
      setLobbyView(lobbyViewFromHash(location.hash));
      setInviteRoomId(id);
      active.current.roomId = id;
      void refresh();
    }

    const timer = window.setInterval(() => {
      const { roomId } = active.current;
      if (!roomId || socket.isLive()) return;
      if (active.current.snapshot) socket.connect(roomId);
      void refresh();
    }, SYNC_INTERVAL_MS);
    window.addEventListener('hashchange', restore);
    restore();
    return () => {
      window.clearInterval(timer);
      window.removeEventListener('hashchange', restore);
      socket.stop();
      connection.current = null;
      active.current.revision++;
    };
  }, [acceptSnapshot, refresh, reset]);

  async function runAction<T>(
    operation: () => Promise<T>,
    onSuccess: (data: T) => void,
  ): Promise<boolean> {
    const session = active.current;
    if (session.pending) return false;
    session.pending = true;
    const revision = ++session.revision;
    setPending(true);
    setNotice('');
    try {
      const data = await operation();
      if (session.revision !== revision) return false;
      onSuccess(data);
      return true;
    } catch (error) {
      if (session.revision === revision)
        setNotice(error instanceof Error ? error.message : String(error));
      return false;
    } finally {
      session.pending = false;
      setPending(false);
    }
  }

  function create(form: FormData) {
    return runAction(() => api<RoomSnapshot>('/api/rooms', 'POST', createPayload(form)), enter);
  }

  function join(form: FormData) {
    const id = String(form.get('roomId') ?? '').trim();
    const { roomId: _roomId, ...body } = Object.fromEntries(form);
    return runAction(
      () => api<RoomSnapshot>(`/api/rooms/${encodeURIComponent(id)}/join`, 'POST', body),
      enter,
    );
  }

  function update(path: string, method: string, body: unknown) {
    const current = active.current.snapshot;
    if (!current) return;
    return runAction(
      () => api<Room>(`/api/rooms/${current.room.id}/${path}`, method, body),
      (room) => acceptSnapshot({ room, playerId: current.playerId }),
    );
  }

  function toggleCell(index: number) {
    const current = active.current.snapshot;
    if (!current) return;
    const state = cellState(current.room, current.playerId, index);
    if (!state.disabled) void update('progress', 'PUT', { index, completed: !state.completed });
  }

  function toggleBowser() {
    const current = active.current.snapshot;
    const mine = current?.room.players.find((player) => player.id === current.playerId);
    if (mine) void update('bowser', 'PUT', { completed: !mine.bowserDefeated });
  }

  function leave() {
    return runAction(
      () => api<void>(`/api/rooms/${active.current.roomId}/leave`, 'POST', {}),
      () => reset(),
    );
  }

  function deleteRoom() {
    if (confirm('ルームと対戦結果を削除しますか？')) {
      void runAction(
        () => api<void>(`/api/rooms/${active.current.roomId}`, 'DELETE'),
        () => reset(),
      );
    }
  }

  async function copyInvite() {
    const link = `${location.origin}/#room=${active.current.roomId}`;
    setNotice('');
    try {
      await navigator.clipboard.writeText(link);
      setInviteCopied(true);
    } catch {
      setInviteCopied(false);
      setNotice(`このURLを共有してください:\n${link}`);
    }
  }

  return {
    snapshot,
    pending,
    notice,
    inviteCopied,
    inviteRoomId,
    lobbyView,
    create,
    join,
    toggleCell,
    toggleBowser,
    leave,
    deleteRoom,
    copyInvite,
    start: () => update('start', 'POST', {}),
    finish: () => update('finish', 'POST', {}),
    withdraw: () => update('leave', 'POST', {}),
    kick: (playerId: string) =>
      update(`players/${encodeURIComponent(playerId)}`, 'DELETE', undefined),
    regenerate: () => update('card', 'POST', { seed: '' }),
  };
}

export type RoomController = ReturnType<typeof useRoom>;
