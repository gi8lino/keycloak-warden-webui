# Authentication

Warden Web UI does **not** support anonymous administrative access. It uses one primary authentication mode per deployment: **local**, **OIDC**, or **trusted proxy**. Each mode still requires the server-side Warden administrator API token.

## Local administrators

Local authentication is the default. On first startup, open `/setup` and create the initial administrator using the configured 32-character-or-longer setup token. Local usernames and password hashes, plus hashed browser sessions, are stored in Web UI's private SQLite database. Passwords must be 12–72 bytes.

Subsequent sessions use `/login`. Administrators can change their password in the account preferences; changing a password invalidates that account's existing local sessions. Additional local administrators can be created or disabled in **User administration**. The initial administrator cannot be disabled.

## OIDC with Keycloak

Configure the following (use Secrets for credentials):

```sh
WARDEN_WEBUI__AUTH_MODE=oidc
WARDEN_WEBUI__PUBLIC_URL=https://warden-ui.example.com
WARDEN_WEBUI__OIDC_ISSUER=https://keycloak.example.com/realms/services
WARDEN_WEBUI__OIDC_CLIENT_ID=warden-webui
WARDEN_WEBUI__OIDC_CLIENT_SECRET=... # Secret
WARDEN_WEBUI__OIDC_SESSION_SECRET=... # Secret, at least 32 bytes
WARDEN_WEBUI__OIDC_GROUP_CLAIM=groups
WARDEN_WEBUI__ADMIN_GROUP=warden-admins
```

In Keycloak, register **`https://warden-ui.example.com/auth/callback`** as the allowed redirect URI. Include the Web UI route prefix if configured. The OIDC client must use the authorization-code flow (PKCE) and expose the exact administrative group in the configured ID-token claim. Web UI verifies the OIDC identity and required group before using the local account mapping.

Users are **not automatically granted administrator access** just because they belong to the group. Their verified identity must also be explicitly linked to a local account (see [Administration](administration.md)).

## Trusted proxy

Use this mode when an ingress or authentication proxy verifies users and supplies identity headers:

```sh
WARDEN_WEBUI__AUTH_MODE=proxy
WARDEN_WEBUI__TRUSTED_PROXY_CIDRS=10.0.5.12/32
WARDEN_WEBUI__ADMIN_GROUP=warden-admins
```

The default identity headers include `X-Forwarded-User` and `X-Forwarded-Groups`. The proxy **must replace or remove headers supplied by clients**, and traffic must reach Web UI only through explicitly trusted proxy IP ranges. Do not trust the general internet or unrestricted CIDRs. The exact required group and an approved identity mapping are both necessary for access.

## First-time setup with external authentication

On a fresh installation, Web UI reads the initial administrator state **once at startup**. Only when initialization is needed does it register `GET /setup` and `POST /setup`. Visiting the Web UI root (`/`) redirects to `/setup` before proxy or OIDC authentication is evaluated, including with a route prefix (for example, `/ui/setup`). Setup still requires the private setup token; the redirect does not grant administrator access. The external ingress or identity proxy must also allow the browser to reach `/setup`.

After the first administrator is committed to SQLite, an atomic in-memory flag immediately disables both setup handlers: subsequent `GET` and `POST` requests return HTTP 404, and normal authentication applies. On subsequent restarts Web UI reads the persisted state again and never registers the setup routes for an initialized database. Normal HTTP requests do not query the database to determine whether setup is required; the one-time database insertion remains atomic against concurrent setup attempts.

Every deployment starts with a local administrator record, even if external sign-in is the primary mode. For initial OIDC or proxy setup, set `WARDEN_WEBUI__SETUP_SUBJECT` to the founder's exact OIDC `sub` or trusted proxy username. On first `/setup`, Web UI creates the administrator and links that subject. Sign in through your external provider afterward.

Alternatively, temporarily enable `WARDEN_WEBUI__LOCAL_LOGIN=true`, create the initial account, sign in at `/auth/local`, and approve the verified external identity. Disable local recovery once the external sign-in works.

## Recovery and identity approvals

When running OIDC or proxy mode, `WARDEN_WEBUI__LOCAL_LOGIN=true` enables an explicit password-based recovery route at `/auth/local`. Recovery is off by default. Secure this route behind appropriate network and ingress policy; it is not a substitute for securing the external provider.

An externally authenticated subject without an approved mapping is recorded as **pending** and denied access. An administrator can review verified pending identities in **User administration**, assign an account, and have the user sign in again. Removing an identity mapping or disabling an account revokes that identity's access.

Read [Configuration](configuration.md) for public URL, route prefix, and session storage settings.
