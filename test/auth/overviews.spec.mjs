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

test("ban history shows recorded releases and source snapshots", async ({ lab, page }) => {
  lab.setDecisionRevisions({
    page: 1,
    items: [{
      decisionId: "decision-test", ipAddress: "203.0.113.42", revision: 2,
      state: "unbanned", changedAt: "2026-11-24T13:00:00Z", sources: [],
    }, {
      decisionId: "decision-test", ipAddress: "203.0.113.42", revision: 1,
      state: "banned", changedAt: "2026-11-24T12:00:00Z",
      sources: [{ kind: "manual", reason: "investigation", expiresAt: "2026-11-25T12:00:00Z" }],
    }],
  });
  await signIn(lab, page);
  await page.getByRole("link", { name: "Ban history" }).first().click();
  await expect(page.getByRole("heading", { name: "Ban history" })).toBeVisible();
  await expect(page.locator("#history-ip")).toBeVisible();
  await expect(page.locator("table")).toContainText("investigation");
  await expect(page.locator("table")).toContainText("unbanned");
});

test("deliveries highlights exhausted release with a safe provider error", async ({ lab, page }) => {
  lab.setDeliveryOverview({
    page: 1,
    items: [{
      decisionId: "decision-test", ipAddress: "203.0.113.43", revision: 4,
      desiredState: "unbanned", destination: "cloudflare.edge", status: "exhausted",
      acknowledgedRevision: 3, attemptCount: 3, lastError: "<script>alert(1)</script>",
    }],
  });
  await signIn(lab, page);
  await page.getByRole("link", { name: "Deliveries" }).first().click();
  await expect(page.getByRole("heading", { name: "Deliveries" })).toBeVisible();
  await expect(page.locator("table")).toContainText("cloudflare.edge");
  await expect(page.locator("table")).toContainText("exhausted");
  await expect(page.locator("table")).toContainText("unbanned");
  await page.getByText("Last provider error").click();
  await expect(page.locator("table")).toContainText("<script>alert(1)</script>");
  expect(await page.locator("table script").count()).toBe(0);
});
