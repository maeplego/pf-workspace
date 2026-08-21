# BYO IdP lab (mock OIDC)

| 項目 | 値 |
| --- | --- |
| 最終更新 | 2026-08-21 |

同梱 P01 の代わりに、`org_id` 付きトークンを出す最小 OIDC を起動して Collab AuthPort を確認する。本番 IdP ではない。

## 起動

```powershell
cd pf-workspace/deploy/byo-oidc/mockoidc
go run .
# listening :5556  issuer=http://127.0.0.1:5556
```

検証:

```powershell
curl.exe -sS http://127.0.0.1:5556/health
curl.exe -sS http://127.0.0.1:5556/.well-known/openid-configuration
```

## Workspace を向け直す

別ターミナルで API / Web（Compose または `go` / `next`）:

```text
WORKSPACE_ENV=staging
WORKSPACE_DEV_AUTH=false
OIDC_ISSUER=http://127.0.0.1:5556
OIDC_INTERNAL_BASE=http://127.0.0.1:5556
OIDC_AUDIENCE=pf-workspace-web
OIDC_CLIENT_ID=pf-workspace-web
OIDC_REDIRECT_URI=http://localhost:3006/callback
```

ブラウザで `http://localhost:3006/` → `/login` → mock は承認 UI なしで即 callback。ホームに `BYO Demo` と組織スイッチャー（Org A/B）が出れば BYO 経路の実演成功。

`/v1/active-org` は無いので切替は Cookie + `X-Workspace-Org` フォールバックを使う（実装済み）。

## 記録テンプレ

| 項目 | 記入 |
| --- | --- |
| 日付 | |
| issuer | http://127.0.0.1:5556 |
| discovery | OK / NG |
| ログイン→ホーム | OK / NG |
| org 切替 | OK / NG |
| 備考 | |

正本: [portability-byo-idp.md](../../../project/portfolio-plan/portability-byo-idp.md)（メタリポ側）。
