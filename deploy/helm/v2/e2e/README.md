# KUTTL End-to-End Tests

This directory contains [KUTTL](https://kuttl.dev/) end-to-end tests for the Omni Helm chart.

## Prerequisites

1. A running Kubernetes cluster (e.g., KIND)
2. [KUTTL](https://kuttl.dev/) installed (`kubectl krew install kuttl`)
3. Helm v3+

## Running Tests

```bash
kubectl kuttl test --config kuttl-test.yaml
```

Useful flags:

```bash
kubectl kuttl test --test 01-omni          # Run specific test
kubectl kuttl test --timeout 600           # Custom timeout
kubectl kuttl test --skip-cluster-delete   # Keep resources after test (for debugging)
```

## Test Structure

```
e2e/
├── kuttl-test.yaml              # Test suite configuration
├── _crds/                       # CRDs (empty - cert-manager CRDs installed via Helm)
├── _manifests/                  # Pre-test manifests (empty)
├── testdata/                    # Test data files (encryption keys, etc.)
└── tests/
    └── 01-omni/                 # Main test suite
        ├── 01-install-prerequisites.yaml  # Install cert-manager and ingress-nginx
        ├── 01-namespace.yaml              # Create test namespace
        ├── 02-selfsigned-certificate.yaml # Create self-signed TLS certificate
        ├── 03-install.yaml                # Install Omni chart with full config
        ├── 04-assert.yaml                 # Verify deployment is ready
        ├── 05-download-sa-key.yaml        # Extract service account key from pod
        ├── 06-test-omni-ingress.yaml      # Test Omni API via ingress with omnictl
        └── 07-assert.yaml                 # Verify test job succeeded
```

## Test Flow

Steps `01-install-prerequisites` and `01-namespace` run together (same KUTTL step prefix), followed by the remaining steps sequentially:

1. Install cert-manager and ingress-nginx (ClusterIP `10.96.0.100`, NodePorts `32080`/`32443`)
2. Create a self-signed TLS certificate for `omni.example.org` and subdomains
3. Install Omni via Helm with test configuration (persistence disabled, dummy Auth0, `image.tag=latest`)
4. Assert the deployment is ready
5. Extract the initial service account key from the Omni pod using an ephemeral debug container
6. Run omnictl commands through ingress (list service accounts, create MachineClass, create cluster template)
7. Assert the test job succeeded

## Notes

- Tests run in namespace `omni-e2e` with a 300-second timeout per step
- Host aliases in test pods resolve ingress hostnames to the ingress controller ClusterIP
- Auth0 is configured with dummy values (not actually used)
- The `omni.asc` GPG key in `testdata/` is for etcd encryption in the test environment only
