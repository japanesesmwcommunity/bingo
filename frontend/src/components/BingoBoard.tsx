import { cellState, formatGoalName } from '../model';
import { cellFill } from '../cell-colors';
import type { Room } from '../types';

interface BingoBoardProps {
  room: Room;
  playerId: string;
  pending: boolean;
  onToggle: (index: number) => void;
}

export function BingoBoard({ room, playerId, pending, onToggle }: BingoBoardProps) {
  const isPlayer = room.players.some((player) => player.id === playerId);
  return (
    <div className="board-scroll">
      <div id="board" className="board" aria-label="ビンゴカード">
        {room.card.goals.map((goal, index) => {
          const state = cellState(room, playerId, index);
          const visibleOwners = isPlayer
            ? state.owners.filter((player) => player.id === playerId)
            : state.owners;
          const label = formatGoalName(goal);
          const className = `cell${state.completed ? ' mine' : visibleOwners.length ? ' claimed' : ''}`;
          return (
            <button
              key={index}
              className={className}
              style={{ background: cellFill(visibleOwners) }}
              type="button"
              data-cell={index}
              disabled={state.disabled || pending}
              aria-pressed={state.completed}
              aria-label={label}
              title={`ゲーム開始から達成まで暫定${goal.timeMin}分（到達・準備を含む） / 操作精度 ${goal.exec} / リスク ${goal.risk}`}
              onClick={() => onToggle(index)}
            >
              <span className="goal">{label}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
