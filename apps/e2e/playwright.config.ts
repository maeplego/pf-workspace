import { defineConfig, devices } from "@playwright/test";
import path from "node:path";

const idpPort = 18080;
const apiPort = 18096;
const webPort = 13006;
const idp = `http://127.0.0.1:${idpPort}`;
const api = `http://127.0.0.1:${apiPort}`;
const web = `http://127.0.0.1:${webPort}`;
const repo = path.join(__dirname, "..", "..");
const identityServer = path.join(repo, "..", "pf-identity", "apps", "server");

export default defineConfig({
  testDir: "./tests",
  timeout: 180_000,
  expect: { timeout: 20_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: "list",
  use: {
    ...devices["Desktop Chrome"],
    baseURL: web,
    trace: "off",
  },
  webServer: [
    {
      command: "go run ./cmd/server",
      cwd: identityServer,
      url: `${idp}/health`,
      reuseExistingServer: false,
      timeout: 120_000,
      env: {
        ...process.env,
        IDENTITY_HTTP_ADDR: `:${idpPort}`,
        IDENTITY_ISSUER: idp,
        IDENTITY_DEV_GENERATE_KEYS: "true",
        IDENTITY_STORE: "memory",
        IDENTITY_COOKIE_SECURE: "false",
        IDENTITY_ADMIN_TOKEN: "e2e-admin-token",
        IDENTITY_SEED_PUBLIC_CLIENT_ID: "pf-workspace-web",
        IDENTITY_SEED_PUBLIC_CLIENT_NAME: "pf-workspace-web",
        IDENTITY_SEED_PUBLIC_REDIRECT_URI: `${web}/callback`,
        IDENTITY_SEED_PUBLIC_POST_LOGOUT_REDIRECT_URI: `${web}/logged-out`,
        IDENTITY_SEED_DEMO_EMAIL: "demo@example.test",
        IDENTITY_SEED_DEMO_PASSWORD: "demo-pass-change-me",
        IDENTITY_SEED_DEMO_NAME: "Demo User",
      },
    },
    {
      command: "go run ./cmd/server",
      cwd: path.join(repo, "apps", "api"),
      url: `${api}/health`,
      reuseExistingServer: false,
      timeout: 120_000,
      env: {
        ...process.env,
        WORKSPACE_HTTP_PORT: String(apiPort),
        WORKSPACE_ENV: "staging",
        WORKSPACE_DEV_AUTH: "false",
        WORKSPACE_REQUIRE_ORG: "true",
        WORKSPACE_CORS_ORIGIN: web,
        WORKSPACE_INTERNAL_TOKEN: "e2e-internal",
        OIDC_ISSUER: idp,
        OIDC_INTERNAL_BASE: idp,
        OIDC_AUDIENCE: "pf-workspace-web",
      },
    },
    {
      command: "npx next dev -p 13006 --hostname 127.0.0.1",
      cwd: path.join(repo, "apps", "web"),
      url: `${web}/health`,
      reuseExistingServer: false,
      timeout: 180_000,
      env: {
        ...process.env,
        WORKSPACE_API_URL: api,
        OIDC_ISSUER: idp,
        OIDC_INTERNAL_BASE: idp,
        OIDC_CLIENT_ID: "pf-workspace-web",
        OIDC_REDIRECT_URI: `${web}/callback`,
        OIDC_POST_LOGOUT_REDIRECT_URI: `${web}/logged-out`,
      },
    },
  ],
});
