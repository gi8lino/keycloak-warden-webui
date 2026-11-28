import { expect, test } from "@playwright/test";
import { createHmac, randomUUID } from "node:crypto";

const warden = `http://127.0.0.1:${process.env.E2E_WARDEN_PORT || 18081}`;
const webui = `http://127.0.0.1:${process.env.E2E_WEBUI_PORT || 18092}`;
const adminToken = "development-admin-token";
const snitchSecret = "development-snitch-secret";
const setupToken = "development-setup-token-0123456789abcdef";
const ip = "203.0.113.42";

// Send the exact schema-v2 body authenticated by Snitch's timestamped HMAC.
async function sendEvent(kind, payload) {
  const now = new Date();
  const timestamp = String(Math.floor(now.getTime() / 1000));
  const body = JSON.stringify({
    schemaVersion: 2,
    id: randomUUID(),
    source: "keycloak",
    kind,
    timestamp: now.toISOString(),
    [kind]: payload,
  });
  const signature = createHmac("sha256", snitchSecret)
    .update(`${timestamp}.${body}`)
    .digest("hex");
  const response = await fetch(`${warden}/events`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Snitch-Timestamp": timestamp,
      "X-Snitch-Signature": `v1=${signature}`,
    },
    body,
    signal: AbortSignal.timeout(10_000),
  });
  expect(response.status, await response.text()).toBe(204);
}

// Read authoritative Warden state while its asynchronous workers finish delivery.
async function readAPI(path) {
  const response = await fetch(warden + path, {
    headers: { Authorization: `Bearer ${adminToken}` },
    signal: AbortSignal.timeout(10_000),
  });
  expect(response.status).toBe(200);
  return response.json();
}

test("real Warden incidents, notifications, retry and worker status", async ({ page }) => {
  test.setTimeout(120_000);
  await page.goto(`${webui}/setup`);
  await page.getByLabel("Setup token").fill(setupToken);
  await page.getByLabel("Username").fill("founder");
  await page.locator("#setup-password").fill("test-local-password");
  await page.getByLabel("Confirm password").fill("test-local-password");
  await page.getByRole("button", { name: "Create administrator" }).click();
  await expect(page.getByRole("heading", { name: "Current bans" })).toBeVisible();

  const login = {
    type: "LOGIN_ERROR",
    realmId: "warden-e2e",
    clientId: "webui-e2e",
    userId: "test-user",
    ipAddress: ip,
    error: "invalid_user_credentials",
  };
  await sendEvent("user", login);
  await sendEvent("user", login);
  await sendEvent("admin", {
    realmId: "warden-e2e",
    operationType: "CREATE",
    resourceType: "USER",
    resourcePath: "users/test-user",
    actor: {
      realmId: "master",
      clientId: "admin-cli",
      userId: "test-admin",
      ipAddress: "203.0.113.43",
    },
    error: "",
  });

  const incidents = await readAPI("/incidents");
  const userIncident = incidents.items.find((item) => item.rule === "brute-force");
  const adminIncident = incidents.items.find((item) => item.rule === "admin-audit");
  expect(userIncident).toMatchObject({ count: 2, threshold: 2, groupKey: ip });
  expect(userIncident.decisionId).toBeTruthy();
  expect(adminIncident).toMatchObject({ eventKind: "admin", operationType: "CREATE", resourceType: "USER" });
  expect(adminIncident.banUntil).toBeUndefined();

  await page.goto(`${webui}/incidents`);
  await expect(page.locator("table")).toContainText("LOGIN_ERROR");
  await expect(page.locator("table")).toContainText("CREATE USER");
  await page.getByLabel("Event kind").selectOption("admin");
  await page.getByRole("button", { name: "Apply filters" }).click();
  await expect(page.locator("table")).toContainText("admin-audit");
  await expect(page.locator("table")).not.toContainText("brute-force");

  await expect.poll(async () => {
    const { items } = await readAPI("/notifications");
    return items.map(({ incidentId, destination, status }) => `${incidentId}:${destination}:${status}`).sort();
  }, { timeout: 30_000 }).toEqual([
    `${userIncident.id}:audit:delivered`,
    `${adminIncident.id}:audit:delivered`,
    `${adminIncident.id}:rejected:exhausted`,
  ].sort());
  await expect.poll(async () => {
    const { items } = await readAPI("/deliveries");
    return items.some((item) => item.decisionId === userIncident.decisionId && item.status === "delivered");
  }, { timeout: 30_000 }).toBe(true);

  const { items } = await readAPI("/notifications");
  const rejected = items.find((item) => item.destination === "rejected");
  expect(rejected.attempts).toBe(1);
  expect(rejected.lastError).toBeTruthy();

  await page.goto(`${webui}/notifications`);
  const row = page.getByRole("row").filter({ has: page.getByRole("cell", { name: "rejected", exact: true }) });
  await expect(row).toContainText(/exhausted/i);
  await row.getByText("Last provider error").click();
  await expect(row).toContainText(rejected.lastError);
  await row.getByRole("button", { name: "Retry notification" }).click();
  await expect(page.getByText("Notification retry queued. Delivery has not yet been acknowledged.")).toBeVisible();
  await expect.poll(async () => {
    const { items } = await readAPI(`/notifications/${rejected.id}/attempts`);
    return items.map((item) => item.number);
  }, { timeout: 30_000 }).toEqual([2, 1]);

  await page.goto(`${webui}/notifications`);
  await row.getByRole("link", { name: "View attempts" }).click();
  await expect(page.getByRole("heading", { name: "Notification attempts" })).toBeVisible();
  await expect(page.locator("tbody tr")).toHaveCount(2);
  await expect(page.locator("tbody")).toContainText("Failed");

  await page.goto(`${webui}/maintenance`);
  await expect(page.getByRole("heading", { name: "Enforcement", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Notifications", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Maintenance", exact: true })).toBeVisible();
  await expect(page.getByText("Not configured", { exact: true })).toHaveCount(0);

  await page.goto(webui);
  await expect(page.getByRole("table")).toContainText(ip);
  await expect(page.getByRole("table")).toContainText("banned");
  await page.goto(`${webui}/bans/${userIncident.decisionId}`);
  await page.getByRole("button", { name: "Release all ban sources" }).click();
  await expect(page.getByRole("heading", { name: "Current bans" })).toBeVisible();
  await expect(page.getByRole("table").getByText(ip, { exact: true })).toHaveCount(0);
  await expect.poll(async () => {
    const { items } = await readAPI("/deliveries");
    return items.some((item) => item.decisionId === userIncident.decisionId && item.desiredState === "unbanned" && item.status === "delivered");
  }, { timeout: 30_000 }).toBe(true);
});
