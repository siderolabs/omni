// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	vault "github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/kubernetes"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/backend/runtime/keyprovider"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// Loader is an interface that returns a private key.
type Loader interface {
	PrivateKey() (keyprovider.PrivateKeyData, error)
}

// NewLoader parses the source string and returns a Loader.
// The source string can be one of the following:
//
//	vault://secret/omni-account/etcdEnc
//
//		    Use VAULT_ADDR and VAULT_TOKEN for auth, and grab the private key from "secret" KVv2 store at the "omni-account/etcdEnc" path.
//
//		    OR
//
//		    If "VAULT_K8S_ROLE" env set, use Kubernetes Service Account auth with the default token, and the private key in the "secret" KVv2 store at the "omni-account/etcEnc" path.
//
//	 vault://@/path/to/token:/secret/omni-account/etcdEnc
//
//			If "VAULT_K8S_ROLE" env set, use Kubernetes Service Account auth with the token at "/path/to/token", and the private key in the "secret" KVv2 store at the "omni-account/etcEnc" path.
//
//	 file:///path/to/file
//
//			path to a private key file
func NewLoader(source string, logger *zap.Logger, vaultConfig config.Vault) (Loader, error) {
	if source == "" {
		return nil, errors.New("private key source is not set")
	}

	switch {
	case os.Getenv("VAULT_K8S_ROLE") != "" && (vaultMatcher.MatchString(source) || vaultTokenMatcher.MatchString(source)):
		return makeVaultK8sLoader(source, os.Getenv("VAULT_K8S_ROLE"), vaultConfig.K8SAuthMountPath, logger)
	case vaultMatcher.MatchString(source):
		return makeVaultHTTPLoader(source, logger, vaultConfig)
	case strings.HasPrefix(source, "file://"):
		return makeFileLoader(source, logger)
	}

	return nil, fmt.Errorf("unknown private key source '%s'", source)
}

func makeFileLoader(source string, logger *zap.Logger) (Loader, error) {
	source = strings.TrimPrefix(source, "file://")
	if source == "" {
		return nil, errors.New("file source is not set")
	}

	return &FileLoader{Filepath: source, logger: logger}, nil
}

// FileLoader loads a private key from a file.
type FileLoader struct {
	logger *zap.Logger

	Filepath string
}

// PrivateKey loads a private key from a file.
func (f *FileLoader) PrivateKey() (keyprovider.PrivateKeyData, error) {
	privateKey, err := os.ReadFile(f.Filepath)
	if err != nil {
		return keyprovider.PrivateKeyData{}, fmt.Errorf("failed to read private key file '%s': %w", f.Filepath, err)
	}

	data, err := keyprovider.MakePrivateKeyData(string(privateKey))
	if err != nil {
		return keyprovider.PrivateKeyData{}, fmt.Errorf("failed to make private key data: %w", err)
	}

	return data, nil
}

func makeVaultHTTPLoader(source string, logger *zap.Logger, vaultConfig config.Vault) (Loader, error) {
	matched := vaultMatcher.FindStringSubmatch(source)
	if len(matched) != 3 {
		return nil, errors.New("failed to parse vault-url source")
	}

	mount := matched[1]
	secretPath := matched[2]

	token, ok := os.LookupEnv("VAULT_TOKEN")
	if !ok {
		token = vaultConfig.GetToken()

		if token == "" {
			return nil, errors.New("VAULT_TOKEN is not set")
		}
	}

	addr, ok := os.LookupEnv("VAULT_ADDR")
	if !ok {
		addr = vaultConfig.GetUrl()
		if addr == "" {
			return nil, errors.New("VAULT_ADDR is not set")
		}
	}

	return &VaultHTTPLoader{
		logger:     logger,
		Token:      token,
		Address:    addr,
		Mount:      mount,
		SecretPath: secretPath,
	}, nil
}

// VaultHTTPLoader loads a private key from a Vault HTTP endpoint.
type VaultHTTPLoader struct {
	logger *zap.Logger

	Token      string
	Address    string
	Mount      string
	SecretPath string
}

// PrivateKey loads a private key from a Vault instance using Kubernetes authentication.
func (v *VaultHTTPLoader) PrivateKey() (keyprovider.PrivateKeyData, error) {
	config := vault.DefaultConfig()
	config.Address = v.Address

	client, err := vault.NewClient(config)
	if err != nil {
		return keyprovider.PrivateKeyData{}, fmt.Errorf("failed to create vault client: %w", err)
	}

	client.SetToken(v.Token)

	v.logger.Info(
		"loading private key using Vault HTTP api",
		zap.String("address", v.Address),
		zap.String("mount_path", v.Mount),
		zap.String("secret_path", v.SecretPath),
	)

	return loadKeyData(client, v.Mount, v.SecretPath)
}

