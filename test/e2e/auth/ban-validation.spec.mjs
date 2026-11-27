import { expect } from "@playwright/test";
import { localPassword, setupToken, test } from "./fixtures.mjs";

test("manual ban validates addresses and durations before sending", async ({
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

  let submissions = 0;
  await page.route("**/bans", async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    submissions++;
    await route.fulfill({ status: 303, headers: { location: "/" } });
  });
  const ip = page.locator("#ban-ip");
  const duration = page.locator("#ban-duration");
  await page.getByLabel("Reason", { exact: true }).fill("test");

  await test.step("invalid IP shows an accessible error and blocks submission", async () => {
    await ip.fill("999.2.3.4");
    await page.getByRole("button", { name: "Create ban" }).click();
    await expect(ip).toHaveAttribute("aria-invalid", "true");
    await expect(page.locator("#ban-ip-error")).toBeVisible();
    await expect(ip).toHaveCSS("border-top-color", "rgb(185, 28, 28)");
    await expect(page.locator("#page-loading")).toBeHidden();
    expect(submissions).toBe(0);
    expect(
      await ip.evaluate((field) => {
        field.dispatchEvent(new Event("invalid"));
        return field
          .getAnimations()
          .some((animation) => animation.id === "validation-shake");
      }),
    ).toBe(true);
    await page.screenshot({ path: "build/ban-validation.png" });
  });

  await test.step("IPv6 is accepted and CIDR notation is rejected", async () => {
    await ip.fill("2001:db8::1");
    await expect(ip).toHaveAttribute("aria-invalid", "false");
    await ip.fill("203.0.113.1/24");
    await expect(ip).toHaveAttribute("aria-invalid", "true");
    await ip.fill("203.0.113.1");
  });

  await test.step("invalid and overflowing durations block submission", async () => {
    await duration.fill("1d");
    await page.getByRole("button", { name: "Create ban" }).click();
    await expect(duration).toHaveAttribute("aria-invalid", "true");
    await expect(page.locator("#ban-duration-error")).toBeVisible();
    await duration.fill("0s");
    expect(await duration.evaluate((field) => field.validity.valid)).toBe(
      false,
    );
    await duration.fill("9223372036854775808ns");
    expect(await duration.evaluate((field) => field.validity.valid)).toBe(
      false,
    );
    await duration.fill("-1h");
    expect(await duration.evaluate((field) => field.validity.valid)).toBe(
      false,
    );
    expect(submissions).toBe(0);
  });

  await test.step("reduced motion suppresses shaking", async () => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await duration.evaluate((field) =>
      field.getAnimations().forEach((animation) => animation.cancel()),
    );
    await duration.blur();
    expect(
      await duration.evaluate(
        (field) =>
          field
            .getAnimations()
            .filter((animation) => animation.id === "validation-shake").length,
      ),
    ).toBe(0);
  });

  await test.step("correcting the fields clears errors and permits submission", async () => {
    await duration.fill("1h30m");
    await expect(duration).toHaveAttribute("aria-invalid", "false");
    await expect(page.locator("#ban-duration-error")).toBeHidden();
    await page.getByRole("button", { name: "Create ban" }).click();
    await expect.poll(() => submissions).toBe(1);
  });
});
