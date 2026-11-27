import { expect } from "@playwright/test";
import { localPassword, setupToken, test } from "./fixtures.mjs";

test("security incidents use the new API and cursor pagination", async ({
  lab,
  page,
}) => {
  lab.setIncidentPages({
    0: {
      items: [
        {
          id: 42,
          rule: "brute-force",
          eventKind: "user",
          eventType: "LOGIN_ERROR",
          groupBy: "ip",
          groupKey: "192.0.2.1",
          at: "2026-11-27T12:00:00Z",
          count: 5,
          threshold: 5,
          banUntil: "2026-11-27T12:15:00Z",
          observeOnly: true,
        },
      ],
      nextBeforeId: 42,
    },
    42: {
      items: [
        {
          id: 7,
          rule: "admin-audit",
          eventKind: "admin",
          operationType: "DELETE",
          resourceType: "USER",
          at: "2026-11-27T11:00:00Z",
          count: 1,
          threshold: 1,
        },
      ],
    },
  });
  await lab.start("local");
  await page.goto(lab.url + "/incidents");
  await expect(
    page.getByRole("heading", { name: "Incidents", level: 1 }),
  ).toHaveCount(0);
  await page.goto(lab.url + "/setup");
  await page.getByLabel("Setup token").fill(setupToken);
  await page.getByLabel("Username").fill("founder");
  await page.locator("#setup-password").fill(localPassword);
  await page.getByLabel("Confirm password").fill(localPassword);
  await page.getByRole("button", { name: "Create administrator" }).click();
  await page.getByRole("link", { name: "View incidents" }).click();
  await expect(
    page.getByText("Hypothetical ban", { exact: false }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Older results" }).click();
  await expect(page.getByText("admin-audit")).toBeVisible();
  await expect(page.getByText("No ban requested", { exact: true })).toBeVisible();
  expect(lab.incidentRequests).toEqual(["", "?before-id=42"]);
  await page.goto(lab.url + "/observations");
  await expect(
    page.getByRole("heading", { name: "Incidents", level: 1 }),
  ).toBeVisible();
});