func makeVaultK8sLoader(source string, role string, k8sAuthMountPath *string, logger *zap.Logger) (Loader, error) {
	var (
		tokenPath  string
		mount      string
		secretPath string
	)

	if matched := vaultTokenMatcher.FindStringSubmatch(source); len(matched) == 4 {
		tokenPath = matched[1]
		mount = matched[2]
		secretPath = matched[3]
	} else if matched := vaultMatcher.FindStringSubmatch(source); len(matched) == 3 {
		mount = matched[1]
		secretPath = matched[2]
	} else {
		return nil, errors.New("failed to parse k8s vault source")
	}

	return &VaultK8sLoader{
		Role:             role,
		TokenPath:        tokenPath,
		K8sAuthMountPath: k8sAuthMountPath,
		Mount:            mount,
		SecretPath:       secretPath,
		logger:           logger,
	}, nil
}

// VaultK8sLoader loads a private key from a Vault instance using Kubernetes authentication.
type VaultK8sLoader struct {
	logger *zap.Logger

	Role             string
	TokenPath        string
	K8sAuthMountPath *string
	Mount            string
	SecretPath       string
}

// PrivateKey loads a private key from a Vault instance using Kubernetes authentication.
func (v *VaultK8sLoader) PrivateKey() (keyprovider.PrivateKeyData, error) {
	client, err := vault.NewClient(nil)
	if err != nil {
		return keyprovider.PrivateKeyData{}, fmt.Errorf("failed to create vault client: %w", err)
	}

	var opts []auth.LoginOption
	if v.TokenPath != "" {
		opts = append(opts, auth.WithServiceAccountTokenPath(v.TokenPath))
	}

	if v.K8sAuthMountPath != nil {
		opts = append(opts, auth.WithMountPath(*v.K8sAuthMountPath))
	}

	k8sAuth, err := auth.NewKubernetesAuth(
		v.Role,
		opts...,
	)
	if err != nil {
		return keyprovider.PrivateKeyData{}, fmt.Errorf("failed to create k8s auth: %w", err)
	}

	authInfo, err := client.Auth().Login(context.TODO(), k8sAuth)
	if err != nil {
		return keyprovider.PrivateKeyData{}, fmt.Errorf("unable to log in with Kubernetes auth: %w", err)
	}

	if authInfo == nil {
		return keyprovider.PrivateKeyData{}, errors.New("no auth info was returned after login")
	}

	v.logger.Info(
		"loading private key using k8s Vault",
		zap.String("role", v.Role),
		zap.String("mount_path", v.Mount),
		zap.String("secret_path", v.SecretPath),
	)

	return loadKeyData(client, v.Mount, v.SecretPath)
}

func loadKeyData(client *vault.Client, mount string, secretPath string) (keyprovider.PrivateKeyData, error) {
	secret, err := client.KVv2(mount).Get(context.Background(), secretPath)
	if err != nil {
		return keyprovider.PrivateKeyData{}, fmt.Errorf("failed to get omni private key: %w", err)
	}

	extractor := func() (string, bool) {
		// vault can return either a simple string or a []any{string, string, string}.
		privateKey, ok := secret.Data["private-key"].(string)
		if ok {
			return privateKey, true
		}

		// it's not a simple string, so it's probably a []any{string, string, string}.
		slc, ok := secret.Data["private-key"].([]any)
		if !ok {
			return "", false
		}

		var sb strings.Builder

		for _, v := range slc {
			privateKey, ok = v.(string)
			if !ok {
				return "", false
			}

			sb.WriteString(privateKey)
			sb.WriteByte('\n')
		}

		return sb.String(), sb.Len() > 0
	}

	privateKey, ok := extractor()
	if !ok {
		return keyprovider.PrivateKeyData{}, errors.New("failed to get omni private key")
	}

	data, err := keyprovider.MakePrivateKeyData(privateKey)
	if err != nil {
		return keyprovider.PrivateKeyData{}, fmt.Errorf("failed to make private key data: %w", err)
	}

	return data, nil
}

var (
	vaultTokenMatcher = regexp.MustCompile(`vault://@([a-zA-Z0-9-/]+):/([a-zA-Z0-9-]+)/([a-zA-Z0-9-/]+)`)
	vaultMatcher      = regexp.MustCompile(`vault://([a-zA-Z0-9-]+)/([a-zA-Z0-9-/]+)`)
)
