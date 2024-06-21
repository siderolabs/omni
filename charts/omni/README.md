# Sidero Omni

![Version: 1.0.0](https://img.shields.io/badge/Version-1.0.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.38.0](https://img.shields.io/badge/AppVersion-v0.38.0-informational?style=flat-square)

## Description

A Helm chart to deploy Omni on Kubernetes

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Kevin Tijssen |  |  |

## Installing the Chart

To install the chart with the release name omni run:

```bash
git clone git@github.com:siderolabs/omni.git
helm install omni charts/omni --version 1.0.0
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| annotations | object | `{}` | Additional deployment annotations |
| authentication.auth0.clientId | string | `""` | Auth0 Client ID |
| authentication.auth0.domain | string | `""` | Auth0 Domain |
| authentication.auth0.initialUsersEmailAddress | string | `""` | Email address |
| authentication.saml.url | string | `""` | SAML URL |
| authentication.type | string | `"saml"` | Which authentication type |
| dind.extraVolumeMounts | list | `[]` | Additional volumeMounts on the output Deployment definition. |
| dind.image.pullPolicy | string | `"IfNotPresent"` | Docker image pull policy |
| dind.image.registry | string | `"docker.io"` | Docker image host registry |
| dind.image.repository | string | `"docker"` | Docker image repository |
| dind.image.tag | string | `"26.1.4-dind"` | Docker image tag |
| dind.securityContext | object | `{"privileged":true}` | Set the container security context |
| etcd.encryptionKeySecret | string | `""` | existing Secret that contains the ETCD encryption key |
| extraVolumeMounts | list | `[]` | Additional volumeMounts on the output Deployment definition. |
| extraVolumes | list | `[]` | Additional volumes on the output Deployment definition. |
| fullnameOverride | string | `""` | Overrides the chart's computed fullname |
| image.pullPolicy | string | `"IfNotPresent"` | Omni image pull policy |
| image.registry | string | `"ghcr.io"` | Omni image host registry |
| image.repository | string | `"siderolabs/omni"` | Omni image repository |
| image.tag | string | `"v0.38.0"` | Omni image tag |
| imagePullSecrets | list | `[]` | Additional imagePullSecrets |
| nameOverride | string | `""` | Overrides the chart's name |
| namespaceOverride | string | `""` | Override the deployment namespace; defaults to .Release.Namespace |
| nodeSelector | object | `{}` |  |
| omniAccount.annotations | object | `{}` | Additional configMap annotations |
| omniAccount.existingConfigMap | string | `""` | Or use a existing configMap with uuid as key |
| omniAccount.uuid | string | `""` | This can be done with the following command: `uuidgen`. |
| persistence.accessMode | string | `"ReadWriteOnce"` |  |
| persistence.accessModes | list | `["ReadWriteOnce"]` | Which AccessModes are used |
| persistence.annotations | object | `{}` | Additional pvc annotations |
| persistence.enabled | bool | `true` | Enable persistence for the ETCD database |
| persistence.existingClaim | string | `""` | Name of a existing PVC |
| persistence.size | string | `"5Gi"` | Size of the pvc |
| persistence.storageClass | string | `""` | Name of the StorageClass (If empty the default be used) |
| podAnnotations | object | `{}` | Additional pod annotations |
| podLabels | object | `{}` | Additional pod labels |
| podSecurityContext | object | `{}` |  |
| replicaCount | int | `1` | Number of pods of the deployment (only applies for Omni Deployment) |
| resources | object | `{}` |  |
| securityContext | object | `{"capabilities":{"add":["NET_ADMIN"]}}` | Set the container security context |
| service.annotations | object | `{}` | Additional service annotations |
| service.events.nodePort | int | `32091` |  |
| service.events.port | int | `8091` |  |
| service.https | object | `{"nodePort":32443,"port":443}` | Exposed services |
| service.k8sProxy.nodePort | int | `32100` |  |
| service.k8sProxy.port | int | `8100` |  |
| service.loadBalancerIP | string | `""` | IP Address of the LoadBalancer |
| service.machineApi.nodePort | int | `32090` |  |
| service.machineApi.port | int | `8090` |  |
| service.type | string | `"LoadBalancer"` | Type of service |
| service.wireguard.nodePort | int | `32180` |  |
| service.wireguard.port | int | `50180` |  |
| tls.domain | string | `""` | Domain that is used by Omni |
| tls.existingTlsSecret | string | `""` | existing Secret that contains the certificate and key |
| tolerations | list | `[]` |  |
| wireguard.ipAddr | string | `""` | IP Address of the public-ip, host or service that exposes wireguard |

