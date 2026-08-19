# P04 Kubernetes manifests

workspace api / collab / web。Yjs（collab）とチャット WS（api `/chat/ws`）は **別 Service**。単体 apply ではなく `pf-cloud-k8s` overlay `b-collab` から参照する。

Ingress（`pf-cloud-k8s`）:

| ホスト | Service | 用途 |
| --- | --- | --- |
| `workspace.localhost` | web:3006 | Next.js。overlay では OIDC 必須 |
| `workspace-api.localhost` | api:8096 | REST + チャット WS |
| `workspace-collab.localhost` | collab:8097 | Hocuspocus / Yjs WS |

## 永続化

Compose は専用 Postgres。overlay `b-collab` は platform の DB 名 `workspace`（`ensure-platform-databases.ps1` / `init-databases.sql`）。接続文字列は `p04-secrets` の `WORKSPACE_DATABASE_URL`。単体テストはメモリ（URL 空）。

Y.Doc は collab プロセスのメモリのまま。

## 認証

| モード | Web | API |
| --- | --- | --- |
| Compose 単体 | `WORKSPACE_DEV_AUTH`（`?user=`） | `WORKSPACE_DEV_AUTH` + `X-Dev-User-Sub` |
| overlay | OIDC（`pf-workspace-web`）。未ログインは `/login` | overlay smoke 用に `WORKSPACE_DEV_AUTH=true`。Bearer も可 |

秘密は overlay の `secretGenerator`（`p04-secrets` の `WORKSPACE_INTERNAL_TOKEN` と `WORKSPACE_DATABASE_URL`）。製品 manifest に平文 Secret は置かない。

## 観測

overlay が `OTEL_EXPORTER_OTLP_ENDPOINT` を api に渡す。現状の Go API は Collector へ送らない（環境変数だけ予約）。

```powershell
cd ..\..\pf-cloud-k8s
.\scripts\cluster-smoke-b-collab.ps1
```

Compose 単体デモは従来どおり `deploy/compose.yaml`。
