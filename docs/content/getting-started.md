# Getting started

The quickest way to try Warden Web UI is the repository's **local Docker Compose stack**, which starts Warden, Hookglass (a webhook receiver), and Web UI. This demo does **not** start Keycloak or Snitch; use Warden's own E2E stack to test real Keycloak login events.

## Prerequisites

Install Docker with the Compose plugin and GNU Make. The demo pulls a published Warden container image, so a separate Warden source checkout is **not** required.

## Start the demo

```sh
git clone https://github.com/gi8lino/keycloak-warden-webui.git
cd keycloak-warden-webui
make e2e-up
```

The demo uses the Warden image pinned in `deploy/compose.yaml`. To test against a different Warden image (for example one you built and tagged locally), override the image:

```sh
E2E_WARDEN_IMAGE=warden:dev make e2e-up
```

The published demo ports remain 18092/18093 (18192/18193 for OIDC); they map to Web UI container ports 8082/8083.

Only the Web UI image is built from this repository; Warden is pulled as a prebuilt image.

Open **[http://127.0.0.1:18092/setup](http://127.0.0.1:18092/setup)**. The demo-only setup token is `development-setup-token-0123456789abcdef`. Choose an administrator username and a password of 12–72 bytes.

After setup, you can create a manual ban for the documentation IP `203.0.113.42` with duration `15m` and a reason, inspect it, and release it. Hookglass receives the resulting webhook notifications.

```sh
make e2e-logs   # Follow the services
make e2e-down   # Stop; keep the demo volumes
make e2e-reset  # Stop and delete demo volumes
```

**Never reuse demo credentials or tokens in production.**

## Try Keycloak sign-in

```sh
make e2e-oidc-up
```

Open **[http://127.0.0.1:18192/setup](http://127.0.0.1:18192/setup)**, initialize the local account with the same demo-only setup token, then continue to Keycloak and sign in as `operator` with password `test-provider-password`. The demo links the initial account to this user. The `visitor` account is intentionally unauthorized.

Stop this separate OIDC stack with `make e2e-oidc-down` (or reset with `make e2e-oidc-reset`). Both demos listen on loopback addresses only.

## Run the application directly

Start a compatible Warden instance with its administrator API token configured, then from the Web UI repository:

```sh
export WARDEN_WEBUI__WARDEN_ADMIN_TOKEN='YOUR_WARDEN_ADMIN_TOKEN'
export WARDEN_WEBUI__SETUP_TOKEN="$(openssl rand -hex 32)"
make run
```

`make run` prints the assigned local URL; open its `/setup` path for first-time initialization. By default, Web UI connects to Warden at `http://127.0.0.1:8080`; set `WARDEN_WEBUI__WARDEN_URL` if necessary. Use a Warden release compatible with the Go client dependency in `go.mod`.

Read [Configuration](configuration.md) and [Authentication](authentication.md) before deploying outside a development environment.
