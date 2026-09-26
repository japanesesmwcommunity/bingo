export type Mode = 'race' | 'lockout';
export type Rule = 'standard' | 'line';
export type LobbyView = 'list' | 'create' | 'join';

export interface RoomSummary {
  id: string;
  name: string;
  mode: Mode;
  rule: Rule;
  playerCount: number;
  maxPlayers: number;
  status: 'waiting' | 'playing';
}

export interface FinishRoute {
  name: string;
  timeMin: number;
  routeAreas?: string[];
}

export interface Goal {
  disabledReason?: string;
  conflictGroups?: string[];
  name: string;
  world: string;
  level: string;
  /** ゲーム開始から達成までの最低所要分。ステージ到達・準備を含み、リスク補正前。 */
  timeMin: number;
  exec: number;
  risk: number;
  tags: string[];
  routeAreas?: string[];
}

export interface Player {
  id: string;
  name: string;
  color: string;
  progress: boolean[];
}

export interface Room {
  id: string;
  version: number;
  name: string;
  ownerId: string;
  mode: Mode;
  card: {
    goals: Goal[];
    seed: string;
    routeAreaTimes?: Record<string, number>;
    bowserRoutes?: FinishRoute[];
  };
  players: Player[];
  options: { maxTime: number; minTarget: number; baseRoute: number; rule: Rule };
  finishedAt: string | null;
  estimatedMinutes: number;
}

export interface RoomSnapshot {
  room: Room;
  playerId: string;
}

export type RoomEvent = ({ type: 'room' } & RoomSnapshot) | { type: 'error'; error: string };

export interface ConnectionOptions {
  onSnapshot: (snapshot: RoomSnapshot) => void;
  onUnavailable: (message: string) => void;
  WebSocketImpl?: typeof WebSocket;
  baseURL?: string;
  now?: () => number;
}
