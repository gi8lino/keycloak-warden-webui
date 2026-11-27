# Configuration

Web UI has **one CLI flag set** (no subcommands). All options are available as flags or as environment variables with the `WARDEN_WEBUI__` prefix. Run `warden-webui --help` to see the exact flags and defaults.

## Essential settings

| Environment variable                       | Purpose                                                                       |
| ------------------------------------------ | ----------------------------------------------------------------------------- |
| `WARDEN_WEBUI__WARDEN_URL`                 | Warden application API URL, default `http://127.0.0.1:8080`                   |
| `WARDEN_WEBUI__WARDEN_ADMIN_TOKEN`         | Private bearer token for Warden administration                                |
| `WARDEN_WEBUI__ALLOW_INSECURE_WARDEN_HTTP` | Allow non-loopback HTTP to Warden (default `false`; sends token in cleartext) |
| `WARDEN_WEBUI__WARDEN_REQUEST_TIMEOUT`     | Timeout for ordinary Warden API requests (default `10s`)                      |
| `WARDEN_WEBUI__WARDEN_MAINTENANCE_TIMEOUT` | Timeout for Warden maintenance requests (default `35s`)                       |
| `WARDEN_WEBUI__WARDEN_MAX_RESPONSE_BYTES`  | Maximum successful Warden API response size (default `1048576` bytes / 1 MiB) |
| `WARDEN_WEBUI__SETUP_TOKEN`                | One-time bootstrap token (at least 32 characters on first startup)            |
| `WARDEN_WEBUI__DATABASE`                   | Web UI's **own** SQLite store, default `warden-webui.db`                      |
| `WARDEN_WEBUI__LISTEN_ADDRESS`             | Browser-facing application listener, default `127.0.0.1:8082`                 |
| `WARDEN_WEBUI__MANAGEMENT_LISTEN_ADDRESS`  | Health listener, default `127.0.0.1:8083`                                     |
| `WARDEN_WEBUI__PUBLIC_URL`                 | Exact browser-visible URL, including scheme, host, port, and any path prefix  |
| `WARDEN_WEBUI__ROUTE_PREFIX`               | Optional application mount path, such as `/ui`                                |
| `WARDEN_WEBUI__AUTH_MODE`                  | Authentication mode: `local` (default), `oidc`, or `proxy`                    |

The admin API token stays in the Web UI process; it must **never** be put in browser code, URLs, HTML, ConfigMaps, or committed files. Web UI doesn't read or write Warden's SQLite file. Supply a separate persistent writable directory for its own database, including SQLite WAL/SHM files.

## Upstream connectivity

Use HTTPS for remotely deployed Warden upstreams by default. Loopback HTTP is always allowed, for example when Warden and WebUI share a Pod:

```sh
WARDEN_WEBUI__WARDEN_URL=http://127.0.0.1:8080
```

For standalone deployment through a trusted Kubernetes ClusterIP Service, Warden's shared client supports an explicit HTTP opt-in:

```sh
WARDEN_WEBUI__WARDEN_URL=http://keycloak-warden.keycloak.svc.cluster.local:8080
WARDEN_WEBUI__ALLOW_INSECURE_WARDEN_HTTP=true
```

The equivalent CLI flag is `--allow-insecure-warden-http=true` (or `=false` to disable it). Its default is `false`, and the CLI requires an explicit boolean value. Enable it only when necessary: the Warden administrator bearer token travels unencrypted over the connection. Apply a restrictive Cilium network policy between WebUI and Warden; the option does not disable TLS verification for HTTPS URLs.

### Administrative HTTP client limits

Web UI connects to Warden server-side. The following settings apply to **outbound Warden API requests**, not the browser listener or Warden's server limits:

```sh
warden-webui \
  --warden-request-timeout=20s \
  --warden-maintenance-timeout=60s \
  --warden-max-response-bytes=4194304
```

The defaults come directly from Warden’s `pkg/client` (`10s`, `35s`, and `1048576` bytes / 1 MiB). All three values must be positive. The maximum response size is validated by `client.ValidateMaxResponseBytes`, using the same rule as the Warden client. The longer maintenance deadline accommodates Warden's 30-second server-side maintenance limit; changing the Web UI timeout does not extend that server deadline. The same values are available through `WARDEN_WEBUI__WARDEN_REQUEST_TIMEOUT`, `WARDEN_WEBUI__WARDEN_MAINTENANCE_TIMEOUT`, and `WARDEN_WEBUI__WARDEN_MAX_RESPONSE_BYTES`.

Warden's `--route-prefix` may be part of the upstream URL. The Web UI's application prefix is separate:

```sh
WARDEN_WEBUI__ROUTE_PREFIX=/ui
WARDEN_WEBUI__PUBLIC_URL=https://example.com/ui
```

Use the **actual external URL**; Web UI checks form Origin or Referer against it to reject cross-origin writes. The application prefix does not change `/healthz` and `/readyz` on the independent management listener.

## First-run setup and storage

A random setup token is required until the first local administrator has been created:

```sh
export WARDEN_WEBUI__SETUP_TOKEN="$(openssl rand -hex 32)"
```

Open `/setup`, enter the token, create a username and a password of 12–72 bytes. Web UI reads setup state once on startup, and registers setup routes only while the first administrator is missing. After the successful database commit, both setup handlers immediately return 404 without database polling; following restarts never register them again. The database still prevents concurrent bootstrap submissions from creating more than one administrator. You can remove the setup token from the deployment after initialization. If bootstrapping directly into an external authentication mode, see [Authentication](authentication.md#first-time-setup-with-external-authentication).

## Logging and appearance

The shared Appkit flags include logging settings such as `--log-format`, `--debug`, and `--access-log`. Use `WARDEN_WEBUI__THEME_DIRECTORY` or `--theme-directory` to load custom theme TOML files at startup; see [Themes](themes.md).

For deployment-specific manifests and secrets, see [Deployment](deployment.md).
