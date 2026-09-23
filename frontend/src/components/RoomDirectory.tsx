import { useRoomList } from '../use-room-list';
import { MODE_LABELS } from '../mode-config';

export function RoomDirectory() {
  const { rooms, error } = useRoomList();
  return (
    <section id="room-directory" aria-labelledby="directory-title">
      <div className="directory-heading">
        <div>
          <h1 id="directory-title">アクティブなルーム</h1>
          <p className="muted">
            {rooms === null ? 'ルームを読み込んでいます…' : `${rooms.length}件のルーム`}
          </p>
        </div>
        <button
          id="open-create"
          className="primary"
          type="button"
          onClick={() => {
            location.hash = 'create';
          }}
        >
          ルーム作成
        </button>
      </div>
      {error && (
        <p className="directory-error" role="alert">
          {error}
        </p>
      )}
      {rooms?.length === 0 && !error && (
        <div className="directory-empty panel">
          <p>アクティブなルームはありません。</p>
        </div>
      )}
      {rooms && rooms.length > 0 && (
        <ul className="room-list">
          {rooms.map((room) => (
            <li className="room-row" key={room.id} data-room-id={room.id}>
              <div className="room-description">
                <h2>{room.name}</h2>
                <p className="muted">{MODE_LABELS[room.mode]}</p>
              </div>
              <span className={`room-phase ${room.status}`}>
                {room.status === 'playing'
                  ? '対戦中'
                  : room.playerCount >= room.maxPlayers
                    ? '満員'
                    : '参加受付中'}
              </span>
              <span className="room-capacity">
                {room.playerCount} / {room.maxPlayers} 人
              </span>
              <a className="room-link" href={`#room=${room.id}`}>
                ルームを開く
              </a>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
