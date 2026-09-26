# 観戦APIとNodeCG連携

観戦APIは認証不要の読み取り専用APIです。ルームIDが分かれば誰でもカード・全員の進捗・勝敗を取得できます。トークンの発行や参加Cookieは不要で、参加枠も使いません。試合終了後も、ルームが削除されるまでは結果を取得できます。

## 読み取りと更新通知

| メソッド | URL | 用途 |
| --- | --- | --- |
| GET | `/api/rooms` | 参加待ち・対戦中のルーム一覧。終了・削除済みは除外 |
| GET | `/api/rooms/{id}/players/{player}` | 指定プレイヤーの進捗を取得 |
| GET | `/api/rooms/{id}` | 最新状態を取得 |
| WebSocket | `/api/rooms/{id}/events` | 接続直後・変更直後・1秒ごとに最新状態全体を受信 |

HTTP取得は `Access-Control-Allow-Origin: *` を返し、観戦WebSocketも任意のOriginから接続できます。NodeCGのextensionとgraphicsのどちらからも利用できます。HTTPは認証ヘッダーやカスタムヘッダーを付けず、Cookieを送信しない通常のGETを使ってください。

```js
const response = await fetch(`${bingoBaseURL}/api/rooms/${roomId}`, {
  credentials: 'omit',
});
if (!response.ok) throw new Error(`HTTP ${response.status}`);
const snapshot = await response.json();
```

一覧は `{ "rooms": [{ "id": "...", "name": "...", "mode": "race", "rule": "standard", "playerCount": 2, "maxPlayers": 4, "status": "waiting" }] }` を返します。`status` はプレイヤー0人なら `waiting`、1人以上なら `playing` で、ルームがなければ `{ "rooms": [] }` です。NodeCGのdashboardではこの一覧から対象のIDを選べます。

ルーム詳細のHTTP取得とWebSocketの正常時の形式は共通です。`room` は参加者が取得するものと同じ内容で、観戦用の省略形式には分けません。

```text
{
  type: "room",
  room: {
    id, version, name, ownerId,
    mode,                  // "race" | "lockout"
    options: { rule, maxTime, minTarget, baseRoute }, // rule: "standard" | "line"
    card: { goals: [{ name, world, level, timeMin, exec, risk, tags, ... }, ...], seed, ... },
    players: [{ id, name, color, progress, hasLine, bowserDefeated }, ...],
    finishedAt,           // ISO 8601形式、未終了ならnull
    estimatedMinutes,
    winnerId              // 勝者なしなら空文字
  }
}
```

`card.goals` は左上から行優先の25件です。各プレイヤーの `progress` は同じ添字の25個の真偽値です。Raceでは同じマスを複数人が達成できます。Lockoutでは1人だけです。参加者の識別には配列位置ではなく `id` を使ってください。名前の表示にはHTML挿入ではなくテキスト描画を使います。

`version` はルームの変更で増加します。開始操作やタイマーはありません。終了・勝者の判定はAPIの値を使います。初回・再接続時には受信した状態全体で置き換えます。

存在しないルーム・削除済みルームではHTTP取得とWebSocket接続時に `404` を返します。購読中にルームが削除されると `{ "type": "error", "error": "ルームが削除されました" }` を送り、WebSocketをコード1008で閉じます。接続数は参加用WebSocketと合計で1ルーム32本までで、超過は `429` です。

観戦APIは読み取り専用です。WebSocketにメッセージを送っても進捗は更新されません。進捗更新・管理操作には引き続き既存の認証と操作権限が必要です。本人の参加状態は `GET /api/rooms/{id}/session` で確認し、このエンドポイントには参加Cookieが必要です。公開情報の `ownerId` やプレイヤーIDだけで操作権限を得ることはできません。

参加画面の同一Origin・Cookie付きWebSocketには本人の `playerId` も含め、参加セッション失効を通知します。外部からの購読は個人の参加セッションに依存せず、ルームが削除されるまで継続します。

## NodeCG側の実装方針

1. extensionがHTTPで取得、またはWebSocketで購読します。接続先はHTTPSなら `wss://`、HTTPなら `ws://` です。必要な設定はビンゴサーバーのURLとルームIDだけです。
2. 受信した `room` をReplicantへ格納し、graphicsはその変更を購読して描画します。進捗や勝敗はビンゴ側で管理します。
3. 通信断では最後の表示を保持しつつ、別の接続状態で更新停止を示します。通常の切断はバックオフ付きで再接続し、再接続直後の全状態で同期します。必要なら切断中だけHTTPポーリングを使います。
4. `404` またはルーム削除のエラーでは再接続を止めます。ルームを切り替えるときは古い購読を閉じ、古い状態をクリアします。

観戦用状態のReplicantは `persistent: false` を推奨します。ビンゴ側のルームがメモリ保存のため、NodeCG再起動時に前の試合の状態だけを復元しないためです。

NodeCGの参考: [Extensions](https://www.nodecg.dev/docs/extensions/)、[Replicant](https://www.nodecg.dev/docs/classes/replicant/)。
