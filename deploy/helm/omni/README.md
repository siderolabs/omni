# Omni Helm Chart

A Helm chart for deploying Sidero Omni on Kubernetes clusters.

## Overview

Omni is a SaaS-native Talos Linux cluster fleet management platform that provides centralized management, monitoring, and orchestration capabilities for Talos Linux clusters. This Helm chart deploys Omni as a containerized application on Kubernetes with support for both embedded and external etcd configurations, automatic scaling, and comprehensive ingress management.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
  - [Add Helm Repository](#add-helm-repository)
  - [Install Chart](#install-chart)
- [Configuration](#configuration)
  - [Required Configuration](#required-configuration)
  - [Authentication Configuration](#authentication-configuration)
  - [Storage Configuration](#storage-configuration)
  - [Security Configuration](#security-configuration)
- [Values Reference](#values-reference)
  - [Global Configuration](#global-configuration)
  - [Deployment Configuration](#deployment-configuration)
  - [Service Configuration](#service-configuration)
  - [Authentication Configuration](#authentication-configuration-1)
  - [Resource Configuration](#resource-configuration)
  - [Volume Configuration](#volume-configuration)
  - [External etcd Configuration](#external-etcd-configuration)
  - [Ingress Configuration](#ingress-configuration)
  - [Pod Disruption Budget](#pod-disruption-budget)
  - [Per-Service Annotations](#per-service-annotations)
  - [Advanced Configuration](#advanced-configuration)
- [Architecture Decisions](#architecture-decisions)
  - [Deployment vs StatefulSet](#deployment-vs-statefulset)
  - [WireGuard Address Resolution](#wireguard-address-resolution)
  - [Service Architecture](#service-architecture)
- [Port Configuration](#port-configuration)
- [Security Considerations](#security-considerations)
  - [Required Capabilities](#required-capabilities)
  - [Device Plugin Requirements](#device-plugin-requirements)
  - [Network Policies](#network-policies)
- [Troubleshooting](#troubleshooting)
  - [Common Issues](#common-issues)
  - [Logs](#logs)
  - [Debug Mode](#debug-mode)
- [Migration Guide](#migration-guide)
  - [Migrating from Deployment to StatefulSet](#migrating-from-deployment-to-statefulset)
  - [Migrating to External etcd](#migrating-to-external-etcd)
- [Configuration Examples](#configuration-examples)
  - [Minimal Embedded etcd (StatefulSet)](#minimal-embedded-etcd-statefulset)
  - [Minimal External etcd (Deployment)](#minimal-external-etcd-deployment)
  - [Production with Ingress](#production-with-ingress)
  - [Development/Testing](#developmenttesting)
- [Upgrading](#upgrading)
  - [Backwards Compatibility](#backwards-compatibility)
  - [Backup](#backup)
  - [Upgrade Process](#upgrade-process)
- [Uninstalling](#uninstalling)
- [Contributing](#contributing)
- [License](#license)

## Prerequisites

- Kubernetes 1.19+
- Helm 3.2.0+
- PersistentVolume provisioner support in the underlying infrastructure
- Device plugin support for `/dev/net/tun` (required for WireGuard functionality)

## Installation

### Add Helm Repository

```bash
# Add the repository (if available)
helm repo add sidero https://charts.sidero.dev
helm repo update
```

### Install Chart

```bash
helm install omni sidero/omni \
  --namespace omni-system \
  --create-namespace \
  --set domainName=omni.example.com \
  --set accountUuid=your-account-uuid \
  --set auth.auth0.clientId=your-auth0-client-id \
  --set auth.auth0.domain=https://your-auth0-domain
```

## Configuration

### Required Configuration

The following values must be configured before deployment:

| Parameter | Description | Required |
|-----------|-------------|----------|
| `domainName` | Primary domain name for Omni API access | Yes |
| `accountUuid` | Unique account identifier | Yes |
| `auth.auth0.clientId` | Auth0 client ID (if using Auth0) | Conditional |
| `auth.auth0.domain` | Auth0 domain (if using Auth0) | Conditional |

### Authentication Configuration

Omni supports two authentication methods:

#### Auth0 Authentication

```yaml
auth:
  auth0:
    enabled: true
    clientId: "your-auth0-client-id"
    domain: "https://your-auth0-domain"
```

#### SAML Authentication

```yaml
auth:
  saml:
    enabled: true
    url: "https://your-saml-provider"
```

### Storage Configuration

#### Embedded etcd (Default)

**New Deployments**: When using embedded etcd (`etcd.external: false`), Omni is deployed as a StatefulSet with automatic PVC provisioning:

```yaml
etcd:
  external: false
volumes:
  etcd:
    size: "50Gi"
    storageClass: "fast-ssd"  # optional
```

**Existing Deployments**: Continue using Deployment with manual PVC management:

```yaml
volumes:
  etcd:
    persistentVolumeClaimName: omni-pvc  # Must exist before deployment
```

**Critical Limitation**: Embedded etcd is hardcoded to 1 replica because Omni's embedded etcd does not support clustering. The `deployment.replicaCount` setting is ignored when using embedded etcd. Attempting to scale beyond 1 replica would result in data corruption and split-brain scenarios.

**When to use embedded etcd**:
- Single-instance deployments
- Development and testing environments
- Small-scale production deployments where high availability is provided at the infrastructure level

**When to use external etcd**:
- Multi-replica deployments for high availability
- Large-scale production environments
- When you need horizontal scaling capabilities

#### External etcd

When using external etcd (`etcd.external: true`), Omni is deployed as a Deployment without persistent storage:

```yaml
etcd:
  external: true
  endpoints:
    - "https://etcd-1.example.com:2379"
    - "https://etcd-2.example.com:2379"
    - "https://etcd-3.example.com:2379"
```

This configuration enables horizontal scaling with `deployment.replicaCount > 1`.

### Security Configuration

#### GPG Key Configuration

Omni requires a GPG private key for signing operations:

```yaml
privateKeySource: "file:///omni.asc"
volumes:
  gpg:
    secretName: gpg-secret
```

Create the secret:
```bash
kubectl create secret generic gpg-secret \
  --from-file=omni.asc=/path/to/your/private.key \
  --namespace omni-system
```

#### TLS Configuration

For production deployments, configure TLS certificates:

```yaml
volumeMounts:
  tls:
    mountPath: "/etc/omni/tls"
    readOnly: true
volumes:
  tls:
    secretName: tls-secret
```

Create the TLS secret:
```bash
kubectl create secret tls tls-secret \
  --cert=/path/to/tls.crt \
  --key=/path/to/tls.key \
  --namespace omni-system
```

## Values Reference

### Global Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `nameOverride` | Override the chart name | `""` |
| `domainName` | Primary domain name for Omni | `omni.example.com` |
| `accountUuid` | Account UUID | `""` |
| `name` | Instance name | `"My Omni instance"` |
| `privateKeySource` | GPG private key source path | `"file:///omni.asc"` |
| `initialUsers` | List of initial user emails | `[]` |
| `includeGenericDevicePlugin` | Include generic device plugin | `true` |

### Deployment Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `deployment.image` | Container image repository | `ghcr.io/siderolabs/omni` |
| `deployment.tag` | Container image tag | `"latest"` |
| `deployment.replicaCount` | Number of replicas | `1` |
| `deployment.imagePullPolicy` | Image pull policy | `IfNotPresent` |
| `deployment.annotations` | Deployment annotations | `{}` |

### Service Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Kubernetes service type | `ClusterIP` |
| `service.siderolink.domainName` | Siderolink API domain | `omni.siderolink.example.com` |
| `service.siderolink.wireguard.address` | WireGuard advertised address (optional) | `""` |
| `service.siderolink.wireguard.port` | WireGuard service port | `30180` |
| `service.siderolink.wireguard.type` | WireGuard service type | `NodePort` |
| `service.siderolink.wireguard.externalTrafficPolicy` | Traffic policy for NodePort/LoadBalancer | `Cluster` |
| `service.k8sProxy.domainName` | Kubernetes proxy domain | `omni.kubernetes.example.com` |

#### WireGuard Service Configuration

The WireGuard service supports flexible addressing:

**Automatic DNS Resolution** (default):
```yaml
service:
  siderolink:
    wireguard:
      address: ""  # Uses wireguard.namespace.svc.cluster.local
```

**Explicit Address**:
```yaml
service:
  siderolink:
    wireguard:
      address: "192.168.1.100"  # External IP or FQDN
```

**Load Balancer Configuration**:
```yaml
service:
  siderolink:
    wireguard:
      type: LoadBalancer
      externalTrafficPolicy: Local  # Preserves client IP
```

### Authentication Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `auth.auth0.enabled` | Enable Auth0 authentication | `true` |
| `auth.auth0.clientId` | Auth0 client ID | `"123456"` |
| `auth.auth0.domain` | Auth0 domain | `"https://www.auth0.example"` |
| `auth.saml.enabled` | Enable SAML authentication | `false` |
| `auth.saml.url` | SAML provider URL | `"https://www.saml.example"` |

### Resource Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `128Mi` |
| `resources.limits.cpu` | CPU limit | `200m` |
| `resources.limits.memory` | Memory limit | `256Mi` |
| `resources.limits["squat.ai/tun"]` | TUN device limit | `1` |

### Volume Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `volumes.etcd.persistentVolumeClaimName` | etcd PVC name (existing deployments) | `omni-pvc` |
| `volumes.etcd.size` | etcd storage size (StatefulSet only) | `"50Gi"` |
| `volumes.etcd.storageClass` | Storage class for etcd PVC (optional) | `""` |
| `volumes.tls.secretName` | TLS secret name | `null` |
| `volumes.gpg.secretName` | GPG secret name | `gpg` |
| `volumeMounts.tls.mountPath` | TLS mount path | `null` |
| `volumeMounts.omniAsc.mountPath` | GPG key mount path | `"/omni.asc"` |

### External etcd Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `etcd.external` | Use external etcd cluster | `false` |
| `etcd.endpoints` | etcd cluster endpoints | `[]` |
| `etcd.username` | etcd username (direct) | `""` |
| `etcd.password` | etcd password (direct) | `""` |
| `etcd.auth.secretName` | Secret containing etcd credentials | `""` |
| `etcd.tls.enabled` | Enable TLS for etcd | `false` |
| `etcd.tls.secretName` | Secret containing TLS certificates | `""` |

#### etcd Authentication

**Direct credentials**:
```yaml
etcd:
  username: "omni-user"
  password: "secure-password"
```

**Secret-based credentials**:
```yaml
etcd:
  auth:
    secretName: "etcd-auth"
    usernameKey: "username"  # optional, defaults to "username"
    passwordKey: "password"  # optional, defaults to "password"
```

#### etcd TLS Configuration

**File paths**:
```yaml
etcd:
  tls:
    enabled: true
    certFile: "/etc/etcd/tls/client.crt"
    keyFile: "/etc/etcd/tls/client.key"
    caFile: "/etc/etcd/tls/ca.crt"
```

**Secret-based certificates**:
```yaml
etcd:
  tls:
    enabled: true
    secretName: "etcd-tls"
    certKey: "client.crt"  # optional, defaults to "client.crt"
    keyKey: "client.key"   # optional, defaults to "client.key"
    caKey: "ca.crt"        # optional, defaults to "ca.crt"
```

### Ingress Configuration

The chart supports four types of ingress resources:

| Ingress Type | Purpose | Default Host |
|--------------|---------|-------------|
| `api` | gRPC API endpoints | `omni.example.com` |
| `ui` | Web interface | `omni.example.com` |
| `siderolink` | Siderolink gRPC API | `siderolink.omni.example.com` |
| `kubernetesProxy` | Kubernetes API proxy | `kubernetes.omni.example.com` |

#### Basic Ingress Configuration

```yaml
ingress:
  api:
    enabled: true
    host: omni.example.com
    ingressClassName: nginx
    tls:
      enabled: true
      secretName: omni-api-tls
```

#### Cert-Manager Integration

```yaml
ingress:
  api:
    enabled: true
    certManager:
      enabled: true
      issuer: letsencrypt-prod
```

#### Kubernetes Proxy Wildcard

The Kubernetes proxy ingress automatically creates a wildcard rule (`*.kubernetes.omni.example.com`) to support tools like ArgoCD that require unique hostnames per cluster.

### Pod Disruption Budget

```yaml
podDisruptionBudget:
  enabled: true
  minAvailable: 1  # or use maxUnavailable
```

### Per-Service Annotations

Supports both global and per-service annotations:

```yaml
service:
  annotations:
    example.com/global: "value"  # Applied to all services
  internal:
    annotations:
      example.com/internal-only: "value"
  siderolink:
    wireguard:
      annotations:
        example.com/wireguard-only: "value"
```

### Advanced Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `extraArgs` | Additional container arguments | `[]` |
| `customVolumes` | Additional volumes | `[]` |
| `customVolumeMounts` | Additional volume mounts | `[]` |

## Backwards Compatibility

The chart maintains full backwards compatibility with existing deployments:

**Existing Deployments**:
- Charts deployed with previous versions continue using Deployment resources
- Storage configuration remains unchanged (manual PVC management)
- No disruption during upgrades
- `etcd.external` setting is ignored for existing deployments

**New Deployments**:
- `etcd.external: false` (default) → StatefulSet with automatic PVC provisioning
- `etcd.external: true` → Deployment for external etcd clusters

**Detection Logic**:
The chart uses Helm's `lookup` function to detect existing Deployment resources and automatically maintains compatibility.

## Architecture Decisions

### Deployment vs StatefulSet

The chart automatically chooses the appropriate Kubernetes resource based on deployment history and etcd configuration:

**Resource Selection Logic**:
1. **Existing Deployment detected** → Continue using Deployment (backwards compatibility)
2. **Existing StatefulSet detected** → Continue using StatefulSet (backwards compatibility)
3. **New deployment + `etcd.external: false`** → Use StatefulSet with embedded etcd
4. **New deployment + `etcd.external: true`** → Use Deployment with external etcd
5. **Resource type changes** → Only occur when switching etcd modes and no existing resource conflicts

**StatefulSet Benefits** (new deployments only):
- Automatic PVC provisioning per replica
- Stable network identities
- Ordered deployment and scaling
- Limited to 1 replica (embedded etcd constraint)

**Deployment Benefits**:
- Backwards compatibility with existing installations
- Horizontal scaling when using external etcd
- Simpler storage management for external etcd scenarios

### WireGuard Address Resolution

The WireGuard service supports both internal gRPC tunneling and external VPN connectivity:

- **Internal**: Uses Kubernetes DNS (`wireguard.namespace.svc.cluster.local`) for cluster-internal communication
- **External**: Allows explicit IP/FQDN configuration for external client connectivity

### Service Architecture

The chart deploys three Kubernetes services:

1. **internal**: Main Omni API service (ports 8080, 8090, 8095)
2. **internal-grpc**: gRPC service for internal communication (ports 8080, 8090)
3. **wireguard**: WireGuard VPN service (configurable type and port)

### Port Configuration

| Service | Port | Protocol | Description |
|---------|------|----------|-------------|
| omni | 8080 | TCP | Main API endpoint |
| siderolink | 8090 | TCP | Siderolink API |
| k8s-proxy | 8095 | TCP | Kubernetes proxy |
| wireguard | 30180 | UDP | WireGuard VPN |

## Security Considerations

### Required Capabilities

The Omni container requires the `NET_ADMIN` capability for WireGuard functionality:

```yaml
securityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
    add:
      - NET_ADMIN
```

### Device Plugin Requirements

WireGuard functionality requires access to `/dev/net/tun`. Ensure your cluster has the appropriate device plugin configured:

```yaml
resources:
  limits:
    squat.ai/tun: 1
```

### Network Policies

Consider implementing network policies to restrict traffic to Omni services based on your security requirements.

## Troubleshooting

### Common Issues

#### Pod Fails to Start

1. **Missing GPG Secret**: Ensure the GPG secret exists and contains the private key
2. **Storage Issues** (embedded etcd): Verify storage class and PVC provisioning
3. **etcd Connection** (external etcd): Check endpoints, credentials, and TLS configuration
4. **Device Plugin**: Confirm the TUN device plugin is available on nodes

#### Scaling Issues

1. **StatefulSet Scaling**: Cannot scale beyond 1 replica with embedded etcd - this is enforced by the chart
2. **Replica Count Ignored**: `deployment.replicaCount > 1` is ignored when `etcd.external: false`
3. **External etcd Required**: Use `etcd.external: true` for multiple replicas
4. **Data Corruption Risk**: Never attempt to manually scale the StatefulSet beyond 1 replica

#### Service Connectivity

1. **WireGuard Address**: Verify address resolution (internal DNS vs external IP)
2. **Ingress Configuration**: Check ingress class and TLS certificate availability

#### Authentication Issues

1. **Auth0 Configuration**: Verify client ID and domain are correct
2. **SAML Configuration**: Ensure SAML metadata is properly configured

#### Network Connectivity

1. **Service Discovery**: Verify DNS resolution within the cluster
2. **WireGuard**: Check NodePort accessibility and firewall rules

### Logs

View Omni logs:
```bash
kubectl logs -n omni-system deployment/omni
```

### Debug Mode

Enable debug logging:
```yaml
extraArgs:
  - --debug
```

## Migration Guide

### Migrating from Deployment to StatefulSet

To migrate an existing Deployment-based installation to StatefulSet (for better storage management):

1. **Backup etcd data**:
```bash
kubectl exec -n omni-system deployment/omni -- tar -czf /tmp/etcd-backup.tar.gz /_out
kubectl cp omni-system/$(kubectl get pod -n omni-system -l app.kubernetes.io/name=omni -o jsonpath='{.items[0].metadata.name}'):/tmp/etcd-backup.tar.gz ./etcd-backup.tar.gz
```

2. **Delete existing Deployment** (this will cause downtime):
```bash
helm uninstall omni --namespace omni-system
kubectl delete pvc omni-pvc --namespace omni-system
```

3. **Reinstall with StatefulSet**:
```bash
helm install omni sidero/omni \
  --namespace omni-system \
  --create-namespace \
  --set etcd.external=false \
  --set domainName=your-domain.com \
  --set accountUuid=your-account-uuid
```

4. **Restore etcd data**:
```bash
kubectl cp ./etcd-backup.tar.gz omni-system/omni-0:/tmp/etcd-backup.tar.gz
kubectl exec -n omni-system omni-0 -- tar -xzf /tmp/etcd-backup.tar.gz -C /
```

### Migrating to External etcd

To migrate from embedded etcd to external etcd:

1. **Set up external etcd cluster** (outside scope of this guide)

2. **Backup embedded etcd data**:
```bash
kubectl exec -n omni-system deployment/omni -- tar -czf /tmp/etcd-backup.tar.gz /_out
kubectl cp omni-system/$(kubectl get pod -n omni-system -l app.kubernetes.io/name=omni -o jsonpath='{.items[0].metadata.name}'):/tmp/etcd-backup.tar.gz ./etcd-backup.tar.gz
```

3. **Restore data to external etcd** (use etcd restore tools)

4. **Update Helm values**:
```yaml
etcd:
  external: true
  endpoints:
    - "https://etcd-1.example.com:2379"
    - "https://etcd-2.example.com:2379"
    - "https://etcd-3.example.com:2379"
deployment:
  replicaCount: 3  # Now supports multiple replicas
```

5. **Upgrade deployment**:
```bash
helm upgrade omni sidero/omni \
  --namespace omni-system \
  --values values.yaml
```

## Configuration Examples

### Minimal Embedded etcd (StatefulSet)

```yaml
# values-embedded.yaml
domainName: omni.example.com
accountUuid: "12345678-1234-1234-1234-123456789012"

auth:
  auth0:
    enabled: true
    clientId: "your-auth0-client-id"
    domain: "https://your-auth0-domain"

etcd:
  external: false

volumes:
  etcd:
    size: "100Gi"
    storageClass: "fast-ssd"
  gpg:
    secretName: "omni-gpg"
```

### Minimal External etcd (Deployment)

```yaml
# values-external-etcd.yaml
domainName: omni.example.com
accountUuid: "12345678-1234-1234-1234-123456789012"

deployment:
  replicaCount: 3

auth:
  auth0:
    enabled: true
    clientId: "your-auth0-client-id"
    domain: "https://your-auth0-domain"

etcd:
  external: true
  endpoints:
    - "https://etcd-1.example.com:2379"
    - "https://etcd-2.example.com:2379"
    - "https://etcd-3.example.com:2379"
  username: "omni"
  password: "secure-password"
  tls:
    enabled: true
    certFile: "/etc/etcd/tls/client.crt"
    keyFile: "/etc/etcd/tls/client.key"
    caFile: "/etc/etcd/tls/ca.crt"

volumes:
  gpg:
    secretName: "omni-gpg"
```

### Production with Ingress

```yaml
# values-production.yaml
domainName: omni.example.com
accountUuid: "12345678-1234-1234-1234-123456789012"

deployment:
  replicaCount: 3

auth:
  auth0:
    enabled: true
    clientId: "your-auth0-client-id"
    domain: "https://your-auth0-domain"

etcd:
  external: true
  endpoints:
    - "https://etcd-1.example.com:2379"
    - "https://etcd-2.example.com:2379"
    - "https://etcd-3.example.com:2379"
  auth:
    secretName: "etcd-credentials"
  tls:
    enabled: true
    secretName: "etcd-tls"

service:
  siderolink:
    wireguard:
      type: LoadBalancer
      externalTrafficPolicy: Local

ingress:
  api:
    enabled: true
    host: omni.example.com
    ingressClassName: nginx
    certManager:
      enabled: true
      issuer: letsencrypt-prod
    tls:
      enabled: true
      secretName: omni-api-tls
  ui:
    enabled: true
    host: omni.example.com
    ingressClassName: nginx
    certManager:
      enabled: true
      issuer: letsencrypt-prod
    tls:
      enabled: true
      secretName: omni-ui-tls
  siderolink:
    enabled: true
    host: siderolink.omni.example.com
    ingressClassName: nginx
    certManager:
      enabled: true
      issuer: letsencrypt-prod
    tls:
      enabled: true
      secretName: omni-siderolink-tls
  kubernetesProxy:
    enabled: true
    host: kubernetes.omni.example.com
    ingressClassName: nginx
    certManager:
      enabled: true
      issuer: letsencrypt-prod
    tls:
      enabled: true
      secretName: omni-kubernetes-proxy-tls

podDisruptionBudget:
  enabled: true
  minAvailable: 2

volumes:
  gpg:
    secretName: "omni-gpg"
  tls:
    secretName: "omni-tls"

resources:
  requests:
    cpu: 500m
    memory: 1Gi
  limits:
    cpu: 2000m
    memory: 4Gi
```

### Development/Testing

```yaml
# values-dev.yaml
domainName: omni-dev.example.com
accountUuid: "12345678-1234-1234-1234-123456789012"

auth:
  auth0:
    enabled: true
    clientId: "your-dev-auth0-client-id"
    domain: "https://your-dev-auth0-domain"

etcd:
  external: false

volumes:
  etcd:
    size: "10Gi"
  gpg:
    secretName: "omni-gpg-dev"

resources:
  requests:
    cpu: 100m
    memory: 256Mi
  limits:
    cpu: 500m
    memory: 512Mi

extraArgs:
  - --debug
```

## Upgrading

### Backwards Compatibility

Upgrading from previous chart versions is fully supported:

- **Existing Deployments**: Continue using the same Deployment resource and storage configuration
- **No Resource Changes**: The chart automatically detects existing deployments and maintains compatibility
- **Configuration Preserved**: All existing values and storage remain unchanged

### Backup

Before upgrading, backup the etcd data:

**For Deployment-based installations**:
```bash
kubectl exec -n omni-system deployment/omni -- tar -czf /tmp/etcd-backup.tar.gz /_out
kubectl cp omni-system/$(kubectl get pod -n omni-system -l app.kubernetes.io/name=omni -o jsonpath='{.items[0].metadata.name}'):/tmp/etcd-backup.tar.gz ./etcd-backup.tar.gz
```

**For StatefulSet-based installations**:
```bash
kubectl exec -n omni-system statefulset/omni -- tar -czf /tmp/etcd-backup.tar.gz /_out
kubectl cp omni-system/omni-0:/tmp/etcd-backup.tar.gz ./etcd-backup.tar.gz
```

### Upgrade Process

```bash
helm upgrade omni sidero/omni \
  --namespace omni-system \
  --reuse-values
```

## Uninstalling

```bash
helm uninstall omni --namespace omni-system
```

Note: This will not delete PVCs. Remove them manually if needed:

**For Deployment-based installations**:
```bash
kubectl delete pvc omni-pvc --namespace omni-system
```

**For StatefulSet-based installations**:
```bash
kubectl delete pvc etcd-data-omni-0 --namespace omni-system
```

## Contributing

For issues and contributions, please refer to the [Sidero Labs GitHub repository](https://github.com/siderolabs/omni).

## License

This chart is licensed under the Mozilla Public License 2.0. See the [LICENSE](https://github.com/siderolabs/omni/blob/main/LICENSE) file for details.