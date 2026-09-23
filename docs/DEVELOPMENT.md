# 開発とAPI

## 作業ディレクトリ

ルートのpnpmコマンドは `frontend/` へ転送します。Goの実行・テストは `backend/` で行います。Viteの出力 `backend/static/` をGoに埋め込みます。区間定義は `backend/bingo/route_segments.json` をフロントから参照し、複製しません。

## コードの書式

フロントエンドとブラウザテストは `.prettierrc.json` に従い、2スペースのインデントと100文字を目安に整形します。CSSは宣言ごと、TypeScriptは処理ごとに改行します。

```sh
pnpm format
pnpm format:check
```

Goコードは `gofmt` を使用します。

## 構成

- `backend/bingo/types.go`: お題・カード・生成条件と配置データ。
- `backend/bingo/bingo.go`: データ読み込み、seed固定の純粋なカード選択、ライン・コスト計算。
- `backend/room/types.go`, `backend/room/player.go`: ルームとプレイヤーのデータ・スナップショット。
- `backend/room/room.go`: 合言葉照合、定員・参加・開始・進捗・勝利の制御。
- `backend/server.go`: JSON API、参加セッション、HTTP入力検証。
- `backend/websocket.go`, `backend/websocket_types.go`: WebSocket接続、ルーム単位の配信と通信設定。
- `frontend/src/types.ts`: ルーム・プレイヤー・カード・通信メッセージの型。
- `frontend/src/model.ts`: 表示と操作可否の純粋な計算。
- `frontend/src/stage-config.ts`: LEVEL_LIST.mdに従うステージ名の連結規則。
- `frontend/src/api.ts`: 型付きHTTPクライアントとAPIエラー。
- `frontend/src/room-sync.ts`, `frontend/src/sync-config.ts`: 再接続処理と1秒同期の設定。
- `frontend/src/use-room.ts`: 参加状態・操作・通信のライフサイクルを管理するReactフック。
- `frontend/src/use-room-list.ts`, `frontend/src/components/RoomDirectory.tsx`: 公開ルーム一覧の取得と表示。トップページ表示中のみ1秒ごとに取得。
- `frontend/src/App.tsx`, `frontend/src/components/`: ロビー、ルーム、ビンゴカード、参加者一覧のReactコンポーネント。
- `frontend/style.css`, `frontend/logo.png`: 共通スタイルとロゴ。
- `frontend/vite.config.ts`: Viteのビルドと開発用API・WebSocketプロキシ。

`pnpm install --frozen-lockfile` の後に `pnpm build` を実行すると、TypeScriptのstrict型チェックを通して `backend/static/` を生成します。Goはこのディレクトリだけを埋め込んで配信します。Goの起動・テスト・ビルド前にフロントをビルドしてください。Dockerでは同じ工程をマルチステージビルドで実行します。

開発時はGoサーバーを8080番で起動し、別ターミナルから `pnpm dev` で5173番のVite開発サーバーを起動します。フロントの変更はホットリロードされ、HTTP APIとWebSocketはGoサーバーへ転送されます。依存バージョンは `pnpm-lock.yaml` で固定しています。

`pnpm test` はTypeScriptのモデル・通信処理を直接検証します。`pnpm test:browser` は起動済みサーバーに対して複数人参加・勝敗・再接続を検証します（Playwrightの指定方法はREADMEを参照）。

ゲーム状態はルーム単位の排他ロックで更新し、参照時にはカードのタグを含めてコピーを返します。ルーム一覧とセッションは別のロックで保護します。お題カタログは検証成功後に置き換え、失敗時は直前の有効データを維持します。

## API

### 管理者ページの設定

`/admin` でお題の検索・編集・追加・削除・一括保存を行えます。ゲームのルーム作成者とは別の権限で、指定したDiscordユーザーだけが利用できます。

