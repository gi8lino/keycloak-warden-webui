# Kubernetes deployment options

Warden WebUI has **two alternative Kustomize builds**. Choose **one** for a cluster/namespace; do not apply both simultaneously. The common `base/` builds the `keycloak-warden-webui-config` ConfigMap, a private SQLite PVC, and a Service for browser traffic. There is no publicly exposed management Service.

## Build and apply

```sh
# Warden + WebUI containers in the same Pod, separate SQLite PVCs:
kubectl kustomize deploy/kubernetes/sidecar
kubectl apply -k deploy/kubernetes/sidecar

# OR a separate WebUI Deployment, with an existing Warden upstream:
kubectl kustomize deploy/kubernetes/standalone
kubectl apply -k deploy/kubernetes/standalone
```

Both are configured for the `keycloak` namespace. Change `namespace` in both the chosen variant and `base` together if using another namespace. Adjust `base/webui.env` before applying: set `WARDEN_WEBUI__PUBLIC_URL` to the externally accessible browser URL, including an optional route prefix (which also requires `WARDEN_WEBUI__ROUTE_PREFIX`).

The generated ConfigMap names have content hashes; the Deployment references are rewritten by Kustomize, triggering new Pods when the configuration changes. Only nonsecret settings belong in the ConfigMap; do not use `secretGenerator` with checked-in credentials.

## Sidecar (`sidecar/`)

`sidecar/deployment.yaml` is a **complete `keycloak-warden` Deployment** with two containers. It pins Warden's v0.25.0 image and includes an example Warden configuration. It also includes the Warden Service and existing Warden PVC name, plus a Kustomize-generated `keycloak-warden-config` from `sidecar/warden.yaml`.
Review and merge any custom Warden configuration first: applying this variant **updates the existing Warden Deployment and its configuration**. Never create a second Warden Deployment pointing to the same SQLite PVC.

The WebUI connects to Warden at `http://127.0.0.1:8080` because both containers share a Pod network namespace. Warden uses `/data/warden.db` on `keycloak-warden-data`, while WebUI uses `/data/warden-webui.db` on its **own** `keycloak-warden-webui-data` claim. Both workloads are single replica with `Recreate` strategy to avoid SQLite multi-writer races.

Create/manage these Secrets separately (for example using your existing ExternalSecrets or SealedSecrets workflow):

- `keycloak-snitch`: key `KC_SPI_EVENTS_LISTENER__SNITCH__WEBHOOK_SECRET`.
- `keycloak-warden`: keys `WARDEN__ADMIN_TOKEN`, `CLOUDFLARE_ZONE_ID`,
  `CLOUDFLARE_API_TOKEN` for the included Cloudflare target example; adjust
  the keys to the targets in your real `sidecar/warden.yaml`.
- `keycloak-warden-webui`: key `setup-token`, containing a random one-time
  setup token with at least 32 characters.

The sidecar reads `WARDEN__ADMIN_TOKEN` directly from Warden's Secret, so the administrative API token cannot accidentally drift between containers.

## Standalone (`standalone/`)

`standalone/deployment.yaml` manages only WebUI. The example uses Warden's internal Kubernetes Service URL (`http://keycloak-warden.keycloak.svc.cluster.local:8080`) and explicitly enables `WARDEN_WEBUI__ALLOW_INSECURE_WARDEN_HTTP=true`. Without this opt-in, remote Warden URLs must use HTTPS (loopback HTTP remains allowed). Set `WARDEN_WEBUI__WARDEN_URL` to your actual Warden application API URL, including its route prefix if configured. The WebUI needs the same Warden administrative API token configured on Warden.

Plain HTTP transmits the admin token unencrypted within the cluster. Use Cilium to restrict egress from WebUI to Warden's application port and ingress to that API from WebUI, or prefer an internal HTTPS endpoint. The opt-in does **not** disable TLS certificate validation.

Create/manage `keycloak-warden-webui` separately with keys:

- `warden-admin-token`: the same value as Warden's `WARDEN__ADMIN_TOKEN`.
- `setup-token`: a random one-time setup token with at least 32 characters.

The standalone WebUI has its own Deployment, PVC, and ClusterIP Service.
No Warden service, volume, configuration, or Secret is recreated in this variant.

## Browser, authentication and exposure

The generated Service `keycloak-warden-webui` exposes port `8082` only.
Configure your own authenticated ingress or Gateway and TLS for the `WARDEN_WEBUI__PUBLIC_URL` host. Keep management port `8083` and Warden's administrative listener private. WebUI is configured for local login as an initial working example; open `/setup` and use the secret's setup token on first installation. Once initialized, the setup token is not required.

For OIDC, set `WARDEN_WEBUI__AUTH_MODE=oidc`,
`WARDEN_WEBUI__OIDC_ISSUER`, `WARDEN_WEBUI__OIDC_CLIENT_ID`,
`WARDEN_WEBUI__OIDC_GROUP_CLAIM`, `WARDEN_WEBUI__ADMIN_GROUP` in
`base/webui.env`; supply `WARDEN_WEBUI__OIDC_CLIENT_SECRET` and
`WARDEN_WEBUI__OIDC_SESSION_SECRET` as Secret-backed environment variables,
then set `WARDEN_WEBUI__SETUP_SUBJECT` for the initial administrator or enable
`WARDEN_WEBUI__LOCAL_LOGIN` for recovery. Register the exact HTTPS callback
`<public-url>/auth/callback` with Keycloak. Never put credentials in `.env`.

The deployment images must be changed from `latest` to a tested release tag or immutable digest before production use. Set PVC sizes and StorageClass for the cluster. Configure Warden's own webhook/Cloudflare target details and associated Secrets before enabling the sidecar alternative.

## Previous example fragments

The old files under `deploy/kubernetes-*.example.yaml` were snippets, not
usable Kustomize builds. They are superseded by these complete variants:

- `kubernetes-sidecar.example.yaml` → `sidecar/deployment.yaml`.
- `kubernetes-webui-pvc.example.yaml` → `base/persistentvolumeclaim.yaml`.
- `kubernetes-webui-volume.example.yaml` → the Pod volumes in each Deployment.

The Compose-only files under `deploy/` stay in place and are not moved.
