# Using the Context on the Client

The UI backend allows communication with 3 different systems:

- `kubernetes` - is using Kubernetes Go client underneath and talks to the
Kubernetes resource API.
- `talos` - talks directly to Talos node API.
COSI Resource API has almost the same request parameters and responses as Kubernetes resource API.
Also provides a way to call any machine APIs.
- `omni` - consists of COSI resource API describing the UI internal state.

The context plays an important role while doing resource APIs of any kinds and
doing Talos API requests.

Depending on the system kind context can contain different options.

Context can contain the following fields:

- `name` - kubeconfig context name.
- `cluster.uid` - CAPI cluster uid (from `metadata.uid`).
- `cluster.namespace` - CAPI cluster namespace (from `metadata.namespace`).
- `cluster.name` - CAPI cluster name (from `metadata.name`)
- `nodes` - Talos nodes IP.

## Kubernetes Context Kinds

Talking to the cluster from the Kubeconfig:

```typescript
context = {
  name: 'admin@talos-default'
}
```

Talking to the cluster which is managed by the cluster from the Kubeconfig:

```typescript
context = {
  name: 'admin@talos-default',
  cluster: {
    namespace: 'default',
    name: 'cluster-1',
    uid: '66be7aba-7c93-4766-b4ec-c5313fb831a2'
  }
}
```

## Talos Context Kinds

Talking to a node in the cluster from the Kubeconfig:

```typescript
context = {
  name: 'admin@talos-default',
  nodes: ['10.5.0.2']
}
```

Doing multiple nodes requests is also supported:

```typescript
context = {
  name: 'admin@talos-default',
  nodes: ['10.5.0.2', '10.5.0.3']
}
```

Talking to the nodes in the cluster managed by the cluster from the Kubeconfig:

```typescript
context = {
  name: 'admin@talos-default',
  cluster: {
    namespace: 'default',
    name: 'cluster-1',
    uid: '66be7aba-7c93-4766-b4ec-c5313fb831a2'
  },
  nodes: ['172.24.0.2']
}
```

### Omni Context Kinds

Omni context doesn't need any parameters as it doesn't proxy requests anywhere in that case.

## Context helpers methods

### `getContext` from `@/context`

Extracts context options from the URL query parameters.
Query should be in the following format:

```bash
/?name=<context name>&cluster=<cluster name>&namespace=<cluster namespace>&uid=<cluster uid>&node=<node ip>
```

> Note: it doesn't support multiple nodes

### `contextName` from `@/context`

Gets current Kubernetes context name defined in the local storage.

### `changeContext` from `@/context`

Changes current Kubernetes context.
