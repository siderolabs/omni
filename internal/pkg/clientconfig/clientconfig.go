// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package clientconfig holds the configuration for the test client for Omni API.
package clientconfig

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"net/http"
	"runtime"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/containers"
	authpb "github.com/siderolabs/go-api-signature/api/auth"
	authcli "github.com/siderolabs/go-api-signature/pkg/client/auth"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"github.com/siderolabs/go-api-signature/pkg/serviceaccount"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/client"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	omnisa "github.com/siderolabs/omni/internal/pkg/auth/serviceaccount"
)

const (
	DefaultServiceAccount = "integration" + access.ServiceAccountNameSuffix
	defaultEmail          = "test-user@siderolabs.com"
)

type clientCacheKey struct {
	role         string
	email        string
	skipUserRole bool
}

type clientCacheValue struct {
	client *client.Client
	key    *pgp.Key
	err    error
}

// ClientConfig is a test client.
type ClientConfig struct {
	endpoint          string
	serviceAccountKey string
	clientCache       containers.ConcurrentMap[clientCacheKey, clientCacheValue]
}

// New creates a new test client config.
func New(endpoint, serviceAccountKey string) *ClientConfig {
	return &ClientConfig{
		endpoint:          endpoint,
		serviceAccountKey: serviceAccountKey,
	}
}

// GetClient returns a test client for the default test email.
//
// Clients are cached by their configuration, so if a client with the
// given configuration was created before, the cached one will be returned.
func (t *ClientConfig) GetClient(ctx context.Context, publicKeyOpts ...authcli.RegisterPGPPublicKeyOption) (*client.Client, error) {
	return t.GetClientForEmail(ctx, DefaultServiceAccount, publicKeyOpts...)
}

// GetClientForEmail returns a test client for the given email.
//
// Clients are cached by their configuration, so if a client with the
// given configuration was created before, the cached one will be returned.
func (t *ClientConfig) GetClientForEmail(ctx context.Context, email string, publicKeyOpts ...authcli.RegisterPGPPublicKeyOption) (*client.Client, error) {
	cacheKey := t.buildCacheKey(email, publicKeyOpts)

	// The client is created by the cache callback, and will be closed by the cache on [ClientConfig.Close].
	cliValue, _ := t.clientCache.GetOrCall(cacheKey, func() clientCacheValue {
		cli, key, err := createServiceAccountClient(ctx, t.endpoint, t.serviceAccountKey, cacheKey)

		return clientCacheValue{
			client: cli,
			key:    key,
			err:    err,
		}
	})

	return cliValue.client, cliValue.err
}

// GetKey fetches service account key for the default email.
func (t *ClientConfig) GetKey(ctx context.Context, publicKeyOpts ...authcli.RegisterPGPPublicKeyOption) (*pgp.Key, error) {
	return t.GetKeyForEmail(ctx, DefaultServiceAccount, publicKeyOpts...)
}

// GetKeyForEmail fetches service account key for the specified email.
func (t *ClientConfig) GetKeyForEmail(ctx context.Context, email string, publicKeyOpts ...authcli.RegisterPGPPublicKeyOption) (*pgp.Key, error) {
	cacheKey := t.buildCacheKey(email, publicKeyOpts)

	// The client is created by the cache callback, and will be closed by the cache on [ClientConfig.Close].
	cliOrErr, _ := t.clientCache.GetOrCall(cacheKey, func() clientCacheValue {
		cli, key, err := createServiceAccountClient(ctx, t.endpoint, t.serviceAccountKey, cacheKey)

		return clientCacheValue{
			client: cli,
			key:    key,
			err:    err,
		}
	})

	return cliOrErr.key, cliOrErr.err
}

// Close closes all the clients created by this config.
func (t *ClientConfig) Close() error {
	var multiErr error

	t.clientCache.ForEach(func(_ clientCacheKey, cliOrErr clientCacheValue) {
		if cliOrErr.client != nil {
			if err := cliOrErr.client.Close(); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}
		}
	})

	return multiErr
}

func (t *ClientConfig) buildCacheKey(email string, publicKeyOpts []authcli.RegisterPGPPublicKeyOption) clientCacheKey {
	var req authpb.RegisterPublicKeyRequest

	for _, o := range publicKeyOpts {
		o(&req)
	}

	return clientCacheKey{
		role:         req.Role,
		email:        email,
		skipUserRole: req.SkipUserRole,
	}
}

// SignHTTPRequest signs the regular HTTP request using the default test email.
func SignHTTPRequest(ctx context.Context, client *client.Client, req *http.Request) error {
	return SignHTTPRequestWithEmail(ctx, client, req, defaultEmail)
}

