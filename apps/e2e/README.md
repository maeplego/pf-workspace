# Collab Playwright E2E

Bundled IdP path: login → home → create workspace → create second org → switch org.

```powershell
cd apps/e2e
npm install
npx playwright install chromium
npm test
```

Requires sibling `../pf-identity/apps/server`. Uses `WORKSPACE_ENV=staging` (DEV_AUTH off).