Discord Developer Portalでアプリケーションを作成し、OAuth2のRedirectsに `https://公開ホスト/api/admin/callback` を登録します。ローカルでは `http://127.0.0.1:8080/api/admin/callback` を使えます。認可コードフローで `identify` スコープのみを要求し、取得したユーザーIDをサーバー側で照合します。Botの作成・サーバー招待は不要です。[Discord公式OAuth2ドキュメント](https://docs.discord.com/developers/topics/oauth2)

| 環境変数 | 設定内容 |
| --- | --- |
| `DISCORD_CLIENT_ID` | DiscordアプリケーションのClient ID |
| `DISCORD_CLIENT_SECRET` | OAuth2のClient Secret。サーバー専用 |
| `DISCORD_REDIRECT_URI` | Developer Portalに登録したコールバックURLと完全一致させる |
| `ADMIN_DISCORD_USER_IDS` | 許可するDiscordユーザーID。複数の場合はカンマ区切り |
| `SECURE_COOKIE` | HTTPS公開時は `true` |
| `BINGO_DATA` | 編集するお題ファイル。Go単体は `backend/bingo.json`、Dockerは `/app/data/bingo.json` が既定 |

DiscordユーザーIDはDiscordの開発者モードで対象ユーザーからコピーした数値IDです。ユーザー名や表示名は指定できません。

認証の4項目が未設定なら管理ページへのログイン・編集は無効です。一部だけ設定されている場合は起動時に設定エラーとなります。`.env.example` を参考に環境変数を設定してください。Goの実行時に `.env` を自動読み込みする機能はありません。Dockerなら設定済みの `.env` を `--env-file` で渡せます。秘密情報はフロントエンドの `VITE_` 変数には設定しないでください。

```sh
docker build -t smw-bingo:prototype .
docker run --name smw-bingo-admin --env-file .env -p 8080:8080 \
  --mount type=volume,source=smw-bingo-catalog,target=/app/data \
  smw-bingo:prototype
```

名前付きボリュームには初回のみイメージのお題がコピーされ、その後の保存内容はコンテナを作り直しても保持されます。リポジトリの `backend/bingo.json` 自体を編集したい場合は、名前付きボリュームの代わりに、そのファイルを含むディレクトリを `/app/data` にbind mountします。ファイル単体のmountでは、保存時のファイル差し替えができません。

保存すると同じディレクトリの `.bingo-backups/` に変更前のJSONを保存し、一時ファイルの書き込み完了後に本体を差し替えます。保存失敗時の一時ファイルは `.bingo-pending-*` として残ります。バックアップは自動削除しません。復元する場合は選んだバックアップを `BINGO_DATA` のファイルへコピーしてサーバーを再起動します。

保存時には保留を除く25件以上のお題、重複、world/level、時間・難度・リスク、タグを検証します。ゲームの生成条件によっては、保存できるお題でもカードを生成できない場合があります。保存した変更は新規ルーム・開始前のカード再生成から反映され、既存ルームが持つカードは変わりません。

編集開始時のファイルのリビジョンが一致しなければ409で保存を拒否します。競合・通信失敗時は画面の編集中データを保持します。編集中にページを離れるとブラウザの確認が表示されます。単一サーバーでの運用を想定しています。

管理セッションはルーム用Cookieと分離したHttpOnly Cookieで8時間有効です。OAuth stateはブラウザに紐づけた5分間・一回限りの値です。認証コード・Discordのアクセストークン・Client Secretをフロントには返しません。API操作ごとに許可IDを確認し、ログアウトと期限切れで管理セッションを失効させます。

| メソッド | パス | 内容 |
| --- | --- | --- |
| GET | `/api/admin/login` | Discord認証へ移動 |
| GET | `/api/admin/callback` | state検証・コード交換・許可ID照合 |
| GET | `/api/admin/session` | 設定済みかどうかとログインユーザーのみを返す |
| POST | `/api/admin/logout` | 管理セッションを破棄 |
| GET | `/api/admin/goals` | 認証済み管理者へ `{ goals, revision }` を返す |
| PUT | `/api/admin/goals` | `{ goals, revision }` を検証して保存。最大1MiB |

認証テストはDiscordのトークン交換・本人確認の応答を差し替えて、state・許可ID・期限切れを検証します。ブラウザの編集テストも管理APIをモックし、実際のDiscordアカウントやお題ファイルには変更を加えません。ファイル保存・バックアップ・再読込・競合は一時ディレクトリ上のGoテストで検証します。実際のDiscordとの接続確認は上記環境変数を設定して行ってください。

### ゲームAPI

更新系のリクエストには `X-Requested-With: bingo` が必要です。本文がある場合は `Content-Type: application/json` を指定します。未知のフィールド・複数JSON・4KBを超える本文は拒否します。

ルーム一覧・作成・参加以外のルームAPIは作成時または参加時のCookieが必要です。APIの基点は `/api/rooms/{id}` です。`id` はUUIDです。作成・参加・状態取得は `{ "room": ..., "playerId": "..." }` を返します。`playerId` はセッションの本人識別子で、`ownerId` と一致すれば管理者、`players` に存在すればプレイヤーです。管理者は参加せずに認証・閲覧・管理できます。

| メソッド | パス | 動作 |
| --- | --- | --- |
| GET | `/health` | 稼働確認 |
| GET | `/api/config` | 既定の生成条件と定員 |
| GET | `/api/rooms` | アクティブなルーム一覧（認証不要）。終了・削除済みは除外 |
| GET | `/create?seed=...` | 互換の単独カード生成。`maxTime`, `minTarget`, `baseRoute`, `rule` をクエリで指定可能 |
| POST | `/api/rooms` | 管理者としてルーム作成。プレイヤーは0人で、表示名・カラーは不要。`/create` も同じ処理 |
| POST | `/{id}/join` | 合言葉と表示名で参加。既存参加者は復帰。管理者Cookieの場合は合言葉の再入力なしで表示名・カラーを登録し、明示的にプレイヤー参加 |
| GET | `/{id}` | カード・参加者・進捗・開始/終了日時・経過秒・勝者を取得 |
| GET (WebSocket) | `/{id}/events` | 参加Cookieと同一Originで接続し、ルーム状態を受信 |
| GET | `/{id}/players/{player}` | 指定参加者の進捗を取得 |
| DELETE | `/{id}/players/{player}` | 作成者が他プレイヤーをキック。開始前・対戦中のみ。対象の全セッション・進捗・占有を解除し、ルーム状態を返す |
| POST | `/{id}/start` | 主催者が開始。1名から動作確認可能 |
| POST | `/{id}/finish` | 主催者が進行中のゲームを勝者なしで終了。進捗を保持し、タイマーと更新を停止 |
| POST | `/{id}/card` | 主催者が開始前に再生成 |
| PUT | `/{id}/progress` | 自分のマスを更新 |
| PUT | `/{id}/bowser` | 自分のクッパ撃破を更新 |
| POST | `/{id}/leave` | 開始前にプレイヤー参加を取り消す。管理者は権限とCookieを保持しルーム状態を返す（200）。一般参加者はCookieを失効させる（204） |
| DELETE | `/{id}` | 主催者が開始前または終了後にルーム削除 |

表中の `/{id}` は `/api/rooms/{id}` の略記です。

一覧は `{ "rooms": [...] }` を返します。各要素は `id`, `name`, `mode`, `rule`, `playerCount`, `maxPlayers`, `status`（`waiting` / `playing`）のみです。カード・進捗・参加者情報・合言葉は含みません。参加前のルーム情報取得には引き続きCookieが必要です。

トップページは一覧、`#create` は作成フォーム、`#room={id}` は参加フォームまたは参加済みルームの画面です。満員・対戦中のルームも開けますが、新規参加は従来どおり拒否され、参加Cookieのあるブラウザだけが復帰できます。

### WebSocketによる同期

`/api/rooms/{id}/events` は受信専用です。進捗更新は既存のHTTP APIを使い、参加・退出・開始・カード再生成・進捗・勝利をルーム内へ直ちに配信します。初回接続時と1秒ごとにも最新状態を送ります。

```json
{ "type": "room", "room": { "id": "...", "version": 8 }, "playerId": "..." }
```

`room` の実際の内容は状態取得APIと同じです。`version` は変更が成功するたびに増え、画面は古いバージョンの応答を無視します。同じバージョンでも経過秒は更新されます。

セッション失効・退出・ルーム削除時には `{ "type": "error", "error": "説明" }` を送り、接続を閉じます。Originが一致しない接続や未参加の接続は拒否します。接続数は1ルーム32本までです。

画面は受信が3.5秒途絶えた接続を再接続し、その間は1秒ごとのHTTP取得で同期します。HTTPでも参加権限が失われていれば参加画面に戻ります。リバースプロキシではWebSocket Upgradeを転送し、HTTPS終端時は `SECURE_COOKIE=true` を指定してください。

### ルーム作成

```json
{
  "name": "SMWレース",
  "passphrase": "仲間に共有する合言葉",
  "playerName": "マリオ",
  "color": "#52c7a5",
  "seed": "weekend-1",
  "mode": "race",
  "rule": "standard",
  "maxTime": 90
}
```

`name`, `passphrase`, `playerName` が必須です。`mode` は `race` / `lockout`、`rule` は `standard` / `line`。数値の `0` は `minTarget` と `baseRoute` で有効です。`maxTime=0` は既定値90になります。

画面から送る時間設定は `maxTime` のみです。基本ルールではクッパ撃破までを含めたレース全体の上限です。API互換用の `minTarget`（既定0）と `baseRoute`（推定用・既定15）は画面の入力項目にしません。

### 参加

```json
{ "passphrase": "仲間に共有する合言葉", "playerName": "ルイージ", "color": "#8b9dff" }
```

同名参加は不可です。合言葉は前後の空白も含めて照合します。

### カード再生成

```json
{ "seed": "another-seed", "maxTime": 90, "minTarget": 20, "rule": "standard" }
```

省略した条件は現在の値を引き継ぎます。空シードで新規生成します。`baseRoute` はルーム作成時の値を維持します。

### お題データ

`backend/bingo.json` と管理APIのドキュメントは、トップレベルに `routeAreaTimes`（区間名→基準時間）を持ち、お題に `routeAreas`（経由区間名の配列）を指定できます。どちらも省略時は経由補正なしです。区間の時間は1〜1440分の整数で、累計ではなく区間単体の時間です。各お題で区間名は重複不可、未登録の区間は不可、区間の合計時間は `timeMin` 以下とします。名前の空欄・前後空白・カンマは禁止です。

管理画面では `backend/bingo/route_segments.json` の全区間を固定一覧で表示し、未設定は空欄で保持します。参照する区間の基準時間を入力して、お題と一緒に保存します。区間自体の追加・削除操作はありません。既存の独自区間も一覧に引き継ぎます。カードは生成時の `routeAreaTimes` をコピーして保持するので、管理画面で更新しても作成済みルームの推定時間は変わりません。

`LineCosts` は `Σ Cost(goal) − Σ((区間の使用回数 − 1) × 基準時間)` を各ラインについて計算します。重複補正は基準時間だけに適用し、リスク加算分は維持します。配置スコアにもこの補正を適用します。作業自体の同時達成は補正しません。`RaceLineCosts` は任意の `bowserRoutes` を各ラインと組み合わせ、共有解放区間を1回だけ控除し、最小の経路費用を選びます。未登録の場合のみ従来の固定分を加算します。

`timeMin` はゲーム開始からそのお題を単独で達成するまでの最低所要分（整数、1〜1440）です。ステージへの到達・コース解放・必要な準備を含みます。分未満は切り上げ、失敗によるやり直しは含めず `risk` で補正します。この値をそのまま `Cost` の入力とし、配置バランスとラインの時間判定に使います。`world`・`level` から到達時間を追加計算しません。既存の分数は未校正の暫定値です。

カードの各お題は `name`, `world`, `level`, `timeMin`, `exec`, `risk`, `tags` を返します。`name`はお題の内容のみで、場所は別プロパティです。通常コースは数字だけでなく `level: "コース3"` のように名前を含めた文字列で扱います。城は `level: "しろ"`、クッパ城は通常のワールドの城と区別して `level: "クッパのしろ"` とします。ワールド・コース・内容の3項目でお題の重複を判定するため、別コースに同じ内容のお題を登録できます。

同じ空でない`world`が縦・横・斜めの1ラインに重複するカードは生成しません。空文字の`world`は場所を指定しないお題を表し、複数配置できます。ステージ表記と再抽選規則の変更により、移行前と同じシードでもカードの配置が変わる場合があります。

区間のID・表示名・旧エリアIDは `backend/bingo/route_segments.json` をGoとフロントで共有します。`vanilla:common` はバニラ1の分岐前までです。出口攻略・再入場は別計上し、2出口を1回分で済ませません。共通区間と分岐別の `vanilla:cheese` / `vanilla:plateau`、`plateau:butter` を別IDにすることで、同じワールドを通っても別の道中を重複補正しません。`retiredAreas` の旧エリアは読み込み・保存互換のためデータを保持しますが、`lineCost` では差し引きません。管理画面は参考値として表示し、新しいお題の選択候補から除外します。抽選対象のお題に旧エリアが残ると、保存前に区間の選び直しを求めます。

### マスの更新

```json
{ "index": 0, "completed": true }
```

`index` は左上から行優先で0〜24。両フィールド必須です。プレイヤーIDを本文で受け付けません。取り消しは `completed=false`。クッパ撃破更新は `{ "completed": true }` のみを送信します。

### エラー

`{ "error": "説明" }` を返します。400は入力不正、401は未参加/セッション失効、403は合言葉・権限・送信元不正、404はルーム/参加者不在、409は満員・占有済み・試合状態との不整合、415はContent-Type不正、429は試行回数制限です。

## 実装上の判断

- 認証はユーザー指定に従いDiscord OAuthからルームの合言葉に変更。
- WebSocket接続はプレイヤーのデータから分離し、HTTPサーバー側で管理します。データの更新と通信処理を分け、1接続につき送信処理を1つに限定しています。
- 所要時間の不可能判定の下界は「経由時間を除いたコストが小さい5件の合計」と「コストが5番目に小さいお題」の大きい方です。上界はコストが大きい5件の合計です。共通経路のある実現可能なカードを、補正前の合計で誤って拒否しません。
- 分布設計の未確定パラメータはREADME記載の暫定値で動作し、推定時間を保証値として表示しません。
- 旧整数ルームIDは設計書のUUIDに変更。ルーム管理APIは作成オプションと参加者セッションを扱う形に更新しています。
- テストで利用するルーム・セッション以外の外部サービスは必要ありません。


### レビュー対応の追加フィールド

お題の `disabledReason` は空なら有効、理由があれば保留して抽選から除外します。`conflictGroups` は同じグループを同一ラインに配置しない制約です。カタログ・管理API・カードは `bowserRoutes`（`name`, `timeMin`, `routeAreas`）を保持します。経路名は一意、区間は未登録・重複・旧ID不可、区間合計は経路時間未満です。保存後の既存カードには影響しません。式と未検証事項は [分布設計](bingo-distribution.md) に記載しています。

### WindowsのViteとDockerのGoバックエンド

Windows側で `pnpm dev` を実行し、http://127.0.0.1:5173 を開きます。Viteの `/api` とWebSocketは、コンテナの8080番に転送されます。

初回のバックエンド起動（PowerShell、プロジェクトのルート、`.env` 設定済み）：

```powershell
docker build --target development -t smw-bingo:development .
docker run -d --name smw-bingo-dev --env-file .env -e BINGO_DATA=/app/backend/bingo.json -e ADDR=:8080 -p 127.0.0.1:8080:8080 --mount "type=bind,source=$($PWD.Path),target=/app" --tmpfs /app/backend/tmp:exec --workdir /app/backend smw-bingo:development air -c .air.toml
pnpm dev
```

作成済みのコンテナは `docker start smw-bingo-dev` で再開します。Goの変更はAirが監視して再ビルドし、フロントの変更はViteがHMRで反映します。Goが埋め込む `backend/static` がない場合は、Windows側で一度 `pnpm build` を実行してください。一時実行ファイルは実行可能なtmpfsに置きます。Airの `include_dir` は空配列としてサブディレクトリも監視します。
