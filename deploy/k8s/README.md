# P04 Kubernetes manifests

workspace api / collab / web。Yjs（collab）とチャット WS（api `/chat/ws`）は **別 Service**。単体 apply ではなく `pf-cloud-k8s` overlay `b-collab` から参照する。

Ingress（`pf-cloud-k8s`）:

| ホスト | Service | 用途 |
| --- | --- | --- |
| `workspace.localhost` | web:3006 | Next.js。overlay では OIDC 必須 |
| `workspace-api.localhost` | api:8096 | REST + チャット WS |
| `workspace-collab.localhost` | collab:8097 | Hocuspocus / Yjs WS |

## 永続化

**メモリ store。** 再起動でカンバン / Wiki / チャット / チケットは消える。platform Postgres の DB 名 `workspace` は予約済みだが、このスライスでは未接続（P10 の `talent` DB パターンへの移行は後続）。

## 認証

| モード | Web | API |
| --- | --- | --- |
| Compose 単体 | `WORKSPACE_DEV_AUTH`（`?user=`） | `WORKSPACE_DEV_AUTH` + `X-Dev-User-Sub` |
| overlay | OIDC（`pf-workspace-web`）。未ログインは `/login` | overlay smoke 用に `WORKSPACE_DEV_AUTH=true`。Bearer も可 |

秘密は overlay の `secretGenerator`（`p04-secrets` / `WORKSPACE_INTERNAL_TOKEN`）。製品 manifest に平文 Secret は置かない。

## 観測

overlay が `OTEL_EXPORTER_OTLP_ENDPOINT` を api に渡す。現状の Go API は Collector へ送らない（環境変数だけ予約）。

```powershell
cd ..\..\pf-cloud-k8s
.\scripts\cluster-smoke-b-collab.ps1
```

Compose 単体デモは従来どおり `deploy/compose.yaml`。
