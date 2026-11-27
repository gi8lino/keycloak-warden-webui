import { expect } from "@playwright/test";
import { test } from "./fixtures.mjs";

// Verify real overflow keeps the shell stationary at desktop and mobile sizes.
async function checkScrolling(page, width, height) {
  await page.setViewportSize({ width, height });
  await page.locator("#page-content").evaluate((element) => {
    const filler = document.createElement("div");
    filler.textContent = "Long page content ".repeat(3000);
    element.append(filler);
  });
  const heading = await page.locator("#page-heading").boundingBox();
  const footer = await page.locator("footer").boundingBox();
  const sidebar =
    width >= 768 ? await page.locator("aside").boundingBox() : null;

  await page.locator("#main-content").evaluate((element) => {
    element.scrollTop = element.scrollHeight;
  });
  expect(
    await page
      .locator("#main-content")
      .evaluate((element) => element.scrollTop),
  ).toBeGreaterThan(0);
  expect(await page.evaluate(() => window.scrollY)).toBe(0);
  expect(await page.locator("#page-heading").boundingBox()).toEqual(heading);
  expect(await page.locator("footer").boundingBox()).toEqual(footer);
  expect(footer.y + footer.height).toBeLessThanOrEqual(height);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(
    width,
  );
  if (sidebar)
    expect(await page.locator("aside").boundingBox()).toEqual(sidebar);
}

test("fixed shell, version footer, and custom 404", async ({ lab, page }) => {
  await lab.start("local");
  await page.goto(lab.url + "/setup");
  await expect(page.locator("footer")).toContainText(/Version \S+/);
  await checkScrolling(page, 1280, 800);
  await page.screenshot({ path: "build/layout-desktop.png" });

  await page.reload();
  await checkScrolling(page, 390, 844);
  await page.screenshot({ path: "build/layout-mobile.png" });

  const response = await page.goto(lab.url + "/missing");
  expect(response.status()).toBe(404);
  await expect(
    page.getByRole("heading", { name: "Page not found" }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Back to Warden" }),
  ).toHaveAttribute("href", "/");
  await expect(page.locator("footer")).toBeInViewport();
  expect(
    (await page.locator("footer").boundingBox()).y +
      (await page.locator("footer").boundingBox()).height,
  ).toBeLessThanOrEqual(844);
  await page.screenshot({ path: "build/layout-not-found.png" });
});
