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

## デモ（招待本線）

メンバー参加の本線は **招待リンク**（`/join/{token}`）です。sub の手入力追加は使いません。

1. ブラウザ A で `/?user=demo-user-a` を開き、ワークスペースを作成する
2. owner として「招待リンク発行」し、表示された URL（既定 `http://localhost:3006/join/{token}`）をコピーする
3. ブラウザ B（別プロファイル／シークレット）で `/?user=demo-user-b` のあと、その `/join/{token}` を開いて参加する
4. Wiki や Docs を 2 ウィンドウで同時編集する。チャット `#general` で投稿と入力中表示を見る
5. ホームの検索、または `/search/{workspaceId}?q=` で横断検索する（部分一致。guest には下書き Wiki は出ません）

招待リンクの origin は `WORKSPACE_PUBLIC_BASE_URL`（Compose 既定 `http://localhost:3006`）です。

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

