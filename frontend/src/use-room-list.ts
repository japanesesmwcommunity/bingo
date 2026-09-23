import { useEffect, useState } from 'react';
import { api } from './api';
import { SYNC_INTERVAL_MS } from './sync-config';
import type { RoomSummary } from './types';

export function useRoomList() {
  const [rooms, setRooms] = useState<RoomSummary[] | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    let disposed = false;
    let pending = false;

    async function refresh() {
      if (pending) return;
      pending = true;
      try {
        const data = await api<{ rooms: RoomSummary[] }>('/api/rooms');
        if (!disposed) {
          setRooms(data.rooms);
          setError('');
        }
      } catch {
        if (!disposed) setError('ルーム一覧を取得できません。');
      } finally {
        pending = false;
      }
    }

    void refresh();
    const timer = window.setInterval(() => void refresh(), SYNC_INTERVAL_MS);
    return () => {
      disposed = true;
      window.clearInterval(timer);
    };
  }, []);

  return { rooms, error };
}
