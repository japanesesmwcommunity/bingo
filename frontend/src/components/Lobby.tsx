import type { FormEvent } from 'react';
import { MODE_LABELS } from '../mode-config';
import type { RoomController } from '../use-room';
import { RoomDirectory } from './RoomDirectory';

type LobbyProps = Pick<
  RoomController,
  'create' | 'join' | 'pending' | 'inviteRoomId' | 'lobbyView'
> & {
  hidden: boolean;
};

export function Lobby({ create, join, pending, inviteRoomId, lobbyView, hidden }: LobbyProps) {
  function submit(handler: (form: FormData) => Promise<boolean>) {
    return async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      const form = event.currentTarget;
      if (await handler(new FormData(form))) {
        const passphrase = form.elements.namedItem('passphrase');
        if (passphrase instanceof HTMLInputElement) passphrase.value = '';
      }
    };
  }

  return (
    <section id="lobby" hidden={hidden}>
      {!hidden && lobbyView === 'list' && <RoomDirectory />}
      <div className="room-form-page" hidden={lobbyView === 'list'}>
        <a href="#" className="back-link">
          ← ルーム一覧に戻る
        </a>
        <form
          id="create-form"
          className="panel"
          hidden={lobbyView !== 'create'}
          onSubmit={submit(create)}
        >
          <h2>ルームをつくる</h2>
          <label>
            ルーム名
            <input name="name" maxLength={80} required placeholder="今夜のSMWレース" />
          </label>
          <label>
            合言葉
            <input
              name="passphrase"
              type="text"
              maxLength={128}
              required
              autoComplete="off"
              autoCapitalize="none"
              spellCheck={false}
            />
            <small>参加する人だけに共有してください。</small>
          </label>
          <div className="pair">
            <label>
              対戦形式
              <input type="hidden" name="rule" value="line" />
              <select name="mode">
                <option value="race">{MODE_LABELS.race}</option>
                <option value="lockout">{MODE_LABELS.lockout}</option>
              </select>
            </label>
          </div>
          <details>
            <summary>カード生成の設定</summary>
            <label>
              シード
              <input name="seed" maxLength={200} placeholder="空欄なら自動生成" />
            </label>
            <label>
              レース全体の目標上限（分）
              <input name="maxTime" type="number" min="1" max="1440" defaultValue="90" required />
            </label>
            <small>
              ビンゴ1ラインを揃えるまでの時間を目安にカードを生成します。自動の勝利判定や制限時間ではありません。既存の時間・経路は実測前のため暫定見積もりです。指定時間内の完走を保証するものではありません。
            </small>
          </details>
          <button type="submit" disabled={pending} className="primary">
            ルームを作成
          </button>
        </form>
        <form
          id="join-form"
          className="panel"
          key={inviteRoomId}
          hidden={lobbyView !== 'join'}
          onSubmit={submit(join)}
        >
          <h2>ルームに参加する</h2>
          <label>
            ルームID
            <input
              defaultValue={inviteRoomId}
              name="roomId"
              required
              placeholder="招待されたルームID"
            />
          </label>
          <label>
            あなたの表示名
            <input name="playerName" maxLength={40} required autoComplete="nickname" />
          </label>
          <p className="muted">定員4人。色は赤・青・黄・緑から自動で割り当てられます。</p>
          <label>
            合言葉
            <input
              name="passphrase"
              type="text"
              maxLength={128}
              required
              autoComplete="off"
              autoCapitalize="none"
              spellCheck={false}
            />
          </label>
          <button type="submit" disabled={pending}>
            ルームに参加
          </button>
          <p className="muted">
            同じブラウザなら再読み込み後も復帰できます。別の人として参加する場合は、別のブラウザや端末を使ってください。
          </p>
        </form>
      </div>
    </section>
  );
}
