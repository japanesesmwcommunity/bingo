import { MODE_LABELS } from '../mode-config';
import type { RoomSnapshot } from '../types';
import type { RoomController } from '../use-room';
import { BingoBoard } from './BingoBoard';
import { PlayerList } from './PlayerList';

interface GameRoomProps {
  snapshot: RoomSnapshot;
  controller: RoomController;
}

export function GameRoom({ snapshot: { room, playerId }, controller }: GameRoomProps) {
  const { pending } = controller;
  const mine = room.players.find((player) => player.id === playerId);
  const owner = room.ownerId === playerId;
  const playing = !room.finishedAt;
  const hasProgress = room.players.some((player) => player.progress.some(Boolean));

  return (
    <section id="game">
      <div className="game-heading">
        <div>
          <p className="eyebrow" id="mode">
            {MODE_LABELS[room.mode]}
          </p>
          <h1 id="room-name">{room.name}</h1>
          {owner && <p className="muted">管理者{mine ? '・プレイヤーとして参加中' : ''}</p>}
        </div>
        <div className="clock">
          <span id="phase">{room.finishedAt ? '終了' : 'プレイ中'}</span>
        </div>
      </div>
      <div className="toolbar">
        <button id="invite" type="button" onClick={controller.copyInvite}>
          {controller.inviteCopied ? 'コピーしました' : '招待URLをコピー'}
        </button>
        <button
          id="finish"
          type="button"
          hidden={!owner || !playing}
          disabled={pending}
          onClick={controller.finish}
        >
          ゲーム終了
        </button>
        <button
          id="regenerate"
          type="button"
          hidden={!owner || !playing || hasProgress}
          disabled={pending}
          onClick={controller.regenerate}
        >
          カードを再生成
        </button>
        <button
          id="withdraw"
          type="button"
          hidden={!owner || !mine || Boolean(room.finishedAt)}
          disabled={pending}
          onClick={controller.withdraw}
        >
          参加を取り消す
        </button>
        <button
          id="leave"
          type="button"
          hidden={owner || Boolean(room.finishedAt)}
          disabled={pending}
          onClick={controller.leave}
        >
          退出
        </button>
        <button
          id="delete-room"
          type="button"
          hidden={!owner}
          disabled={pending}
          onClick={controller.deleteRoom}
        >
          ルームを削除
        </button>
      </div>
      {owner && !mine && !room.finishedAt && (
        <details id="owner-participation">
          <summary>プレイヤーとして参加</summary>
          <form
            id="owner-join-form"
            className="panel"
            onSubmit={(event) => {
              event.preventDefault();
              const form = new FormData(event.currentTarget);
              form.set('roomId', room.id);
              void controller.join(form);
            }}
          >
            <label>
              表示名
              <input name="playerName" maxLength={40} required autoComplete="nickname" />
            </label>
            <p className="muted">定員4人。色は赤・青・黄・緑から自動で割り当てられます。</p>
            <button type="submit" disabled={pending || room.players.length >= 4}>
              {room.players.length >= 4 ? '満員' : 'プレイヤーとして参加'}
            </button>
          </form>
        </details>
      )}
      <p id="room-info" className="muted">
        ルームID: {room.id} · レース全体の目標上限 {room.options.maxTime} 分 · 暫定見積もり{' '}
        {room.estimatedMinutes.toFixed(1)} 分
      </p>
      <p id="seed-info" className="muted">
        シード: {room.card.seed}
      </p>
      <div id="finished" role="status" hidden={!room.finishedAt}>
        主催者がゲームを終了しました。
      </div>
      <div className="play-area">
        <div>
          <BingoBoard
            room={room}
            playerId={playerId}
            pending={pending}
            onToggle={controller.toggleCell}
          />
          <p className="muted" hidden={!mine}>
            自分が達成したマスをクリック。もう一度押すと取り消せます。ゲーム終了後は変更できません。
          </p>
        </div>
        <PlayerList
          players={room.players}
          playerId={playerId}
          ownerId={room.ownerId}
          finished={Boolean(room.finishedAt)}
          pending={pending}
          onKick={controller.kick}
        />
      </div>
    </section>
  );
}
