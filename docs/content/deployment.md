# Deployment

Deploy the Web UI as **one replica with its own SQLite database**. Keep its application and management listeners separate, expose the application over HTTPS, and provide credentials using Kubernetes Secrets.

## Kubernetes with Kustomize

The repository includes [Kustomize manifests](https://github.com/gi8lino/keycloak-warden-webui/tree/main/deploy/kubernetes) with two **alternative** variants:

| Variant    | Scope                                                                    | Warden connection                                                     |
| ---------- | ------------------------------------------------------------------------ | --------------------------------------------------------------------- |
| Sidecar    | One `keycloak-warden` Deployment containing Warden and Web UI containers | `http://127.0.0.1:8080`                                               |
| Standalone | An independent `keycloak-warden-webui` Deployment                        | HTTPS by default, or HTTP to a ClusterIP Service with explicit opt-in |

Use **only one** variant in a namespace. Both generate a nonsecret Web UI ConfigMap, create a Web UI PVC, and expose a `ClusterIP` Service on application port `8082`. The management port (`8083`) is internal to the Pod.

### Sidecar

```sh
kubectl kustomize deploy/kubernetes/sidecar
kubectl apply -k deploy/kubernetes/sidecar
```

The sidecar build also generates the Warden ConfigMap from `sidecar/warden.yaml` and manages the Warden Deployment, Service, and its own **separate** PVC. Before applying, review this configuration against any existing Warden deployment: the sidecar option will **update the existing `keycloak-warden` Deployment**.

Supply Secrets independently for `keycloak-snitch`, `keycloak-warden`, and `keycloak-warden-webui` as described in [deploy/kubernetes/README.md](https://github.com/gi8lino/keycloak-warden-webui/blob/main/deploy/kubernetes/README.md).

### Standalone

```sh
kubectl kustomize deploy/kubernetes/standalone
kubectl apply -k deploy/kubernetes/standalone
```

The standalone manifest connects to `http://keycloak-warden.keycloak.svc.cluster.local:8080` and sets `WARDEN_WEBUI__ALLOW_INSECURE_WARDEN_HTTP=true` explicitly. Change `WARDEN_WEBUI__WARDEN_URL` if your Service name, namespace, port, or route prefix differs. Alternatively, use an internal HTTPS URL and omit the opt-in. The administrative token must match Warden's token and be supplied through a WebUI Secret. Restrict plain HTTP connectivity with Cilium because the bearer token is unencrypted.

### Common settings

Before deployment, adjust `deploy/kubernetes/base/webui.env` to match the browser-visible HTTPS URL and desired authentication mode. Keep the setup token, Warden admin token, and OIDC secrets **out** of ConfigMaps. Pin tested container images rather than using `:latest`, and use persistent storage for each database.

Expose the Web UI Service through a protected TLS ingress or Gateway. Do not publicly expose the management listener or Warden's administrative API. See [Authentication](authentication.md) for OIDC and proxy requirements.

## Docker Compose (local only)

The repository includes a Compose environment for integration testing; it is **not** a production credential template:

```sh
make e2e-up
make e2e-down
```

For initial local setup, see [Getting started](getting-started.md). For health checks and database maintenance, see [Operations](operations.md).
