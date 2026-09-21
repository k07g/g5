# g5

キャリアシートのバックエンドAPI

Go + MongoDB によるキャリアシートAPI。[career-sheet](https://github.com/k07g/career-sheet)
(フロントエンド)から呼び出され、キャリアシートのデータをユーザーごとに保存します。
認証情報の発行・検証(サインアップ/サインイン)は [g4](https://github.com/k07g/g4) が担当し、
g5 は g4 と同じ Amazon Cognito ユーザープールが発行したアクセストークンを検証するだけで、
自身では認証情報を持ちません。

## 機能

- キャリアシート取得 (`GET /career-sheet`, 要アクセストークン)
- キャリアシート保存 (`PUT /career-sheet`, 要アクセストークン) — ドキュメント全体を置き換えます
- キャリアシート削除 (`DELETE /career-sheet`, 要アクセストークン)

キャリアシートは呼び出し元の Cognito `sub` を `_id` としたドキュメントとして
MongoDB (`career_sheets` コレクション) に1ユーザー1件で保存されます。`_id` が
そのまま一意制約になるため、追加のインデックス作成は不要です。データ形式は
career-sheet の [`src/types/career-sheet.ts`](https://github.com/k07g/career-sheet/blob/main/src/types/career-sheet.ts)
と1対1で対応しており、フロントエンドはこれまでの `localStorage` ベースの
`CareerSheetRepository` をHTTP版の実装に差し替えるだけで利用できます。

## セットアップ(本番 / Cognito 接続)

1. `.env.example` を `.env` にコピーし、AWSリージョンと MongoDB の接続情報を設定
2. AWS 認証情報(環境変数 / `~/.aws/credentials` など)を用意
3. `go run ./cmd/server`

g5 は Cognito の `GetUser` 操作でアクセストークンを検証するだけなので、
ユーザープールIDやアプリクライアントIDの設定は不要です(トークン自体に
その情報が含まれ、Cognito側で検証されます)。

## ローカルでの動作検証(AWSアカウント不要)

`AUTH_PROVIDER=memory` を指定すると、Amazon Cognito の代わりにプロセス内蔵の
インメモリ認証ベリファイア([internal/auth/memory.go](internal/auth/memory.go))が使われます。
このベリファイアは非空のBearerトークンをそのまま `sub` として扱うため、
g4 を `AUTH_PROVIDER=memory` で動かしている場合と組み合わせて使えます。
本番では絶対に使用しないでください(実際の認証を行いません)。

1. ローカルMongoDBを起動

   ```sh
   docker compose up -d mongo
   ```

2. `.env.local.example` を参考に環境変数を設定してサーバーを起動

   ```sh
   PORT=8080 \
   AUTH_PROVIDER=memory \
   MONGODB_URI="mongodb://g5:g5@localhost:27018" \
   MONGODB_DATABASE=g5 \
   go run ./cmd/server
   ```

3. curl で動作確認

   ```sh
   curl -X PUT localhost:8080/career-sheet \
     -H "Authorization: Bearer any-token" \
     -H "Content-Type: application/json" \
     -d '{"basicInfo":{"name":"山田太郎"},"summary":"","workExperiences":[],"skills":[],"educations":[],"certifications":[],"selfPromotion":""}'

   curl localhost:8080/career-sheet -H "Authorization: Bearer any-token"
   ```

## API

すべて `Authorization: Bearer <access_token>` が必須です。トークンは
[g4](https://github.com/k07g/g4) のサインインAPIが発行するアクセストークンを使用します。

### GET /career-sheet

保存済みのキャリアシートを返します。未保存の場合は `404`。

### PUT /career-sheet

リクエストボディ全体でキャリアシートを置き換えます(新規作成 / 上書き更新)。
成功時は `204`。

### DELETE /career-sheet

保存済みのキャリアシートを削除します。未保存の状態で呼んでも `204` を返します
(冪等な操作として扱われます)。

## テストについて

3種類のテストがあります。

- **単体テスト**(`internal/config`、`internal/auth`、`internal/models`、
  `internal/api`)— 外部依存を使いません。`internal/api` のHTTPハンドラー
  テストは `internal/db.CareerSheetRepository` の代わりにインメモリの fake
  (`internal/api/api_test.go`)を、認証には `auth.MemoryVerifier` を使って
  検証しています。
- **リポジトリテスト**(`internal/db`)— MongoDB公式ドライバ(v2)には
  `database/sql` の `sqlmock` に相当する外部公開されたモック手段が無いため、
  [testcontainers-go](https://golang.testcontainers.org/) で実際のMongoDBコンテナを
  `go test` 実行時に自動起動・自動破棄し、`CareerSheetRepository` 単体の
  クエリ/更新ロジックを検証しています(`internal/db/career_sheets_test.go`)。
- **結合テスト**(`internal/integration`)— ルーター・認証ミドルウェア・
  MongoDBリポジトリを `cmd/server/main.go` と同じ組み方で実際に結線し、
  実際のMongoDBコンテナ相手に `httptest.Server` 経由の本物のHTTPリクエストで
  一連のライフサイクル(認証エラー→未保存→作成→取得→更新→別ユーザーからの
  非可視性→削除→冪等な再削除)を検証しています(`internal/integration/server_test.go`)。
  認証はCognitoの代わりに `auth.MemoryVerifier` を使っています(ローカル開発の
  `AUTH_PROVIDER=memory` と同じ代替です。実AWSアカウントがCIに無いため)。

`internal/db` と `internal/integration` の実行にはDockerが必要です。Dockerが
利用できない環境では、エラーではなくスキップ扱いになります(`go test` の
標準出力に理由が表示されます)。
