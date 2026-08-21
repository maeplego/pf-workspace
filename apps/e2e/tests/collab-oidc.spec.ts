import { expect, test, type Page } from "@playwright/test";

const idp = "http://127.0.0.1:18080";
const email = "demo@example.test";
const password = "demo-pass-change-me";

async function loginCollab(page: Page) {
  await page.goto("/login", { waitUntil: "networkidle" });
  await page.waitForURL(/18080\/(authorize|login)/, { timeout: 30_000 });

  // Authorize may bounce to /login when no session.
  if (page.url().includes("/login") || (await page.locator('input[name="email"]').count()) > 0) {
    await page.locator('input[name="email"]').fill(email);
    await page.locator('input[name="password"]').fill(password);
    await page.getByRole("button", { name: "ログイン" }).click();
  }

  const consent = page.getByRole("button", { name: "許可" });
  await consent.waitFor({ state: "visible", timeout: 30_000 });
  await consent.click();

  await page.waitForURL((url) => url.port === "13006" && !url.pathname.includes("callback"), {
    timeout: 30_000,
  });
  await expect(page.getByTestId("workspace-home")).toBeVisible({ timeout: 30_000 });
}

test("login home create workspace and switch org", async ({ page, request }) => {
  await loginCollab(page);
  await expect(page.getByText("Demo User")).toBeVisible();

  const name = `E2E WS ${Date.now()}`;
  await page.getByTestId("workspace-name").fill(name);
  await page.getByTestId("create-workspace").click();
  await expect(page.getByRole("heading", { name })).toBeVisible({ timeout: 20_000 });

  const cookies = await page.context().cookies();
  const access = cookies.find((c) => c.name.startsWith("rp_access"))?.value;
  expect(access, "access token cookie").toBeTruthy();
  const createOrg = await request.post(`${idp}/v1/organizations`, {
    headers: { Authorization: `Bearer ${access}`, "Content-Type": "application/json" },
    data: { name: `E2E Org ${Date.now()}` },
  });
  expect(createOrg.ok(), await createOrg.text()).toBeTruthy();
  const org = (await createOrg.json()) as { id: string };
  expect(org.id).toBeTruthy();

  await page.reload();
  await expect(page.getByTestId("org-switcher")).toBeVisible();
  const options = page.getByTestId("org-switcher").locator("option");
  await expect(options).toHaveCount(2, { timeout: 15_000 });
  await page.getByTestId("org-switcher").selectOption(org.id);
  await expect(page.getByTestId("org-switcher")).toHaveValue(org.id, { timeout: 15_000 });
  await expect(page.getByRole("heading", { name })).toHaveCount(0);
});
