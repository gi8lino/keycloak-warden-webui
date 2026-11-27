# Warden WebUI: local end-to-end stack

This development-only environment runs WebUI, the pinned Warden release and Hookglass. The optional OIDC overlay starts Keycloak for real browser sign-in. Warden's own E2E suite tests Keycloak → Snitch; this suite tests WebUI → Warden against the actual API and signed schema-v2 event ingestion without requiring a second repository checkout.

## Start

From the WebUI repository root:

```sh
make e2e-up          # build WebUI and start the published Warden image and Hookglass
make e2e-logs        # follow service logs
make e2e-down        # stop services, preserve SQLite volumes
make e2e-reset       # stop services and delete demo volumes
make e2e-oidc-up     # separate Keycloak OIDC stack
make e2e-oidc-down   # stop the OIDC stack
```

Open http://127.0.0.1:18092/setup and use `development-setup-token-0123456789abcdef` to create a local administrator. For OIDC, use http://127.0.0.1:18192/setup and sign in to Keycloak as `operator` / `test-provider-password`. The `visitor` account is intentionally unauthorized. All credentials are local demo values; never reuse them in production.

## Run automated tests

Install Docker with Compose, Go, Node.js, GNU Make and curl. Install the browser dependencies:

```sh
npm ci
npx playwright install chromium
```

On Linux, add `--with-deps` if Chromium system dependencies are missing.

```sh
make e2e-test       # reset the base stack, start it and exercise real Warden APIs
make e2e-auth-test  # isolated Playwright authentication and UI fixtures
```

**`e2e-test` deletes the base stack's test volumes** to start with a fresh setup. It uses `compose.test.yaml` and `warden-test.yaml`, which configure a deliberately rejected incident notification alongside a working destination. The browser test creates an administrator, sends authenticated user and admin events, verifies automatic bans, filters incidents, observes successful and exhausted deliveries, retries a failed notification and checks attempt history and worker status, then releases the ban and confirms enforcement delivery. The retry is deliberately rejected again so the test verifies that both attempts remain recorded. The stack remains running for inspection. To return to the normal demo with a clean database, run `make e2e-reset && make e2e-up`. This also removes any test-only pending delivery obligations.

`e2e-auth-test` builds a local WebUI binary, uses isolated temporary databases and a fake Warden API, and launches real Keycloak only for OIDC browser tests. It does not require the Compose demo. Both suites write Playwright output under `build/`.

## Endpoints

| Service | Local URL |
| --- | --- |
| WebUI | http://127.0.0.1:18092/ |
| WebUI management | http://127.0.0.1:18093/readyz |
| Warden | http://127.0.0.1:18081/ |
| Warden management | http://127.0.0.1:18082/readyz |
| Hookglass | http://127.0.0.1:18090/header |
| Keycloak (OIDC stack only) | http://127.0.0.1:18180/realms/warden-test |

The OIDC stack uses ports 18181, 18182, 18190, 18192 and 18193 for the corresponding services. Override `E2E_PROJECT`, `E2E_WARDEN_PORT`, `E2E_WARDEN_MANAGEMENT_PORT`, `E2E_HOOKGLASS_PORT`, `E2E_WEBUI_PORT`, `E2E_WEBUI_MANAGEMENT_PORT` and `E2E_KEYCLOAK_PORT` through Make as needed. `E2E_WARDEN_IMAGE` overrides the published Warden image; keep it compatible with the Go client in `go.mod`.

WebUI and Hookglass share Warden's network namespace to keep internal HTTP connections on loopback. Stop and restart the entire stack rather than recreating Warden alone. Production Kubernetes manifests remain under `deploy/kubernetes/`.
