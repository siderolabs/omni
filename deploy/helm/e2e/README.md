# KUTTL End-to-End Tests

This directory contains KUTTL (Kubernetes Test Tool) integration tests for the Omni Helm chart.

## Prerequisites

1. **Install KUTTL**:
   ```bash
   kubectl krew install kuttl
   ```

2. **Helm**: Helm v3+ installed

## Running Tests

KUTTL is configured to run tests in an existing Kubernetes cluster. The main test configuration is in the repository root.

### Run all tests:
```bash
# From deploy/helm/e2e directory
kubectl kuttl test

# Or specify config explicitly
kubectl kuttl test --config kuttl-test.yaml
```

### Run specific test:
```bash
kubectl kuttl test --test 01-omni
```

### Run with custom timeout:
```bash
kubectl kuttl test --timeout 600
```

### Skip cluster cleanup (for debugging):
```bash
kubectl kuttl test --skip-cluster-delete
```

## Test Structure

```
e2e/
├── kuttl-test.yaml              # KUTTL test suite configuration
├── _crds/                       # Custom Resource Definitions (currently empty)
├── _manifests/                  # Pre-test manifests (currently empty)
├── testdata/                    # Test data files (encryption keys, etc.)
└── tests/
    └── 01-omni/                 # Main test suite
        ├── 01-install-prerequisites.yaml  # Install cert-manager and ingress-nginx
        ├── 01-namespace.yaml              # Create test namespace
        ├── 02-selfsigned-certificate.yaml # Create self-signed TLS certificate
        ├── 03-install.yaml                # Install Omni chart with full config
        ├── 04-assert.yaml                 # Verify deployment is ready
        ├── 05-download-sa-key.yaml        # Download service account key
        ├── 06-test-omni-ingress.yaml      # Test Omni via ingress with omnictl
        └── 07-assert.yaml                 # Verify ingress test succeeded
```

## Test Scenarios

### 01-omni: Main Test Suite

Comprehensive test of core Omni functionality with multiple steps:

### Step 01: Install Prerequisites
- Installs cert-manager with CRDs enabled
- Installs ingress-nginx as NodePort service
- NodePort configuration: HTTP on 32080, HTTPS on 32443
- Configures ClusterIP: 10.96.0.100 for ingress controller
- Both components deployed with 180-second timeout

### Step 01: Create Namespace
- Creates dedicated test namespace
- Sets up environment for Omni deployment

### Step 02: Create Self-Signed Certificate
- Uses cert-manager to issue self-signed certificate
- Certificate for: omni.example.org, siderolink.omni.example.org, kubernetes.omni.example.org
- Creates TLS secret: `omni-tls-secret`
- All ingresses use this certificate

### Step 03: Install Omni
- Installs Omni Helm chart from `../../../v2/omni/`
- Configuration:
  - Uses `omni.asc` encryption key from testdata
  - Persistence disabled for testing
  - Image tag: latest
  - Skip version check enabled
  - Initial service account with Admin role (automation)
  - Auth0 configuration with dummy values
  - Initial user: user@example.org
  - SideroLink WireGuard endpoint: 10.5.0.2:50180
- Ingress Configuration:
  - Main UI ingress: omni.example.org
  - gRPC ingress: omni.example.org (backend-protocol: GRPC)
  - Kubernetes proxy ingress: kubernetes.omni.example.org
  - SideroLink API ingress: siderolink.omni.example.org
  - All ingresses use nginx IngressClass
  - All ingresses share the same TLS secret

### Step 04: Assert Deployment
- Deployment is ready with 1 replica
- All replicas available and ready

### Step 05: Download Service Account Key
- Retrieves initial service account key from Omni pod
- Stores key in Kubernetes secret `omni-sa-key`
- Key used for authentication in next test step

### Step 06: Test Omni Ingress
- Runs integration test via ingress
- Uses Alpine container with curl and omnictl
- Host aliases configured to resolve domains to ClusterIP 10.96.0.100
- Downloads and installs omnictl CLI
- Authenticates with service account key
- Tests:
  - Lists service accounts
  - Creates a MachineClass resource
  - Validates API functionality through ingress

### Step 07: Assert Ingress Test
- Verifies integration test job succeeded
- Confirms Omni is fully operational via ingress

## Configuration

### Test Suite Settings (kuttl-test.yaml)

The main test configuration is located at: `deploy/helm/e2e/kuttl-test.yaml`

- **startKIND**: `false` - Uses existing Kubernetes cluster (create with KIND manually if needed)
- **namespace**: `omni-e2e` - Fixed namespace for tests
- **crdDir**: `./_crds` - CRDs directory (currently empty, cert-manager CRDs installed via Helm)
- **manifestDirs**: `./_manifests` - Pre-test manifests directory
- **testDirs**: `./tests` - Test cases location
- **timeout**: `300` - Each step has 300 second timeout
- **parallel**: `1` - Tests run sequentially
- **commands**: Pre-test validation commands:
  - `kubectl kuttl version` - Verify KUTTL is installed
  - `helm version` - Verify Helm is available

## CI/CD Integration

### GitHub Actions Example:
```yaml
name: E2E Tests
on: [push, pull_request]
jobs:
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Install KUTTL
        run: |
          VERSION=0.24.0
          curl -LO https://github.com/kudobuilder/kuttl/releases/download/v${VERSION}/kubectl-kuttl_${VERSION}_linux_x86_64
          chmod +x kubectl-kuttl_${VERSION}_linux_x86_64
          sudo mv kubectl-kuttl_${VERSION}_linux_x86_64 /usr/local/bin/kubectl-kuttl
      
      - name: Install and Start KIND
        uses: helm/kind-action@v1
        with:
          cluster_name: omni-test
      
      - name: Run E2E tests
        run: |
          cd deploy/helm/e2e
          kubectl kuttl test
```

## Notes

- Tests require an existing Kubernetes cluster (use KIND or any other cluster)
- Configuration file is at: `deploy/helm/e2e/kuttl-test.yaml`
- cert-manager and ingress-nginx are installed as prerequisites
- Each test step has a 300-second timeout
- Tests run in fixed namespace `omni-e2e`
- Ingress tests require Kubernetes 1.19+ (KIND uses recent version)
- ClusterIP 10.96.0.100 is hardcoded for ingress controller
- NodePort 32080/32443 used for HTTP/HTTPS access
- Self-signed certificates used for TLS in tests
- Host aliases used in test pods to resolve ingress hosts to ClusterIP
- Service account key is generated and extracted during test execution
- omnictl binary is downloaded from latest GitHub release

### Test Configuration Notes

- Persistence disabled for faster testing
- Auth0 configured with dummy values (not used in tests)
- Initial service account created with Admin role for API testing
- All ingress endpoints share the same TLS certificate
- gRPC ingress uses special annotations for backend protocol
- Test validates full stack: deployment, ingress routing, and API functionality

