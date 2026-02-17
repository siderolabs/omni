# Omni Helm Chart (v2)

![Version: 2.1.1](https://img.shields.io/badge/Version-2.1.1-informational?style=flat) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat) ![AppVersion: v1.5.3](https://img.shields.io/badge/AppVersion-v1.5.3-informational?style=flat)

A Helm chart to deploy [Omni](https://omni.siderolabs.com) on a Kubernetes cluster.

> [!WARNING]
> **This chart is for new installations only.** Upgrading or migrating from the deprecated v1 chart is not supported.
> All new Omni installations should use this chart. Only Omni **1.5.0** and above is supported.

For all available configuration options, see the [`values.yaml`](values.yaml) file and the [Values](#values) section below.

**Homepage:** <https://www.siderolabs.com/omni/>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Sidero Labs | <info@siderolabs.com> | <https://www.siderolabs.com> |

## Installation

This chart is not yet published to a Helm repository. To install it, clone the repository and install from the local path.

### 1. Generate an etcd Encryption Key

Omni requires a GPG private key to encrypt its database. If you don't have one, generate one with:

```bash
export GNUPGHOME=$(mktemp -d)
gpg --batch --passphrase '' --quick-gen-key "Omni" default default never
gpg --batch --passphrase '' --armor --export-secret-keys "Omni" > omni.asc
rm -rf "$GNUPGHOME"
```

### 2. Create a values.yaml file

Create a `values.yaml` file with your configuration. See the example below.

### 3. Install the Chart

```bash
helm install omni ./deploy/helm/omni -n omni --create-namespace -f values.yaml
```

To upgrade an existing installation:

```bash
helm upgrade omni ./deploy/helm/omni -n omni -f values.yaml
```

## Authentication

One of the following authentication providers must be enabled and configured under `config.auth`:

- `auth0` - Auth0 authentication
- `saml` - SAML authentication
- `oidc` - OpenID Connect authentication

Additionally, at least one initial admin user must be specified in `config.auth.initialUsers`.

## Example

For Omni configuration reference, see the [Omni config schema](https://github.com/siderolabs/omni/blob/main/internal/pkg/config/schema.json).

This example uses the [Traefik](https://traefik.io/traefik/) ingress controller and [cert-manager](https://cert-manager.io/) for TLS certificates (with a wildcard certificate for `*.example.com`).

The example assumes three DNS entries pointing to your ingress controller:

| Hostname                       | Purpose                          |
| ------------------------------ | -------------------------------- |
| `omni.example.com`             | UI, gRPC API, and Talos API proxy |
| `omni-k8s.example.com`         | Kubernetes API proxy             |
| `omni-siderolink.example.com`  | SideroLink Machine API           |

> [!NOTE]
> The main service serves both the web UI and all gRPC APIs (Omni API and Talos API proxy) from the same port using h2c (HTTP/2 cleartext).
> The backend demuxes traffic by `Content-Type` header, so no separate gRPC ingress is needed.

```yaml
# Paste the contents of your omni.asc file here
etcdEncryptionKey:
  omniAsc: |
    -----BEGIN PGP PRIVATE KEY BLOCK-----
    <your-gpg-private-key-here>
    -----END PGP PRIVATE KEY BLOCK-----

ingress:
  main:
    enabled: true
    className: traefik
    host: omni.example.com
    tls:
      - hosts:
          - omni.example.com
  kubernetesProxy:
    enabled: true
    className: traefik
    host: omni-k8s.example.com
    tls:
      - hosts:
          - omni-k8s.example.com
  siderolinkApi:
    enabled: true
    className: traefik
    host: omni-siderolink.example.com
    tls:
      - hosts:
          - omni-siderolink.example.com

config:
  account:
    # Generate a unique UUID for your installation (e.g., using `uuidgen`)
    id: 123e4567-e89b-12d3-a456-426614174000
  auth:
    auth0:
      enabled: true
      clientID: <your-auth0-client-id>
      domain: <your-auth0-domain>
    initialUsers:
      - admin@example.com
  services:
    api:
      advertisedURL: https://omni.example.com
    kubernetesProxy:
      advertisedURL: https://omni-k8s.example.com
    machineAPI:
      advertisedURL: https://omni-siderolink.example.com
    siderolink:
      wireGuard:
        # The externally accessible IP:port for WireGuard connections.
        # If using an externally accessible node IP, the port should match
        # service.wireguard.nodePort (default: 30180).
        advertisedEndpoint: <node-ip>:30180
```

## Workload Proxy (Optional)

Workload Proxy allows you to expose HTTP services running in your managed clusters through Omni. Once configured, you can annotate Kubernetes Services to make them accessible, protected by Omni's authentication.

For details on exposing services, see [Expose an HTTP Service from a Cluster](https://omni.siderolabs.com/docs/how-to-guides/self-hosted/how-to-expose-http-service-from-a-cluster/).

### Domain Structure

The workload proxy domain is **not** a subdomain of Omni—it exists alongside it. For example:

- Omni: `omni.example.com`
- Workload Proxy: `*.omni-workload.example.com`

Exposed services have URLs in the format:

```
https://<prefix>-<account-name>.<subdomain>.<parent-domain>/
```

For example: `https://grafana-app.omni-workload.example.com/`

Where:
- `<prefix>` is randomly generated, or user-specified via a Service annotation
- `<account-name>` is the value of `config.account.name`
- `<subdomain>` is the value of `config.services.workloadProxy.subdomain` (default: `omni-workload`)
- `<parent-domain>` is the parent domain of your Omni installation

### Requirements

To set up workload proxy, you need:

1. **Enable workload proxy** in values:
   ```yaml
   config:
     services:
       workloadProxy:
         enabled: true
         subdomain: omni-workload  # default
   ```

2. **Wildcard TLS certificate** for `*.<subdomain>.<parent-domain>` (e.g., `*.omni-workload.example.com`). You can use cert-manager to create one.

3. **Wildcard DNS record** pointing `*.<subdomain>.<parent-domain>` to your ingress controller IP address.

4. **Ingress/routing rule** to route traffic matching `*-<account-name>.<subdomain>.<parent-domain>` to the Omni service. Standard Kubernetes Ingress does not support wildcard hostnames well, so you may need to use ingress controller-specific resources.

### Traefik Example

With Traefik, you can use an `IngressRoute` custom resource via `extraObjects`:

```yaml
config:
  account:
    name: app  # This will be part of the workload proxy URL
  services:
    workloadProxy:
      enabled: true
      subdomain: omni-workload

extraObjects:
  # Wildcard certificate for workload proxy (using cert-manager)
  - apiVersion: cert-manager.io/v1
    kind: Certificate
    metadata:
      name: omni-workload-proxy-wildcard
    spec:
      secretName: omni-workload-proxy-wildcard-tls
      issuerRef:
        name: your-cluster-issuer
        kind: ClusterIssuer
      dnsNames:
        - "*.omni-workload.example.com"

  # Traefik IngressRoute for wildcard routing
  - apiVersion: traefik.io/v1alpha1
    kind: IngressRoute
    metadata:
      name: omni-workload-proxy
    spec:
      entryPoints:
        - websecure
      routes:
        - kind: Rule
          # Match any subdomain of omni-workload.example.com
          match: HostRegexp(`.+\.omni-workload\.example\.com`)
          services:
            - kind: Service
              name: omni
              port: omni
              passHostHeader: true
              scheme: h2c
      tls:
        secretName: omni-workload-proxy-wildcard-tls
```

## Values

Here are the configurable parameters of the Omni chart and their default values.

> [!NOTE]
> Some configuration options are not listed here, as they are commented out in `values.yaml`.
>
> For the full list, refer to the [`values.yaml`](values.yaml) file and the [Omni config schema](https://github.com/siderolabs/omni/blob/main/internal/pkg/config/schema.json).

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| additionalConfigSources | list | `[]` | Additional config sources to merge on top of the base config. These are merged in order: base config (from config: section) -> additionalConfigSources (in order) -> flags. This is useful for bringing in secrets from external sources (e.g., External Secrets Operator). Each source must contain a valid Omni config YAML that will be merged with the base config. |
| affinity | object | `{}` | Pod affinity settings for the Omni pod |
| args | list | `[]` | Extra arguments to pass to Omni. Note: --config-path is automatically generated by the chart based on the base config and additionalConfigSources. |
| config.account.name | string | `"app"` | Name is the human-readable name of the account. |
| config.auth.auth0.clientID | string | `""` | ClientID is the Auth0 client ID. |
| config.auth.auth0.domain | string | `""` | Domain is the Auth0 domain. |
| config.auth.auth0.enabled | bool | `true` | Enabled controls whether the Auth0 authentication provider is enabled. |
| config.auth.initialServiceAccount | string | `nil` | InitialServiceAccount contains configuration for the initial service account created when Omni is run for the first time. |
| config.auth.initialUsers | list | `[]` | InitialUsers is a list of emails which should be created as admins when Omni is run for the first time. |
| config.auth.keyPruner | string | `nil` | KeyPruner contains configuration for the public keys pruner. |
| config.auth.oidc | string | `nil` | OIDC contains OIDC authentication provider configuration. |
| config.auth.saml | string | `nil` | SAML contains SAML authentication provider configuration. |
| config.debug.pprof | string | `nil` | Pprof contains pprof profiling configuration. |
| config.debug.server | string | `nil` | Server contains debug server configuration. |
| config.etcdBackup.localPath | string | `"/data/etcd-backup"` | LocalPath is the local path where etcd backups are stored. Path-based backups are enabled when this is set. This path is mounted from the persistent volume by default. |
| config.etcdBackup.s3Enabled | bool | `false` | S3Enabled controls whether an S3-compatible storage is used for etcd backups. Mutually exclusive with localPath. |
| config.features | string | `nil` | Features contains feature flags to enable/disable various Omni features. |
| config.logs.audit | string | `nil` | Audit contains audit logs configuration. |
| config.logs.machine.storage | string | `nil` | Storage contains configuration for machine logs storage. |
| config.logs.resourceLogger | string | `nil` | ResourceLogger contains resource logger configuration. |
| config.registries | string | `nil` | Registries contains container image registries configuration. |
| config.services.api.advertisedURL | string | `"https://omni.example.com"` | The advertised URL for the main Omni GRPC and HTTP API and Web UI. MUST be specified as a full URL, including scheme (http:// or https://). It MUST match the URL of the main ingress if ingress is used. |
| config.services.embeddedDiscoveryService | string | `nil` | EmbeddedDiscoveryService contains embedded discovery service configuration. |
| config.services.kubernetesProxy.advertisedURL | string | `"https://kubernetes.omni.example.com"` | The advertised URL for the Kubernetes API Proxy. MUST be specified as a full URL, including scheme (https://). It MUST match the URL of the kubernetes proxy ingress if ingress is used. |
| config.services.loadBalancer | string | `nil` | LoadBalancer contains load balancer service configuration. |
| config.services.localResourceService | string | `nil` | LocalResourceService contains local resource service configuration. |
| config.services.machineAPI.advertisedURL | string | `"https://siderolink.omni.example.com"` | The advertised URL for the SideroLink (Machine) API. MUST be specified as a full URL, including scheme (http:// or https://). It MUST match the URL of the siderolink API ingress if ingress is used. |
| config.services.siderolink.eventSinkPort | int | `8091` | EventSinkPort is the port to be used by the nodes to publish their events over SideroLink to Omni. |
| config.services.siderolink.joinTokensMode | string | `"strict"` | JoinTokensMode configures how machine join tokens are generated and used. |
| config.services.siderolink.wireGuard | string | `nil` |  |
| config.services.workloadProxy.enabled | bool | `false` | Enabled controls whether the workload proxy service is enabled. In on-prem setups, it is often disabled. |
| config.services.workloadProxy.subdomain | string | `"omni-workload"` | Subdomain is the subdomain used by the workload proxy service to expose workloads. |
| config.storage.default.boltdb.path | string | `"/data/omni-boltdb.db"` | Path to the primary BoltDB database. Is **NOT USED by default**: only used if the storage.default.kind is set to "boltdb". This path is mounted from the persistent volume by default. |
| config.storage.default.etcd.embedded | bool | `true` | Embedded controls whether to use embedded etcd server as the storage backend. |
| config.storage.default.etcd.embeddedDBPath | string | `"/data/etcd/"` | EmbeddedDBPath is the path where the embedded etcd database files are stored. This path is mounted from the persistent volume by default. |
| config.storage.default.etcd.privateKeySource | string | `"file:///omni.asc"` | PrivateKeySource is the source of the private key for decrypting master key slot. |
| config.storage.default.kind | string | `"etcd"` | Kind is the kind of the default storage backend (etcd or boltdb). |
| config.storage.sqlite.path | string | `"/data/secondary-storage/sqlite.db"` | Path to the SQLite database (secondary storage). This path is mounted from the persistent volume by default. |
| config.storage.vault.token | string | `""` | Token is the authentication token for the Vault server. Tip: Use additionalConfigSources to load this from an existing Secret, or set the VAULT_TOKEN environment variable via env/envFrom. |
| config.storage.vault.url | string | `""` | Url is the URL of the Vault server. |
| dnsConfig | object | `{}` | DNS configuration for the Omni pod. |
| dnsPolicy | string | `""` | DNS policy for the Omni pod. When not set and hostNetwork is enabled, defaults to ClusterFirstWithHostNet. |
| env | list | `[]` | Environment variables to pass to Omni. |
| envFrom | list | `[]` | envFrom to pass to Omni. |
| etcdEncryptionKey.existingSecret | string | `""` | Name of an existing Secret containing the GPG private key. |
| etcdEncryptionKey.omniAsc | string | `""` | GPG private key content (multiline string). |
| extraContainers | list | `[]` | Extra containers (sidecars) to add to the Omni pod. |
| extraObjects | list | `[]` | Extra Kubernetes objects to deploy with the helm chart |
| extraVolumeMounts | list | `[]` | List of additional mounts to add (normally used with extraVolumes) |
| extraVolumes | list | `[]` | List of extra volumes to add |
| fullnameOverride | string | `""` | String to fully override `"omni.fullname"`. |
| gatewayApi.kubernetesProxy.annotations | object | `{}` | Additional Annotations |
| gatewayApi.kubernetesProxy.enabled | bool | `false` | Enable HTTPRoute for Kubernetes Proxy. |
| gatewayApi.kubernetesProxy.hostnames | list | `["kubernetes.omni.example.com"]` | Omni Kubernetes Proxy hostname |
| gatewayApi.kubernetesProxy.labels | object | `{}` | Additional Labels |
| gatewayApi.kubernetesProxy.parentRefs | list | `[]` | The Gateway(s) to attach this route to. You MUST define at least one parentRef for the route to be active. |
| gatewayApi.siderolinkApi.annotations | object | `{}` | Additional Annotations |
| gatewayApi.siderolinkApi.enabled | bool | `false` | Enable GRPCRoute for SideroLink Machine API. |
| gatewayApi.siderolinkApi.hostnames | list | `["siderolink.omni.example.com"]` | Omni SideroLink Machine API hostname |
| gatewayApi.siderolinkApi.labels | object | `{}` | Additional Labels |
| gatewayApi.siderolinkApi.parentRefs | list | `[]` | The Gateway(s) to attach this route to. You MUST define at least one parentRef for the route to be active. |
| gatewayApi.ui.annotations | object | `{}` | Additional Annotations |
| gatewayApi.ui.enabled | bool | `false` | Enable HTTPRoute for Web UI. |
| gatewayApi.ui.hostnames | list | `["omni.example.com"]` | Omni UI hostname |
| gatewayApi.ui.labels | object | `{}` | Additional Labels |
| gatewayApi.ui.parentRefs | list | `[]` | The Gateway(s) to attach this route to. You MUST define at least one parentRef for the route to be active. |
| hostNetwork | bool | `false` | Use host networking for the Omni pod. This may be required in some environments (e.g., GKE with Container-Optimized OS) where the NET_ADMIN capability alone is not sufficient for creating the WireGuard interface. When enabled, dnsPolicy is automatically set to ClusterFirstWithHostNet (unless dnsPolicy is explicitly set). |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy for Omni. |
| image.repository | string | `"ghcr.io/siderolabs/omni"` | Repository to use for Omni. |
| image.tag | string | `""` | Tag to use for Omni. |
| imagePullSecrets | list | `[]` | Secrets with credentials to pull images from a private registry. |
| ingress.kubernetesProxy.annotations | object | `{}` | Additional Annotations |
| ingress.kubernetesProxy.className | string | `""` | Ingress Class Name |
| ingress.kubernetesProxy.enabled | bool | `false` | Enable ingress for Kubernetes Proxy. |
| ingress.kubernetesProxy.host | string | `"kubernetes.omni.example.com"` | Omni Kubernetes Proxy hostname |
| ingress.kubernetesProxy.labels | object | `{}` | Additional Labels |
| ingress.kubernetesProxy.skipConfigCheck | bool | `false` | Set to true to skip validation between this Ingress and config.services.kubernetesProxy.advertisedURL |
| ingress.kubernetesProxy.tls | list | `[]` | TLS configuration |
| ingress.main.annotations | object | `{"nginx.ingress.kubernetes.io/backend-protocol":"GRPC","nginx.ingress.kubernetes.io/proxy-body-size":"32m","nginx.ingress.kubernetes.io/service-upstream":"true"}` | Additional Annotations. The default annotations ensure compatibility with NGINX Ingress Controller. They are ignored by other ingress controllers. |
| ingress.main.className | string | `""` | Ingress Class Name |
| ingress.main.enabled | bool | `false` | Enable ingress for Web UI and gRPC API. |
| ingress.main.host | string | `"omni.example.com"` | Omni hostname |
| ingress.main.labels | object | `{}` | Additional Labels |
| ingress.main.skipConfigCheck | bool | `false` | Set to true to skip validation between this Ingress and config.services.api.advertisedURL |
| ingress.main.tls | list | `[]` | TLS configuration |
| ingress.siderolinkApi.annotations | object | `{"nginx.ingress.kubernetes.io/backend-protocol":"GRPC","nginx.ingress.kubernetes.io/proxy-body-size":"100m"}` | Additional Annotations. The default annotations ensure compatibility with NGINX Ingress Controller. They are ignored by other ingress controllers. |
| ingress.siderolinkApi.className | string | `""` | Ingress Class Name |
| ingress.siderolinkApi.enabled | bool | `false` | Enable ingress for SideroLink Machine API. |
| ingress.siderolinkApi.host | string | `"siderolink.omni.example.com"` | Omni SideroLink Machine API hostname |
| ingress.siderolinkApi.labels | object | `{}` | Additional Labels |
| ingress.siderolinkApi.skipConfigCheck | bool | `false` | Set to true to skip validation between this Ingress and config.services.machineAPI.advertisedURL |
| ingress.siderolinkApi.tls | list | `[]` | TLS configuration |
| initContainers | list | `[]` | Init containers to add to the Omni pod. |
| livenessProbe | object | `{}` | Liveness probe configuration. |
| metrics.enabled | bool | `false` | Deploy metrics service |
| metrics.rules.annotations | object | `{}` | PrometheusRule annotations |
| metrics.rules.enabled | bool | `false` | Deploy a PrometheusRule for the application controller |
| metrics.rules.labels | object | `{}` | PrometheusRule labels |
| metrics.rules.namespace | string | `""` | PrometheusRule namespace |
| metrics.rules.selector | object | `{}` | PrometheusRule selector |
| metrics.rules.spec | list | `[]` | PrometheusRule.Spec for the application controller |
| metrics.service.annotations | object | `{}` | Metrics service annotations |
| metrics.service.clusterIP | string | `""` | Metrics service clusterIP. `None` makes a "headless service" (no virtual IP) |
| metrics.service.labels | object | `{}` | Metrics service labels |
| metrics.service.servicePort | int | `2122` | Metrics service port |
| metrics.service.type | string | `"ClusterIP"` | Metrics service type |
| metrics.serviceMonitor.annotations | object | `{}` | Prometheus ServiceMonitor annotations |
| metrics.serviceMonitor.enabled | bool | `false` | Enable a prometheus ServiceMonitor |
| metrics.serviceMonitor.honorLabels | bool | `false` | When true, honorLabels preserves the metric's labels when they collide with the target's labels. |
| metrics.serviceMonitor.interval | string | `"30s"` | Prometheus ServiceMonitor interval |
| metrics.serviceMonitor.labels | object | `{}` | Prometheus ServiceMonitor labels |
| metrics.serviceMonitor.metricRelabelings | list | `[]` | Prometheus [MetricRelabelConfigs] to apply to samples before ingestion |
| metrics.serviceMonitor.namespace | string | `""` | Prometheus ServiceMonitor namespace |
| metrics.serviceMonitor.relabelings | list | `[]` | Prometheus [RelabelConfigs] to apply to samples before scraping |
| metrics.serviceMonitor.scheme | string | `""` | Prometheus ServiceMonitor scheme |
| metrics.serviceMonitor.scrapeTimeout | string | `""` | Prometheus ServiceMonitor scrapeTimeout. If empty, Prometheus uses the global scrape timeout unless it is less than the target's scrape interval value in which the latter is used. |
| metrics.serviceMonitor.selector | object | `{}` | Prometheus ServiceMonitor selector |
| metrics.serviceMonitor.tlsConfig | object | `{}` | Prometheus ServiceMonitor tlsConfig |
| nameOverride | string | `""` | Provide a name in place of `"omni"`. |
| namespaceOverride | string | `""` | String to fully override `"omni.namespace"`. |
| nodeSelector | object | `{}` | Node selector to constrain which nodes the Omni pod is scheduled on |
| persistence.accessModes | list | `["ReadWriteOnce"]` | Access modes for the PVC. 'ReadWriteOnce' is recommended as Omni does not support multiple active replicas writing to the same file-based DB. |
| persistence.annotations | object | `{}` | Annotations |
| persistence.enabled | bool | `true` | Enable persistence. Mounts to `/data` inside the container. |
| persistence.existingClaim | string | `""` | Name of an existing PersistentVolumeClaim to use. If set, a new PVC will NOT be created. |
| persistence.size | string | `"16Gi"` | Size of the persistent volume. Recommended: 16Gi+ for production to handle etcd snapshots and logs. |
| persistence.storageClassName | string | `""` | Storage Class to use for the PVC. Leave empty to use the cluster's default storage class. |
| podAnnotations | object | `{}` | Additional annotations to add to the Omni pod |
| podLabels | object | `{}` | Additional labels to add to the Omni pod |
| podSecurityContext | object | `{}` | Pod-level security context |
| priorityClassName | string | `""` | Priority class name for the Omni pod. |
| readinessProbe | object | `{"failureThreshold":3,"httpGet":{"path":"/healthz","port":"omni","scheme":"HTTP"},"periodSeconds":5,"successThreshold":1,"timeoutSeconds":3}` | Readiness probe configuration. |
| replicaCount | int | `1` | Number of replicas to run. Omni currently only supports a single replica. If embedded etcd is used, this value should remain at 1. If an external etcd cluster is used, multiple replicas can be used, but they will run an etcd election and only one instance will be active at a time, others will be standby. |
| resources | object | `{}` | CPU and Memory resource requests and limits. |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"add":["NET_ADMIN"],"drop":["ALL"]}}` | Pod Security Context Omni container-level security context |
| service.k8sProxy.annotations | object | `{}` | Annotations for the Kubernetes proxy service. |
| service.k8sProxy.clusterIP | string | `""` | ClusterIP for the Kubernetes proxy service. |
| service.k8sProxy.labels | object | `{}` | Additional labels for the Kubernetes proxy service. |
| service.k8sProxy.port | int | `8095` | Port for the Kubernetes proxy. |
| service.k8sProxy.type | string | `"ClusterIP"` | Service type for the Kubernetes proxy service. |
| service.main.annotations | object | `{"traefik.ingress.kubernetes.io/service.serversscheme":"h2c"}` | Annotations for the main service. Traefik does not yet support appProtocol (https://github.com/traefik/traefik/issues/11089), so this annotation is needed to tell Traefik to use h2c when communicating with the backend. It is ignored by other ingress controllers. |
| service.main.clusterIP | string | `""` | ClusterIP for the main service. |
| service.main.labels | object | `{}` | Additional labels for the main service. |
| service.main.loadBalancerIP | string | `""` | LoadBalancer IP for the main service (only used when type is LoadBalancer). |
| service.main.omniNodePort | string | `""` | NodePort for the Omni HTTP/gRPC API (only used when type is NodePort). |
| service.main.omniPort | int | `8080` | Port for the Omni HTTP/gRPC API and UI. |
| service.main.siderolinkApiNodePort | string | `""` | NodePort for the SideroLink API (only used when type is NodePort). |
| service.main.siderolinkApiPort | int | `8090` | Port for the SideroLink API. |
| service.main.type | string | `"ClusterIP"` | Service type for the main service. |
| service.wireguard.annotations | object | `{}` | Annotations for the WireGuard service. |
| service.wireguard.clusterIP | string | `""` | ClusterIP for the WireGuard service. |
| service.wireguard.labels | object | `{}` | Additional labels for the WireGuard service. |
| service.wireguard.nodePort | int | `30180` | NodePort for the WireGuard service. |
| service.wireguard.port | int | `50180` | Port for the WireGuard service. |
| service.wireguard.type | string | `"NodePort"` | Service type for WireGuard (NodePort or LoadBalancer recommended). |
| serviceAccount.annotations | object | `{}` | Annotations for the ServiceAccount |
| serviceAccount.automount | bool | `false` | Automount API credentials for a ServiceAccount |
| serviceAccount.create | bool | `true` | Whether to create a ServiceAccount for Omni |
| serviceAccount.name | string | `""` | Name of the ServiceAccount to use. If not set and create is true, a name is generated using the fullname template. |
| skipVersionCheck | bool | `false` | Set to true to bypass the SemVer check (e.g. if using a custom build or nightly tag). |
| strategy.type | string | `"Recreate"` | Deployment strategy type. |
| terminationGracePeriodSeconds | int | `30` | Termination grace period in seconds for the Omni pod. |
| tolerations | list | `[]` | Tolerations for the Omni pod |
