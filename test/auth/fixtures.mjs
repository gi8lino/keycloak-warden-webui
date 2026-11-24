import { test as base, expect } from "@playwright/test";
import { execFileSync, spawn } from "node:child_process";
import { once } from "node:events";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import http from "node:http";
import { tmpdir } from "node:os";
import path from "node:path";

export const setupToken = "e2e-only-bootstrap-token-32-characters";
export const localPassword = "test-local-password";
export const subject = "36a15cdc-ed1b-489e-ab8c-198790c9e920";
export const providerPassword = "test-provider-password";

// Listen on a kernel-assigned loopback port and register deterministic cleanup.
async function listen(handler, cleanup) {
  const server = http.createServer(handler);
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  cleanup.push(
    () =>
      new Promise((resolve) => {
        server.close(resolve);
        server.closeAllConnections();
      }),
  );
  return `http://127.0.0.1:${server.address().port}`;
}

// Reserve distinct addresses before handing them to the Web UI process.
async function address() {
  const cleanup = [];
  const url = await listen(undefined, cleanup);
  await cleanup[0]();
  return new URL(url);
}

function docker(...args) {
  return execFileSync("docker", args, {
    encoding: "utf8",
    timeout: 120_000,
  }).trim();
}

// Import a disposable realm with one confidential PKCE client and two real accounts.
async function startKeycloak(directory, callback, cleanup) {
  const realm = JSON.parse(
    await readFile(
      new URL("../../deploy/keycloak-webui-dev.json", import.meta.url),
      "utf8",
    ),
  );
  realm.clients[0].redirectUris = [callback];
  const imports = path.join(directory, "import");
  await mkdir(imports);
  const file = path.join(imports, "realm.json");
  await writeFile(file, JSON.stringify(realm), { mode: 0o644 });
  const id = docker(
    "create",
    "-p",
    "127.0.0.1::8080",
    "quay.io/keycloak/keycloak:26.8.1",
    "start-dev",
    "--import-realm",
  );
  cleanup.push(() => docker("rm", "-f", id));
  docker("cp", imports, id + ":/opt/keycloak/data/");
  docker("start", id);
  const issuer =
    "http://" + docker("port", id, "8080/tcp") + "/realms/warden-test";
  await expect
    .poll(
      async () => {
        try {
          return (await fetch(issuer + "/.well-known/openid-configuration"))
            .status;
        } catch {
          return 0;
        }
      },
      { timeout: 90_000 },
    )
    .toBe(200);
  return issuer;
}

export const test = base.extend({
  lab: async ({}, use) => {
    const directory = await mkdtemp(path.join(tmpdir(), "warden-login-"));
    const cleanup = [() => rm(directory, { recursive: true, force: true })];
    try {
      const url = await address();
      const management = await address();
      let backendFailure;
      let historyPages;
      let decisionRevisions;
      let deliveryOverview;
      const historyRequests = [];
      const warden = await listen((req, res) => {
        if (backendFailure) {
          res
            .writeHead(backendFailure, { "Content-Type": "application/json" })
            .end(JSON.stringify({ error: "secret backend response" }));
          return;
        }
        if (historyPages && req.method === "GET") {
          const requested = new URL(req.url, "http://warden.test");
          if (requested.pathname === "/bans/history-test") {
            historyRequests.push(requested.search);
            const page = Number(
              requested.searchParams.get("history-page") || "1",
            );
            res
              .writeHead(200, { "Content-Type": "application/json" })
              .end(JSON.stringify(historyPages[page - 1]));
            return;
          }
        }
        if (req.method === "GET") {
          const requested = new URL(req.url, "http://warden.test");
          if (requested.pathname === "/decisions/history") {
            res
              .writeHead(200, { "Content-Type": "application/json" })
              .end(JSON.stringify(decisionRevisions || { items: [], page: 1 }));
            return;
          }
          if (requested.pathname === "/deliveries") {
            res
              .writeHead(200, { "Content-Type": "application/json" })
              .end(JSON.stringify(deliveryOverview || { items: [], page: 1 }));
            return;
          }
        }
        if (req.method !== "GET" || req.url !== "/bans") {
          res.writeHead(404).end();
          return;
        }
        res.writeHead(200, { "Content-Type": "application/json" }).end("[]");
      }, cleanup);

      await use({
        url: url.origin,
        setDecisionRevisions: (page) => {
          decisionRevisions = page;
        },
        setDeliveryOverview: (page) => {
          deliveryOverview = page;
        },
        setHistoryPages: (pages) => {
          historyPages = pages;
        },
        historyRequests,
        failBackend: (status) => {
          backendFailure = status;
        },
        keycloak: () =>
          startKeycloak(directory, url.origin + "/auth/callback", cleanup),
        // Simulate a trusted ingress that overwrites browser-supplied identity headers.
        proxy: (name, groups) =>
          listen((req, res) => {
            const upstream = http.request(
              new URL(req.url, url),
              {
                method: req.method,
                headers: {
                  "x-forwarded-user": name,
                  "x-forwarded-groups": groups,
                },
              },
              (reply) => {
                res.writeHead(reply.statusCode, reply.headers);
                reply.pipe(res);
              },
            );
            upstream.on("error", () => res.writeHead(502).end());
            req.pipe(upstream);
          }, cleanup),
        start: async (mode, flags = []) => {
          const child = spawn(
            process.env.E2E_WEBUI_BINARY || "build/warden-webui",
            [
              "--listen-address=" + url.host,
              "--management-listen-address=" + management.host,
              "--public-url=" + url.origin,
              "--database=" + path.join(directory, "admin.db"),
              "--warden-url=" + warden,
              "--warden-admin-token=e2e-only",
              "--setup-token=" + setupToken,
              "--auth-mode=" + mode,
              ...flags,
            ],
            {
              stdio: ["ignore", "pipe", "pipe"],
              env: Object.fromEntries(
                Object.entries(process.env).filter(
                  ([key]) => !key.startsWith("WARDEN_WEBUI__"),
                ),
              ),
            },
          );
          let logs = "";
          for (const stream of [child.stdout, child.stderr])
            stream.on("data", (chunk) => {
              logs += chunk;
            });
          const stopped = once(child, "exit");
          cleanup.push(async () => {
            if (child.exitCode === null && child.signalCode === null)
              child.kill();
            await stopped;
          });
          await expect
            .poll(async () => {
              if (child.exitCode !== null)
                throw new Error("Web UI exited: " + logs);
              try {
                return (await fetch(management.origin + "/readyz")).status;
              } catch {
                return 0;
              }
            })
            .toBe(204);
        },
      });
    } finally {
      for (const close of cleanup.reverse()) await close();
    }
  },
});
