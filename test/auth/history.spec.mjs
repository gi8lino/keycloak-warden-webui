import { expect } from "@playwright/test";
import { localPassword, setupToken, test } from "./fixtures.mjs";

test("decision delivery history shows attempts and navigates pinned pages", async ({
  lab,
  page,
}) => {
  const decision = {
    id: "history-test",
    ipAddress: "203.0.113.42",
    revision: 3,
    state: "unbanned",
  };
  const deliveries = [
    { destination: "webhook.slack", acknowledgedRevision: 2 },
  ];
  lab.setHistoryPages([
    {
      decision,
      deliveries,
      sources: [],
      historyPage: 1,
      historyNextPage: 2,
      deliveryHistory: [
        {
          destination: "webhook.slack",
          revision: 3,
          desiredState: "unbanned",
          status: "retrying",
          nextAttemptAt: "2026-11-24T12:15:00Z",
          attempts: [
            {
              number: 1,
              at: "2026-11-24T12:00:00Z",
              outcome: "failed",
              error: "Slack: not_authed",
            },
            {
              number: 2,
              at: "2026-11-24T12:05:00Z",
              outcome: "failed",
              error: "<script>unsafe()</script>",
            },
          ],
        },
        {
          destination: "webhook.other",
          revision: 3,
          desiredState: "unbanned",
          status: "exhausted",
          attempts: [],
        },
      ],
    },
    {
      decision,
      deliveries,
      sources: [],
      historyPage: 2,
      deliveryHistory: [
        {
          destination: "webhook.slack",
          revision: 2,
          desiredState: "banned",
          status: "delivered",
          acknowledgedAt: "2026-11-24T11:00:00Z",
          attemptsTruncated: true,
          attempts: [
            { number: 4, at: "2026-11-24T11:00:00Z", outcome: "succeeded" },
          ],
        },
      ],
    },
  ]);
  await lab.start("local");
  await page.goto(lab.url + "/setup");
  await page.getByLabel("Setup token").fill(setupToken);
  await page.getByLabel("Username").fill("founder");
  await page.locator("#setup-password").fill(localPassword);
  await page.getByLabel("Confirm password").fill(localPassword);
  await page.getByRole("button", { name: "Create administrator" }).click();
  await expect(
    page.getByRole("heading", { name: "Active bans", exact: true }),
  ).toBeVisible();

  await page.goto(lab.url + "/bans/history-test");
  const history = page.locator("#delivery-history");
  const current = page.locator("#current-deliveries");
  const unacknowledged = current.locator(
    '[data-delivery-destination="webhook.other"]',
  );
  await expect(
    page.getByRole("region", { name: "Decision summary" }),
  ).toContainText("Revision 3");
  await expect(
    page.getByRole("heading", { name: "Active sources", exact: true }),
  ).toBeVisible();
  await expect(
    current.getByRole("heading", { name: "Current delivery", exact: true }),
  ).toBeVisible();
  const sourcesPanel = page.getByRole("region", { name: "Active sources" });
  const desktopSources = await sourcesPanel.boundingBox();
  const desktopCurrent = await current.boundingBox();
  expect(desktopSources).not.toBeNull();
  expect(desktopCurrent).not.toBeNull();
  expect(Math.abs(desktopSources.y - desktopCurrent.y)).toBeLessThan(5);
  await expect(unacknowledged).toContainText("exhausted");
  await expect(unacknowledged).toContainText("—");
  const destinationFont = await unacknowledged
    .locator("h3")
    .evaluate((node) => getComputedStyle(node).fontFamily);
  const headingFont = await current
    .locator("h2")
    .evaluate((node) => getComputedStyle(node).fontFamily);
  expect(destinationFont).toBe(headingFont);
  await expect(
    history.getByRole("heading", { name: "Delivery history", exact: true }),
  ).toBeVisible();
  await expect(
    history.getByRole("list", { name: "Delivery history timeline" }),
  ).toBeVisible();
  await expect(history.getByText("retrying", { exact: true })).toBeVisible();
  await expect(history.getByText("exhausted", { exact: true })).toBeVisible();
  await expect(history.locator("details[open]").first()).toContainText(
    "Attempt 2",
  );
  await history.getByText("Failure details", { exact: true }).first().click();
  await expect(history).toContainText("<script>unsafe()</script>");
  expect(await history.locator("script").count()).toBe(0);
  await page.screenshot({ path: "build/delivery-history-desktop.png" });

  await history.getByRole("link", { name: "Older history" }).click();
  await expect(page).toHaveURL(/history-page=2&history-through-revision=3/);
  await expect(history).toContainText("Revision 2");
  await expect(history).toContainText(
    "Older attempts in this group were omitted",
  );
  await expect(
    history.getByRole("link", { name: "Older history" }),
  ).toHaveCount(0);
  expect(lab.historyRequests).toContain(
    "?history-page=2&history-through-revision=3",
  );

  await page.setViewportSize({ width: 390, height: 844 });
  const mobileSources = await sourcesPanel.boundingBox();
  const mobileCurrent = await current.boundingBox();
  expect(mobileSources).not.toBeNull();
  expect(mobileCurrent).not.toBeNull();
  expect(mobileCurrent.y).toBeGreaterThan(mobileSources.y);
  await history.getByRole("link", { name: "Newer history" }).click();
  await expect(history).toContainText("Revision 3");
  await history.getByRole("link", { name: "Refresh latest" }).click();
  await expect(page).toHaveURL(/\/bans\/history-test#delivery-history$/);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(
    390,
  );
  await page.screenshot({ path: "build/delivery-history-mobile.png" });
});
