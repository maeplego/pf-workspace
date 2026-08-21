# BYO IdP — Auth0 / Entra（実装済み・外部接続は未検証）

ポートフォリオデモでは Auth0 / Entra への実ログイン確認はしない。  
設定とコードパスは用意済み。接続試験は顧客環境または将来のステージングで行う。

## 共通（API + Web）

| 変数 | 意味 |
| --- | --- |
| `OIDC_ISSUER` | 例: Auth0 `https://YOUR.auth0.com/` / Entra `https://login.microsoftonline.com/{tenant}/v2.0` |
| `OIDC_INTERNAL_BASE` | クラスタ内から見える issuer（省略時は ISSUER） |
| `OIDC_CLIENT_ID` | アプリ（SPA）クライアント ID |
| `OIDC_CLIENT_SECRET` | 機密クライアント時のみ（トークン交換） |
| `OIDC_AUDIENCE` | API が検証する access token aud（空なら aud 検証スキップ） |
| `OIDC_ORG_CLAIM` | テナント ID クレーム名（既定 `org_id`。カンマ区切り可） |
| `OIDC_ORGS_CLAIM` | 所属一覧クレーム（既定 `organizations`） |
| `OIDC_SCOPES` | 既定 `openid profile email org offline_access`。Auth0/Entra では `org` を外す |

Web / API は OpenID discovery（`jwks_uri` / `userinfo_endpoint` / `end_session_endpoint`）を使う。P01 固定パスにもフォールバック。

## Auth0（設定例・未接続）

```text
OIDC_ISSUER=https://YOUR_TENANT.auth0.com/
OIDC_CLIENT_ID=...
OIDC_AUDIENCE=https://api.your-app.example
OIDC_SCOPES=openid profile email offline_access
OIDC_ORG_CLAIM=https://schemas.portfolio.example/org_id
OIDC_ORGS_CLAIM=https://schemas.portfolio.example/organizations
WORKSPACE_ENV=staging
WORKSPACE_DEV_AUTH=false
```

Auth0 Action / Rule でカスタムクレームに `org_id`（または上表の URI）を載せる想定。

## Microsoft Entra ID（設定例・未接続）

```text
OIDC_ISSUER=https://login.microsoftonline.com/{tenant-id}/v2.0
OIDC_CLIENT_ID=...
OIDC_SCOPES=openid profile email offline_access
OIDC_ORG_CLAIM=tid
# またはアプリの Optional claim / 拡張属性名
# OIDC_ORG_CLAIM=extension_OrgId
WORKSPACE_ENV=staging
WORKSPACE_DEV_AUTH=false
```

`tid` フォールバックはコード側にもある（ディレクトリ全体を 1 テナントとみなす簡易マッピング）。

## ローカル代替

外部 IdP の代わりに `deploy/byo-oidc/mockoidc`（`org_id` 付き）で経路確認する。
