# Administration

After sign-in, Warden Web UI presents the current state of Warden through its administrative API. It does not edit Warden's rule or destination YAML configuration.

## Active bans and history

The **Active bans** dashboard lists current decisions. Open a decision to inspect its IP address, state, revision, independent ban sources, and acknowledgement status per configured destination. A pending acknowledgement indicates that Warden has not yet confirmed delivery of the current revision; it is not proof of enforcement by the destination.

## Create and release bans

Create a manual ban by entering an IP address, a positive duration (for example `15m` or `1h`), and a nonempty reason. The request is sent to Warden's administrator API; Web UI does not write enforcement records directly.

Use **Release** to release all active sources of an existing decision. This can also release a ban that was created automatically. Warden persists the new desired state and handles destination delivery.

## User administration

Open **User administration** to create additional local administrator accounts and enable or disable them. The first (founder) account cannot be disabled; administrators cannot disable their own account through the UI. Each account has administrator-level access rather than a separate per-role permission set.

For OIDC or trusted proxy mode, a user who authenticates successfully at the identity provider but has no approved local mapping is recorded under **Pending sign-ins**. You can approve that verified subject for a chosen account. After approval, ask the user to sign in again. Linked identities are scoped to the active provider; unlinking a subject removes its access and invalidates sessions associated with the mapping.

For local accounts, the user menu provides **Preferences** and **Sign out**. Local password changes are offered when local password authentication is available. For recovery scenarios under external authentication, see [Authentication](authentication.md).

## Personalization

Each account can save a theme under **Preferences → Appearance**. See [Themes](themes.md) for built-in palettes and custom theme overrides.
