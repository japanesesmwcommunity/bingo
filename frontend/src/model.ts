import type { Goal, LobbyView, Room } from './types';
import { LEVEL_SEPARATORS, WORLD_SEPARATORS } from './stage-config';

export function formatGoalName({
  world,
  level,
  name,
}: Pick<Goal, 'world' | 'level' | 'name'>): string {
  if (!world) return name;
  const separator = WORLD_SEPARATORS[world] ?? LEVEL_SEPARATORS[level] ?? ' ';
  const stage = level ? `${world}${separator}${level}` : world;
  return `${stage} ${name}`;
}

export function formatTime(seconds: number): string {
  const safe = Math.max(0, Math.floor(seconds));
  return `${String(Math.floor(safe / 60)).padStart(2, '0')}:${String(safe % 60).padStart(2, '0')}`;
}

export function roomIdFromHash(hash: string): string {
  const match = /^#room=([0-9a-f-]{36})$/i.exec(hash);
  return match ? match[1] : '';
}

export function lobbyViewFromHash(hash: string): LobbyView {
  if (hash === '#create') return 'create';
  return roomIdFromHash(hash) ? 'join' : 'list';
}

export function cellState(room: Room, playerId: string, index: number) {
  const mine = room.players.find((p) => p.id === playerId);
  const owners = room.players.filter((p) => p.progress[index]);
  return {
    completed: Boolean(mine?.progress[index]),
    owners,
    disabled:
      !mine ||
      !room.startedAt ||
      Boolean(room.finishedAt) ||
      (room.mode === 'lockout' && owners.some((p) => p.id !== playerId)),
  };
}

export function createPayload(form: Iterable<readonly [string, FormDataEntryValue]>) {
  const payload: Record<string, FormDataEntryValue | number> = Object.fromEntries(form);
  payload.maxTime = Number(payload.maxTime);
  return payload;
}

export function shouldApplySnapshot(current: Room | null, incoming: Room): boolean {
  return !current || current.id !== incoming.id || incoming.version >= current.version;
}
