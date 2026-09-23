import type { Player } from '../types';

interface PlayerListProps {
  players: Player[];
  playerId: string;
  ownerId: string;
  finished: boolean;
  pending: boolean;
  onKick: (playerId: string) => void;
}

export function PlayerList({
  players,
  playerId,
  ownerId,
  finished,
  pending,
  onKick,
}: PlayerListProps) {
  return (
    <aside className="panel">
      <h2>
        プレイヤー <span id="player-count">{players.length} / 4</span>
      </h2>
      <div id="players">
        {players.map((player) => (
          <div className="player" key={player.id} data-player-id={player.id}>
            <strong>
              <span className="swatch" style={{ backgroundColor: player.color }} />
              {player.name}
              {player.id === playerId ? '（あなた）' : ''}
              {player.id === ownerId ? ' · 主催' : ''}
            </strong>
            <p>
              {player.progress.filter(Boolean).length} / 25 マス ·{' '}
              {player.hasLine ? 'ビンゴ！' : '挑戦中'}
              {player.bowserDefeated ? ' · クッパ撃破済' : ''}
            </p>
            {playerId === ownerId && player.id !== ownerId && !finished && (
              <button
                type="button"
                className="kick-player"
                aria-label={`${player.name}をキック`}
                disabled={pending}
                onClick={() => onKick(player.id)}
              >
                キック
              </button>
            )}
          </div>
        ))}
      </div>
    </aside>
  );
}
