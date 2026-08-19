# pf-workspace

P04 workspace — カンバン + Wiki + 共同編集 + チャット + 横断検索 + スプリント / Wiki 履歴の製品リポジトリ（学習用。本番 Linear / Notion / Slack の置き換えではない）。

## 構成

| パス | 役割 |
| --- | --- |
| `apps/api` | Go API — ワークスペース、カンバン、Wiki、collab チケット、チャット、横断検索、添付（ローカルまたは P03）、スプリント、Wiki 履歴 |
| `apps/collab` | Node + Hocuspocus — Wiki 本文と独立ドキュメントの Yjs 同期 |
| `apps/web` | Next.js — ボード、Wiki、Docs、チャット、横断検索、バーンダウン |
| `deploy/` | Compose 単体デモ + `deploy/k8s/`（連携 overlay） |

collab / chat はまだ独立 git リポジトリにしていない（DESIGN の段階化）。チャット WS は Yjs と同じソケットに乗せていない（`/chat/ws`）。

## 単体デモ（開発モード）

```powershell
cd deploy
copy .env.example .env
docker compose up -d --build
```

- Web: http://localhost:3006
- API: http://localhost:8096/health
- Collab: http://localhost:8097/health
- Chat WS: `ws://localhost:8096/chat/ws`

永続化は **Compose 専用 Postgres**（ホスト `localhost:5439`、DB/user `workspace`）。`WORKSPACE_DATABASE_URL` が空ならメモリ（単体テスト）。overlay `b-collab` は platform Postgres の `workspace` DB。

## 連携デモ（Kubernetes）

`pf-cloud-k8s` overlay `b-collab`（P01 + P02 + P03 + P04。P11 portal は後続）。

| URL | 用途 |
| --- | --- |
| http://workspace.localhost | Web。OIDC 必須 |
| http://workspace-api.localhost/health | API。チャット WS もこのホスト |
| http://workspace-collab.localhost/health | Yjs / Hocuspocus。チャット WS とは別 |

Compose は `WORKSPACE_DEV_AUTH` のまま。overlay の web は `pf-workspace-web` クライアント。手順は `project/portfolio-plan/integration-demo.md`。

### 2 ウィンドウ

1. A（`/?user=demo-user-a`）でワークスペースを作る
2. `demo-user-b` を member で追加する
3. Wiki / Docs で同時編集、Chat の `#general` で投稿と入力中表示

collab が落ちてもカンバンとチャットは動く。チャット WS が切れても REST 履歴は残る。再接続は `afterSeq` で差分取得。

### 横断検索

ホームの検索欄、または `/search/{workspaceId}?q=`。guest（`?user=` で切替）では draft Wiki は出ない。照合はストア上の部分一致（Postgres FTS ではない）。

### 添付（P03 は任意）

単体 Compose に media は含めない。Wiki / Chat の画像は API のローカル保存（20MB まで）。P03 を別起動する場合だけ API に `MEDIA_API_URL` を渡し、ブラウザが `purpose=wiki|chat` で presign する。Yjs には fileId だけを載せる。

### IME

変換中の中間入力は Yjs に送らず、`compositionend` で確定分だけ同期する。変換中に相手が同じ箇所を編集すると稀に食い違うことがある。

### スプリントとバーンダウン

ボードから「スプリント」へ。期間は timestamptz（UTC）。カード詳細でスプリントに割り当てる。バーンダウンは **未完了カード数**（Done 列以外）。ストーリーポイントは持たない。現在の割り当てと Done 時刻から日次の残りを出す（移動イベントの完全ログではない）。

### Wiki 履歴

ページ下部で版一覧・行 diff・復元。スナップショットは API の title+body（Y.Doc バイトではない）。guest は published のみ（FilterGuestPages と同じ）。collab 接続中の復元は API 本文を戻し、開いている Y.Doc は再読込後に揃う。

## テスト

```powershell
cd apps/api
go test ./...

cd ../collab
npm test

cd ../web
npm test
npm run build
```

## 実装状況

- [x] ワークスペース CRUD、メンバー（owner / member / guest）
- [x] カンバン MVP
- [x] OIDC ログイン（web）+ dev auth（api）
- [x] Wiki ツリー + Markdown。guest は published のみ
- [x] collab（Wiki 本文と独立 Docs。IME は composition 中に同期しない）
- [x] chat（REST 履歴 + seq + 別 WS + typing。guest は閲覧のみ）
- [x] 横断検索（page / document / card / message。guest は FilterGuestPages）
- [x] メンション（`@sub` をメンバーに解決。WS に載せる。メール / 未読なし）
- [x] 添付（P03 任意 + ローカルフォールバック。guest は追加不可）
- [x] スプリント CRUD とカード割り当て、日次バーンダウン（cards）
- [x] Wiki 履歴（版一覧 / 行 diff / 復元）。guest は draft 履歴なし
- [x] Postgres（Compose 専用 DB。overlay B は platform `workspace`。単体テストはメモリ）

設計: `project/portfolio-plan/workspace/DESIGN.md`  
人間向け書類: `project/portfolio-plan/workspace/docs/`
