# Administration

After sign-in, Warden Web UI presents the current state of Warden through its administrative API. It does not edit Warden's rule or destination YAML configuration.

## Active bans and history

The **Active bans** dashboard lists current decisions. Open a decision to inspect its IP address, state, revision, independent ban sources, and acknowledgement status per configured destination. A pending acknowledgement indicates that Warden has not yet confirmed delivery of the current revision; it is not proof of enforcement by the destination.

The **Delivery history** section groups completed attempts by revision and destination, with the newest attempts first. Expand **Failure details** to inspect a provider error. Pending, retrying, delivered, exhausted, and superseded states are shown alongside acknowledgement and next-retry times when available.

Use **Older history** and **Newer history** to browse pages. Navigation retains the starting revision; **Refresh latest** includes newer revisions and returns to the history section. Current acknowledgement summaries remain separate from historical attempts and include destinations with current progress even if none has ever acknowledged a revision; these show a dash for the acknowledged revision. If the latest bounded history page does not contain a destination's current status, it is shown as **not acknowledged**, rather than guessing whether it is retrying. Warden may omit older attempts within a group; the UI labels these omissions. Attempt durations are not available from the API.

## Global ban history and delivery overview

The **Ban history** navigation entry shows effective decision revisions recorded in
Warden's append-only audit journal, including releases and extensions. Filter by exact IP
address or desired state and page through the results. Each event includes its UTC commit
time, cause, and the source reasons recorded at that revision. Older decisions created
before Warden migration 0008 cannot be reconstructed.

The separate **Deliveries** navigation entry displays the current revision of every
decision/destination pair, not just active bans. Filter by delivery status or destination
to find `retrying` and `exhausted` deliveries, including unban notifications that were not
accepted. The **Inspect** link opens the existing decision detail and full attempt timeline.

Both lists are queried through Warden's authenticated API; WebUI does not retain a second
copy of Warden's audit data. Pagination is bounded and newer revisions can shift the
results until the operator refreshes.

## Create and release bans

Create a manual ban by entering an IP address, a positive duration (for example `15m` or `1h`), and a nonempty reason. The request is sent to Warden's administrator API; Web UI does not write enforcement records directly.

Use **Release** to release all active sources of an existing decision. This can also release a ban that was created automatically. Warden persists the new desired state and handles destination delivery.

## User administration

Open **User administration** to create additional local administrator accounts and enable or disable them. The first (founder) account cannot be disabled; administrators cannot disable their own account through the UI. Each account has administrator-level access rather than a separate per-role permission set.

For OIDC or trusted proxy mode, a user who authenticates successfully at the identity provider but has no approved local mapping is recorded under **Pending sign-ins**. You can approve that verified subject for a chosen account. After approval, ask the user to sign in again. Linked identities are scoped to the active provider; unlinking a subject removes its access and invalidates sessions associated with the mapping.

For local accounts, the user menu provides **Preferences** and **Sign out**. Local password changes are offered when local password authentication is available. For recovery scenarios under external authentication, see [Authentication](authentication.md).

## Personalization

Each account can save a theme under **Preferences → Appearance**. See [Themes](themes.md) for built-in palettes and custom theme overrides.
