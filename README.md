# pf-workspace

P04 workspace — カンバン + Wiki + 共同編集 + チャットの製品リポジトリ（学習用。本番 Linear / Notion / Slack の置き換えではない）。

## 構成

| パス | 役割 |
| --- | --- |
| `apps/api` | Go API — ワークスペース、カンバン、Wiki、collab チケット、チャット履歴 / seq / chat WS |
| `apps/collab` | Node + Hocuspocus — Wiki 本文と独立ドキュメントの Yjs 同期 |
| `apps/web` | Next.js — ボード、Wiki、Docs、チャット |
| `deploy/` | Compose 単体デモ |

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

### 2 ウィンドウ

1. A（`/?user=demo-user-a`）でワークスペースを作る
2. `demo-user-b` を member で追加する
3. Wiki / Docs で同時編集、Chat の `#general` で投稿と入力中表示

collab が落ちてもカンバンとチャットは動く。チャット WS が切れても REST 履歴は残る。再接続は `afterSeq` で差分取得。

### IME

変換中の中間入力は Yjs に送らず、`compositionend` で確定分だけ同期する。変換中に相手が同じ箇所を編集すると稀に食い違うことがある。

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

設計: `project/portfolio-plan/workspace/DESIGN.md`  
人間向け書類: `project/portfolio-plan/workspace/docs/`
