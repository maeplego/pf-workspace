# pf-workspace

| まず | リンク |
| --- | --- |
| 採用の位置づけ | [HIRING.md](https://github.com/maeplego/portfolio-plan/blob/master/portfolio-plan/HIRING.md) |
| 確認手順 | [REVIEW.md](https://github.com/maeplego/portfolio-plan/blob/master/portfolio-plan/REVIEW.md) |

学習用のチームワークスペースです。カンバン、Wiki、共同編集、チャット、横断検索、スプリントのバーンダウンを一つの製品にまとめています。**Linear / Notion / Slack の置き換えではありません。**

| ディレクトリ | 役割 |
| --- | --- |
| `apps/api` | Go API |
| `apps/collab` | Wiki / ドキュメントの Yjs 同期 |
| `apps/web` | Next.js |
| `deploy/` | 単体 Compose |

チャットの WebSocket は Yjs とは別経路（`/chat/ws`）です。共同編集が落ちてもカンバンとチャットは動きます。

## 起動

```powershell
cd deploy
copy .env.example .env
docker compose up -d --build
```

| URL | 用途 |
| --- | --- |
| http://localhost:3006 | Web |
| http://localhost:8096/health | API |
| http://localhost:8097/health | 共同編集 |

Compose は開発用ヘッダ認証です。ブラウザで `/?user=demo-user-a` のようにユーザーを切り替えます。

## デモ

1. ユーザー A でワークスペースを作る
2. `demo-user-b` をメンバーに追加する
3. Wiki や Docs を 2 ウィンドウで同時編集する。チャット `#general` で投稿と入力中表示を見る
4. ホームの検索、または `/search/{workspaceId}?q=` で横断検索する（部分一致。guest には下書き Wiki は出ません）

IME の変換中は共同編集に送らず、確定後だけ同期します。

## テスト

```powershell
cd apps/api
go test ./...
cd ..\collab
npm test
cd ..\web
npm test
npm run build
```

設計の詳細は [portfolio-plan](https://github.com/maeplego/portfolio-plan) の `portfolio-plan/workspace/docs/` です。Kubernetes 連携は [pf-cloud-k8s](https://github.com/maeplego/pf-cloud-k8s) の collab overlay です。

## ライセンスと利用条件

本リポジトリは **デモ・学習・社内評価用** です。現状品質に **保証はありません**。

- 許可: クローン、ローカル実行、学習、非本番の評価
- 別契約が必要: 本番運用、有償サービスへの組込み、再販・托管の提供

詳細は [LICENSE](./LICENSE) と [licensing.md](https://github.com/maeplego/portfolio-plan/blob/master/portfolio-plan/licensing.md) を参照してください。

