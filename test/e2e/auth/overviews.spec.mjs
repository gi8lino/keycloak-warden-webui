import { expect } from "@playwright/test";
import { localPassword, setupToken, test } from "./fixtures.mjs";

async function signIn(lab, page) {
  await lab.start("local");
  await page.goto(lab.url + "/setup");
  await page.getByLabel("Setup token").fill(setupToken);
  await page.getByLabel("Username").fill("founder");
  await page.locator("#setup-password").fill(localPassword);
  await page.getByLabel("Confirm password").fill(localPassword);
  await page.getByRole("button", { name: "Create administrator" }).click();
}

test("ban history shows recorded releases and source snapshots", async ({
  lab,
  page,
}) => {
  lab.setDecisionRevisions({
    page: 1,
    items: [
      {
        decisionId: "decision-test",
        ipAddress: "203.0.113.42",
        revision: 2,
        state: "unbanned",
        changedAt: "2026-11-24T13:00:00Z",
        sources: [],
      },
      {
        decisionId: "decision-test",
        ipAddress: "203.0.113.42",
        revision: 1,
        state: "banned",
        changedAt: "2026-11-24T12:00:00Z",
        sources: [
          {
            kind: "manual",
            reason: "investigation",
            expiresAt: "2026-11-25T12:00:00Z",
          },
        ],
      },
    ],
  });
  await signIn(lab, page);
  await page.getByRole("link", { name: "Ban history" }).first().click();
  await expect(
    page.getByRole("heading", { name: "Ban history" }),
  ).toBeVisible();
  await expect(page.locator("#history-ip")).toBeVisible();
  const clear = page.getByRole("link", { name: "Clear filters" });
  await expect(clear).toHaveAttribute("href", "/ban-history");
  await expect(clear.locator("svg")).toHaveCount(1);
  await expect(page.locator("table")).toContainText("investigation");
  await expect(page.locator("table")).toContainText("unbanned");
  const inspect = page.getByRole("link", { name: "Inspect decision" }).first();
  await expect(inspect).toHaveAttribute("href", /\/bans\/decision-test$/);
  await expect(inspect.locator("svg")).toHaveCount(1);
  await expect(
    page.getByRole("button", { name: "Apply filters" }),
  ).toBeVisible();
});

test("deliveries highlights exhausted release with a safe provider error", async ({
  lab,
  page,
}) => {
  lab.setDeliveryHealth([
    {
      destination: "cloudflare.edge",
      pending: 0,
      retrying: 0,
      exhausted: 1,
      failedUnbans: 1,
      exhaustedUnbans: 1,
      delivered: 3,
    },
  ]);
  lab.setDeliveryOverview({
    page: 1,
    items: [
      {
        decisionId: "decision-test",
        ipAddress: "203.0.113.43",
        revision: 4,
        desiredState: "unbanned",
        destination: "cloudflare.edge",
        status: "exhausted",
        acknowledgedRevision: 3,
        attemptCount: 3,
        lastError: "<script>alert(1)</script>",
      },
    ],
  });
  await signIn(lab, page);
  await page.getByRole("link", { name: "Deliveries" }).first().click();
  await expect(page.getByRole("heading", { name: "Deliveries" })).toBeVisible();
  const health = page.getByRole("region", { name: "Destination health" });
  await expect(
    health.getByRole("row").filter({ hasText: "cloudflare.edge" }),
  ).toContainText("1");
  const deliveries = page.locator("table").last();
  await expect(deliveries).toContainText("cloudflare.edge");
  await expect(deliveries).toContainText("exhausted");
  await expect(deliveries).toContainText("unbanned");
  await expect(
    page.getByRole("button", { name: "Retry delivery" }),
  ).toBeVisible();
  await page.getByText("Last provider error").click();
  await expect(deliveries).toContainText("<script>alert(1)</script>");
  expect(await deliveries.locator("script").count()).toBe(0);
  const refresh = page.getByRole("link", { name: "Refresh latest deliveries" });
  await expect(refresh).toHaveAttribute("href", /\/deliveries\?page=1$/);
  await expect(refresh.locator("svg")).toHaveCount(1);
  const inspect = page.getByRole("link", { name: "Inspect decision" });
  await expect(inspect).toHaveAttribute(
    "href",
    /\/bans\/decision-test#delivery-history$/,
  );
  await expect(inspect.locator("svg")).toHaveCount(1);
});


test("incidents show administrative rule hits and hypothetical bans", async ({ lab, page }) => {
  lab.setIncidents({
    nextBeforeId: 8,
    items: [
      { id: 9, rule: "role-changes", eventKind: "admin", operationType: "CREATE", resourceType: "REALM_ROLE_MAPPING", realmId: "hq", groupBy: "realm", groupKey: "hq", count: 1, threshold: 1, window: "1m" },
      { id: 8, rule: "brute-force", eventKind: "user", eventType: "LOGIN_ERROR", realmId: "hq", groupBy: "ip", groupKey: "203.0.113.42", count: 5, threshold: 5, window: "1m", observeOnly: true, banUntil: "2026-11-27T12:15:00Z" },
    ],
  });
  await signIn(lab, page);
  await page.getByRole("link", { name: "Incidents", exact: true }).first().click();
  await expect(page.getByRole("heading", { name: "Incidents" })).toBeVisible();
  await expect(page.getByRole("link", { name: /^Refresh latest/ })).toHaveCount(1);
  await expect(page.getByRole("link", { name: "Refresh latest incidents" })).toHaveAttribute("href", "/incidents");
  await expect(page.locator("table")).toContainText("CREATE REALM_ROLE_MAPPING");
  await expect(page.locator("table")).toContainText("Hypothetical ban");
  await expect(page.getByRole("link", { name: "Older results" })).toHaveAttribute("href", "/incidents?before-id=8");
});

test("notification deliveries expose failed sends safely", async ({ lab, page }) => {
  lab.setNotifications({ items: [{ id: 7, incidentId: 9, destination: "slack-security", status: "exhausted", attempts: 3, lastError: "<script>alert(1)</script>" }] });
  lab.setNotificationAttempts({ items: [{ number: 3, completedAt: "2026-11-27T12:00:00Z", error: "receiver unavailable" }] });
  await signIn(lab, page);
  await page.getByRole("link", { name: "Notifications", exact: true }).first().click();
  await expect(page.getByRole("heading", { name: "Notifications" })).toBeVisible();
  await expect(page.getByRole("link", { name: /^Refresh latest/ })).toHaveCount(1);
  await expect(page.getByRole("link", { name: "Refresh latest notification deliveries" })).toHaveAttribute("href", "/notifications");
  await expect(page.getByRole("button", { name: "Retry notification" })).toBeVisible();
  await page.getByText("Last provider error").click();
  await expect(page.locator("table")).toContainText("<script>alert(1)</script>");
  expect(await page.locator("table script").count()).toBe(0);
  await page.getByRole("link", { name: "View attempts" }).click();
  await expect(page.getByRole("heading", { name: "Notification attempts" })).toBeVisible();
  await expect(page.getByRole("link", { name: /^Refresh latest/ })).toHaveCount(1);
  await expect(page.getByRole("link", { name: "Refresh latest notification attempts" })).toHaveAttribute("href", "/notifications/7/attempts");
  await expect(page.locator("table")).toContainText("receiver unavailable");
});
