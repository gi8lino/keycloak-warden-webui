import { expect } from "@playwright/test";
import {
  localPassword,
  providerPassword,
  setupToken,
  subject,
  test,
} from "./fixtures.mjs";

test.describe.configure({ mode: "parallel", timeout: 180_000 });

// Provision the founder through the public first-run form in every mode.
async function setup(page, url) {
  await page.goto(url + "/setup");
  await page.getByLabel("Setup token").fill(setupToken);
  await page.getByLabel("Username").fill("founder");
  await page.locator("#setup-password").fill(localPassword);
  await page.getByLabel("Confirm password").fill(localPassword);
  await page.getByRole("button", { name: "Create administrator" }).click();
}

// Assert the session or proxy identity actually maps to the named local account.
async function expectFounder(page, url) {
  const response = await page.goto(url + "/admin/users");
  expect(response.status()).toBe(200);
  await expect(
    page.locator("summary:visible").filter({ hasText: "founder" }),
  ).toBeVisible();
}

test("local password login, rejection, and logout", async ({ lab, page }) => {
  await lab.start("local");
  await setup(page, lab.url);
  await expectFounder(page, lab.url);
  await page.locator("summary:visible").filter({ hasText: "founder" }).click();
  await page.getByRole("button", { name: "Sign out" }).click();
  await page.goto(lab.url + "/admin/users");
  await expect(page).toHaveURL(lab.url + "/login");

  await page.getByLabel("Username").fill("founder");
  await page.getByLabel("Password").fill("wrong-password");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("alert")).toContainText(
    "Invalid username or password",
  );
  await page.getByLabel("Username").fill("founder");
  await page.getByLabel("Password").fill(localPassword);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expectFounder(page, lab.url);
});

test("trusted proxy requires the linked identity and administrator group", async ({
  lab,
  page,
}) => {
  await lab.start("proxy", [
    "--trusted-proxy-cidrs=127.0.0.1/32",
    "--setup-subject=upstream-user",
  ]);
  await setup(page, lab.url);
  await expectFounder(page, await lab.proxy("upstream-user", "warden-admins"));

  const wrongGroup = await lab.proxy("upstream-user", "readers");
  await page.setExtraHTTPHeaders({ "X-Forwarded-Groups": "warden-admins" });
  expect((await page.goto(wrongGroup + "/admin/users")).status()).toBe(403);
  const unlinked = await lab.proxy("unknown", "warden-admins");
  expect((await page.goto(unlinked + "/admin/users")).status()).toBe(403);
});

test("OIDC authorization code login via Keycloak", async ({
  lab,
  page,
  browser,
}) => {
  const issuer = await lab.keycloak();
  await lab.start("oidc", [
    "--oidc-issuer=" + issuer,
    "--oidc-client-id=webui",
    "--oidc-client-secret=e2e-client-secret",
    "--oidc-session-secret=e2e-only-oidc-state-secret-32-characters",
    "--setup-subject=" + subject,
    "--oidc-group-claim=groups",
  ]);
  await setup(page, lab.url);
  await page.getByRole("link", { name: "Continue", exact: true }).click();
  await expect(page).toHaveURL(
    new RegExp("/realms/warden-test/protocol/openid-connect/auth"),
  );
  expect(new URL(page.url()).searchParams.get("code_challenge_method")).toBe(
    "S256",
  );
  await keycloakLogin(page, "operator");
  await expect(page).toHaveURL(lab.url + "/");
  await expectFounder(page, lab.url);

  await page.locator("summary:visible").filter({ hasText: "founder" }).click();
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(
    page.getByRole("link", { name: "Continue", exact: true }),
  ).toBeVisible();
  expect(
    (
      await page.request.get(lab.url + "/admin/users", { maxRedirects: 0 })
    ).status(),
  ).toBe(303);

  const outsider = await browser.newContext();
  try {
    const denied = await outsider.newPage();
    await denied.goto(lab.url + "/admin/users");
    await keycloakLogin(denied, "visitor");
    await expect(
      denied.getByText(
        "OIDC authentication failed or administrator group missing.",
      ),
    ).toBeVisible();
    expect(
      (
        await outsider.request.get(lab.url + "/admin/users", {
          maxRedirects: 0,
        })
      ).status(),
    ).toBe(303);
  } finally {
    await outsider.close();
  }
});

// Use Keycloak's hosted login form so discovery, code exchange and token verification run normally.
async function keycloakLogin(page, username) {
  await page.locator("#username").fill(username);
  await page.locator("#password").fill(providerPassword);
  await page.locator("#kc-login").click();
}
