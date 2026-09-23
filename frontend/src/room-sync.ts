import { RECONNECT_DELAY_MS, SOCKET_TIMEOUT_MS } from './sync-config';
import type { ConnectionOptions, RoomEvent } from './types';

// Connection lifecycle is kept separate from DOM rendering and HTTP updates.
export class RoomConnection {
  private readonly onSnapshot: ConnectionOptions['onSnapshot'];
  private readonly onUnavailable: ConnectionOptions['onUnavailable'];
  private readonly WebSocketImpl: typeof WebSocket;
  private readonly baseURL: string | undefined;
  private readonly now: () => number;
  private socket: WebSocket | null;
  private roomId: string;
  private lastMessageAt: number;
  private retryAt: number;

  constructor({
    onSnapshot,
    onUnavailable,
    WebSocketImpl = globalThis.WebSocket,
    baseURL = globalThis.location?.href,
    now = Date.now,
  }: ConnectionOptions) {
    this.onSnapshot = onSnapshot;
    this.onUnavailable = onUnavailable;
    this.WebSocketImpl = WebSocketImpl;
    this.baseURL = baseURL;
    this.now = now;
    this.socket = null;
    this.roomId = '';
    this.lastMessageAt = 0;
    this.retryAt = 0;
  }

  connect(roomId: string): void {
    if (this.roomId !== roomId) {
      this.stop();
      this.roomId = roomId;
    }

    const now = this.now();
    if (now < this.retryAt) return;
    if (
      this.socket &&
      this.socket.readyState <= 1 &&
      now - this.lastMessageAt < SOCKET_TIMEOUT_MS
    ) {
      return;
    }

    // Retire the old connection before closing it so its callbacks cannot
    // change the state of a newer connection or a different room.
    const previous = this.socket;
    this.socket = null;
    previous?.close();

    const url = new URL(`/api/rooms/${encodeURIComponent(roomId)}/events`, this.baseURL);
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';

    let socket;
    try {
      socket = new this.WebSocketImpl(url.href);
    } catch {
      this.retryAt = now + RECONNECT_DELAY_MS;
      return;
    }
    this.socket = socket;
    this.lastMessageAt = now;

    socket.onopen = () => {
      if (this.socket !== socket) return;
      this.lastMessageAt = this.now();
    };

    socket.onmessage = (event) => {
      if (this.socket !== socket) return;
      let message: RoomEvent;
      try {
        message = JSON.parse(event.data);
      } catch {
        socket.close();
        return;
      }
      if (message?.type === 'error') {
        this.stop();
        this.onUnavailable(message.error);
        return;
      }
      if (message?.type !== 'room' || message.room?.id !== this.roomId || !message.playerId) {
        return;
      }
      this.lastMessageAt = this.now();
      this.onSnapshot(message);
    };

    socket.onerror = () => {
      if (this.socket === socket) socket.close();
    };

    socket.onclose = () => {
      if (this.socket !== socket) return;
      this.socket = null;
      this.retryAt = this.now() + RECONNECT_DELAY_MS;
    };
  }

  isLive(): boolean {
    return Boolean(
      this.socket?.readyState === 1 && this.now() - this.lastMessageAt < SOCKET_TIMEOUT_MS,
    );
  }

  stop(): void {
    const previous = this.socket;
    this.socket = null;
    this.roomId = '';
    this.retryAt = 0;
    previous?.close();
  }
}
