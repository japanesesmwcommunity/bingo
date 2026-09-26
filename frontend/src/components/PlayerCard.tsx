import type { RoomController } from '../use-room';
import { BingoBoard } from './BingoBoard';

export function PlayerCard({ controller }: { controller: RoomController }) {
  const { snapshot, notice, pending, toggleCell } = controller;
  const mine = snapshot?.room.players.some((player) => player.id === snapshot.playerId);
  return (
    <main className="card-popup">
      {notice && (
        <div role="alert" className="admin-error">
          {notice}
        </div>
      )}
      {snapshot && mine ? (
        <BingoBoard
          room={snapshot.room}
          playerId={snapshot.playerId}
          pending={pending}
          onToggle={toggleCell}
        />
      ) : (
        <p role="status">カードを表示するには、元の画面でプレイヤーとして参加してください。</p>
      )}
    </main>
  );
}