// SignHTTPRequestWithEmail signs the regular HTTP request using the given email.
func SignHTTPRequestWithEmail(ctx context.Context, client *client.Client, req *http.Request, email string) error {
	newKey, err := pgp.GenerateKey("", "", email, 4*time.Hour)
	if err != nil {
		return err
	}

	err = registerKey(ctx, client.Auth(), newKey, email)
	if err != nil {
		return err
	}

	msg, err := message.NewHTTP(req)
	if err != nil {
		return err
	}

	return msg.Sign(email, newKey)
}

// RegisterKeyGetIDSignatureBase64 registers a new public key with the default test email and returns its ID and the base-64 encoded signature of the same ID.
func RegisterKeyGetIDSignatureBase64(ctx context.Context, client *client.Client) (id, idSignatureBase66 string, err error) {
	newKey, err := pgp.GenerateKey("", "", defaultEmail, 4*time.Hour)
	if err != nil {
		return "", "", err
	}

	err = registerKey(ctx, client.Auth(), newKey, defaultEmail)
	if err != nil {
		return "", "", err
	}

	id = newKey.Fingerprint()

	signedIDBytes, err := newKey.Sign([]byte(id))
	if err != nil {
		return "", "", err
	}

	idSignatureBase66 = base64.StdEncoding.EncodeToString(signedIDBytes)

	return id, idSignatureBase66, nil
}

// CreateServiceAccount using the direct access to the Omni state.
func CreateServiceAccount(ctx context.Context, name string, st state.State) (string, error) {
	// generate a new PGP key with long lifetime
	comment := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	serviceAccountEmail := name + access.ServiceAccountNameSuffix

	key, err := pgp.GenerateKey(name, comment, serviceAccountEmail, auth.ServiceAccountMaxAllowedLifetime)
	if err != nil {
		return "", err
	}

	armoredPublicKey, err := key.ArmorPublic()
	if err != nil {
		return "", err
	}

	identity, err := safe.ReaderGetByID[*authres.Identity](ctx, st, serviceAccountEmail)
	if err != nil && !state.IsNotFoundError(err) {
		return "", err
	}

	if identity != nil {
		err = omnisa.Destroy(ctx, st, name)
		if err != nil {
			return "", err
		}
	}

	_, err = omnisa.Create(ctx, st, name, string(role.Admin), false, []byte(armoredPublicKey))
	if err != nil {
		return "", err
	}

	return serviceaccount.Encode(name, key)
}

func createServiceAccountClient(ctx context.Context, endpoint, serviceAccountKey string, cacheKey clientCacheKey) (*client.Client, *pgp.Key, error) {
	rootClient, err := client.New(endpoint, client.WithServiceAccount(serviceAccountKey))
	if err != nil {
		return nil, nil, err
	}

	defer rootClient.Close() //nolint:errcheck

	name := fmt.Sprintf("%x", md5.Sum([]byte(cacheKey.email+cacheKey.role)))

	// generate a new PGP key with long lifetime
	comment := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	suffix := access.ServiceAccountNameSuffix

	if cacheKey.role == string(role.InfraProvider) {
		suffix = access.InfraProviderServiceAccountNameSuffix
	}

	serviceAccountEmail := name + suffix

	key, err := pgp.GenerateKey(name, comment, serviceAccountEmail, auth.ServiceAccountMaxAllowedLifetime)
	if err != nil {
		return nil, nil, err
	}

	if cacheKey.role == string(role.InfraProvider) {
		name = access.InfraProviderServiceAccountPrefix + name
	}

	armoredPublicKey, err := key.ArmorPublic()
	if err != nil {
		return nil, nil, err
	}

	serviceAccounts, err := rootClient.Management().ListServiceAccounts(ctx)
	if err != nil {
		return nil, nil, err
	}

	if slices.IndexFunc(serviceAccounts, func(account *management.ListServiceAccountsResponse_ServiceAccount) bool {
		return account.Name == name
	}) != -1 {
		if err = rootClient.Management().DestroyServiceAccount(ctx, name); err != nil {
			return nil, nil, err
		}
	}

	// create service account with the generated key
	_, err = rootClient.Management().CreateServiceAccount(ctx, name, armoredPublicKey, cacheKey.role, cacheKey.role == "")
	if err != nil {
		return nil, nil, err
	}

	encodedKey, err := serviceaccount.Encode(name, key)
	if err != nil {
		return nil, nil, err
	}

	cli, err := client.New(endpoint, client.WithServiceAccount(encodedKey))
	if err != nil {
		return nil, nil, err
	}

	return cli, key, nil
}
