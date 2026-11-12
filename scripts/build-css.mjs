// Generate the standalone Tailwind stylesheet from classes used in Go templates.
// The generated output is embedded by Go; Node is not needed at runtime.
import { readFile, readdir, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { compile } from "tailwindcss";

const require = createRequire(import.meta.url);
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const templates = resolve(root, "web/templates");
const theme = await readFile(require.resolve("tailwindcss/theme.css"), "utf8");
const preflight = await readFile(
  require.resolve("tailwindcss/preflight.css"),
  "utf8",
);
const tokens = new Set();

for (const name of await readdir(templates)) {
  if (!name.endsWith(".html")) continue;
  const contents = await readFile(resolve(templates, name), "utf8");
  for (const match of contents.matchAll(/\bclass\s*=\s*"([^"]*)"/g)) {
    for (const candidate of match[1].split(/\s+/)) {
      if (candidate && !candidate.includes("{{")) tokens.add(candidate);
    }
  }
}

const sheet = await compile(
  `@layer theme, base, utilities;\n@layer theme {\n${theme}\n}\n@layer base {\n${preflight}\n}\n@tailwind utilities;`,
);
await writeFile(
  resolve(root, "web/static/css/style.css"),
  sheet.build([...tokens].sort()),
);
console.log(`Tailwind: generated ${tokens.size} utilities from HTML templates`);
