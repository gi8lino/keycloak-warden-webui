import { expect } from "@playwright/test";
import { localPassword, setupToken, test } from "./fixtures.mjs";

test("backend read and action failures render safe styled pages", async ({
  lab,
  page,
}) => {
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

  lab.failBackend(503);
  const failed = await page.goto(lab.url + "/");
  expect(failed.status()).toBe(503);
  await expect(page.getByRole("alert")).toContainText("currently unavailable");
  await expect(page.locator("body")).not.toContainText(
    "secret backend response",
  );
  await expect(page.locator("footer")).toBeInViewport();
  await page.screenshot({ path: "build/backend-error.png" });

  lab.failBackend(undefined);
  await page.getByRole("link", { name: "Back to Warden" }).click();
  await page.getByLabel("IP address", { exact: true }).fill("203.0.113.42");
  await page.getByLabel("Reason", { exact: true }).fill("test");
  lab.failBackend(409);
  const rejected = page.waitForResponse(
    (response) =>
      response.url() === lab.url + "/bans" &&
      response.request().method() === "POST",
  );
  await page.getByRole("button", { name: "Create ban" }).click();
  expect((await rejected).status()).toBe(409);
  await expect(page.getByRole("alert")).toContainText("Review the latest data");
  await expect(page.locator("body")).not.toContainText(
    "secret backend response",
  );
});
