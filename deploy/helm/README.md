# Omni Helm chart

## Prerequisites

To run Omni using this Helm chart the following additional components are required:

- An ingress controller e.g. [ingress-nginx](https://github.com/kubernetes/ingress-nginx)
- SSL certificates e.g. created via [cert-manager](https://cert-manager.io/)
- An Authentication provider. e.g. See [Configure Authentication](https://omni.siderolabs.com/how-to-guides/self_hosted/index#configure-authentication) for using Auth0.
- A GPG key. See [Create Etcd Encryption Key](https://omni.siderolabs.com/how-to-guides/self_hosted/index#create-etcd-encryption-key)
  The GPG key can be added to a Secret like:

  ```sh
  kubectl create secret -n omni generic gpg --from-file=omni.asc=./omni.asc
  ```

## Example Walkthrough

This example assumes 3 separate DNS entries will be used:

| Example URL                 | Purpose          |
| --------------------------- | ---------------- |
| omni.example.com            | UI and API       |
| kubernetes.omni.example.com | Kubernetes proxy |
| siderolink.omni.example.com | SideroLink       |

Assuming the above URLs and an Auth0 account the only values that need
to be modified are:

```yaml
domainName: omni.example.com 
accountUuid: <UUID> # This can be created using uuidgen
name: "MyOmniInstance"
auth:
  auth0:
    clientId: <AUTH0_CLIENT_ID>
    domain: <AUTH0_DOMAIN>
initialUsers:
  - <EMAIL>
service:
  siderolink:
    domainName: siderolink.omni.example.com
    wireguard:
      # The Helm chart currently assumes that Omni will be bound on a particular Node.
      address: <IP address of worker node>
  k8sProxy:
    domainName: kubernetes.omni.example.com
```

### Ingresses

Example Ingresses are given below for ingress-nginx. N.B. They also require SSL certs to exist.

#### api

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: GRPC
    nginx.ingress.kubernetes.io/proxy-body-size: 32m
    nginx.ingress.kubernetes.io/service-upstream: "true"
  labels:
    app.kubernetes.io/name: omni
  name: api
  namespace: omni
spec:
  ingressClassName: nginx
  rules:
  - host: omni.example.com
    http:
      paths:
      - backend:
          service:
            name: omni-internal-grpc
            port:
              number: 8080
        path: /cosi.resource.State
        pathType: ImplementationSpecific
      - backend:
          service:
            name: internal-grpc
            port:
              number: 8080
        path: /management.ManagementService
        pathType: ImplementationSpecific
      - backend:
          service:
            name: internal-grpc
            port:
              number: 8080
        path: /machine.MachineService
        pathType: ImplementationSpecific
      - backend:
          service:
            name: internal-grpc
            port:
              number: 8080
        path: /cluster.ClusterService
        pathType: ImplementationSpecific
      - backend:
          service:
            name: internal-grpc
            port:
              number: 8080
        path: /inspect.InspectService
        pathType: ImplementationSpecific
      - backend:
          service:
            name: internal-grpc
            port:
              number: 8080
        path: /resource.ResourceService
        pathType: ImplementationSpecific
      - backend:
          service:
            name: internal-grpc
            port:
              number: 8080
        path: /storage.StorageService
        pathType: ImplementationSpecific
      - backend:
          service:
            name: internal-grpc
            port:
              number: 8080
        path: /time.TimeService
        pathType: ImplementationSpecific
      - backend:
          service:
            name: internal-grpc
            port:
              number: 8080
        path: /auth.AuthService
        pathType: ImplementationSpecific
      - backend:
          service:
            name: internal-grpc
            port:
              number: 8080
        path: /oicd.
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - omni.example.com
    secretName: omni.example.com-tls
```

#### kubernetes-proxy

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kubernetes-proxy
  namespace: omni
spec:
  ingressClassName: nginx
  rules:
  - host: kubernetes.omni.example.com
    http:
      paths:
      - backend:
          service:
            name: internal
            port:
              number: 8095
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - kubernetes.omni.example.com
    secretName: kubernetes.omni.example.com-tls
```

#### siderolink

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: GRPC
  name: siderolink
  namespace: omni
spec:
  ingressClassName: nginx
  rules:
  - host: siderolink.omni.example.com
    http:
      paths:
      - backend:
          service:
            name: internal-grpc
            port:
              number: 8090
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - siderolink.omni.example.com
    secretName: siderolink.omni.example.com-tls
```

#### ui

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ui
  namespace: omni
spec:
  ingressClassName: nginx
  rules:
  - host: omni.example.com
    http:
      paths:
      - backend:
          service:
            name: internal
            port:
              number: 8080
        path: /
        pathType: Prefix
  tls:
  - hosts:
    - omni.example.com
    secretName: omni.example.com-tls
```
