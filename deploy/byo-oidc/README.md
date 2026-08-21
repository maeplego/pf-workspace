# BYO IdP lab (mock OIDC)

| 項目 | 値 |
| --- | --- |
| 最終更新 | 2026-08-21 |

同梱 P01 の代わりに、`org_id` 付きトークンを出す最小 OIDC。本番ではない。

Auth0 / Entra の設定例（**外部実接続はデモ非目標**）: [AUTH0-ENTRA.md](./AUTH0-ENTRA.md)

## 起動

```powershell
cd pf-workspace/deploy/byo-oidc/mockoidc
go run .
```

```powershell
curl.exe -sS http://127.0.0.1:5556/health
curl.exe -sS http://127.0.0.1:5556/.well-known/openid-configuration
```

## Workspace 向け設定

```text
WORKSPACE_ENV=staging
WORKSPACE_DEV_AUTH=false
OIDC_ISSUER=http://127.0.0.1:5556
OIDC_INTERNAL_BASE=http://127.0.0.1:5556
OIDC_AUDIENCE=pf-workspace-web
OIDC_CLIENT_ID=pf-workspace-web
OIDC_REDIRECT_URI=http://localhost:3006/callback
```

`/v1/active-org` 無しでも Cookie + `X-Workspace-Org` で org 切替可能。
