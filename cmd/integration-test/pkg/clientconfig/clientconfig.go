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
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/containers"
	authpb "github.com/siderolabs/go-api-signature/api/auth"
	authcli "github.com/siderolabs/go-api-signature/pkg/client/auth"
	"github.com/siderolabs/go-api-signature/pkg/client/interceptor"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"github.com/siderolabs/go-api-signature/pkg/serviceaccount"
	"google.golang.org/grpc"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/internal/pkg/auth"
)

const (
	defaultEmail = "test-user@siderolabs.com"
)

type clientCacheKey struct {
	role         string
	email        string
	skipUserRole bool
}

type clientOrError struct {
	client *client.Client
	err    error
}

var clientCache = containers.ConcurrentMap[clientCacheKey, clientOrError]{}

// ClientConfig is a test client.
type ClientConfig struct {
	endpoint string
}

// New creates a new test client config.
func New(endpoint string) *ClientConfig {
	return &ClientConfig{
		endpoint: endpoint,
	}
}

// GetClient returns a test client for the default test email.
//
// Clients are cached by their configuration, so if a client with the
// given configuration was created before, the cached one will be returned.
func (t *ClientConfig) GetClient(ctx context.Context, publicKeyOpts ...authcli.RegisterPGPPublicKeyOption) (*client.Client, error) {
	return t.GetClientForEmail(ctx, defaultEmail, publicKeyOpts...)
}

// GetClientForEmail returns a test client for the given email.
//
// Clients are cached by their configuration, so if a client with the
// given configuration was created before, the cached one will be returned.
func (t *ClientConfig) GetClientForEmail(ctx context.Context, email string, publicKeyOpts ...authcli.RegisterPGPPublicKeyOption) (*client.Client, error) {
	cacheKey := t.buildCacheKey(email, publicKeyOpts)

	cliOrErr, _ := clientCache.GetOrCall(cacheKey, func() clientOrError {
		if !constants.IsDebugBuild {
			cli, err := createServiceAccountClient(ctx, t.endpoint, cacheKey)

			return clientOrError{
				client: cli,
				err:    err,
			}
		}

		signatureInterceptor := buildSignatureInterceptor(email, publicKeyOpts...)

		cli, err := client.New(t.endpoint,
			client.WithGrpcOpts(
				grpc.WithUnaryInterceptor(signatureInterceptor.Unary()),
				grpc.WithStreamInterceptor(signatureInterceptor.Stream()),
			),
		)

		return clientOrError{
			client: cli,
			err:    err,
		}
	})

	return cliOrErr.client, cliOrErr.err
}

// Close closes all the clients created by this config.
func (t *ClientConfig) Close() error {
	var multiErr error

	clientCache.ForEach(func(_ clientCacheKey, cliOrErr clientOrError) {
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

var talosAPIKeyMutex sync.Mutex

// TalosAPIKeyPrepare prepares a public key to be used with tests interacting via Talos API client using the default test email.
func TalosAPIKeyPrepare(ctx context.Context, client *client.Client, contextName string) error {
	return TalosAPIKeyPrepareWithEmail(ctx, client, contextName, defaultEmail)
}

// TalosAPIKeyPrepareWithEmail prepares a public key to be used with tests interacting via Talos API client using the given email.
func TalosAPIKeyPrepareWithEmail(ctx context.Context, client *client.Client, contextName, email string) error {
	talosAPIKeyMutex.Lock()
	defer talosAPIKeyMutex.Unlock()

	path, err := xdg.DataFile(filepath.Join("talos", "keys", fmt.Sprintf("%s-%s.pgp", contextName, email)))
	if err != nil {
		return err
	}

	stat, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if stat != nil && time.Since(stat.ModTime()) < 2*time.Hour {
		return nil
	}

	newKey, err := pgp.GenerateKey("", "", email, 4*time.Hour)
	if err != nil {
		return err
	}

	err = registerKey(ctx, client.Auth(), newKey, email)
	if err != nil {
		return err
	}

	keyArmored, err := newKey.Armor()
	if err != nil {
		return err
	}

	return os.WriteFile(path, []byte(keyArmored), 0o600)
}

func buildSignatureInterceptor(email string, publicKeyOpts ...authcli.RegisterPGPPublicKeyOption) *interceptor.Interceptor {
	userKeyFunc := func(ctx context.Context, cc *grpc.ClientConn, _ *interceptor.Options) (message.Signer, error) {
		newKey, err := pgp.GenerateKey("", "", email, 4*time.Hour)
		if err != nil {
			return nil, err
		}

		authCli := authcli.NewClient(cc)

		err = registerKey(ctx, authCli, newKey, email, publicKeyOpts...)
		if err != nil {
			return nil, err
		}

		return newKey, nil
	}

	return interceptor.New(interceptor.Options{
		GetUserKeyFunc:   userKeyFunc,
		RenewUserKeyFunc: userKeyFunc,
		Identity:         email,
	})
}

func createServiceAccountClient(ctx context.Context, endpoint string, cacheKey clientCacheKey) (*client.Client, error) {
	serviceAccount := os.Getenv("OMNI_SERVICE_ACCOUNT_KEY")
	if serviceAccount == "" {
		return nil, fmt.Errorf("OMNI_SERVICE_ACCOUNT_KEY environment variable is not set")
	}

	rootClient, err := client.New(endpoint, client.WithServiceAccount(serviceAccount))
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("%x", md5.Sum([]byte(cacheKey.email+cacheKey.role)))

	// generate a new PGP key with long lifetime
	comment := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	serviceAccountEmail := name + access.ServiceAccountNameSuffix

	key, err := pgp.GenerateKey(name, comment, serviceAccountEmail, auth.ServiceAccountMaxAllowedLifetime)
	if err != nil {
		return nil, err
	}

	armoredPublicKey, err := key.ArmorPublic()
	if err != nil {
		return nil, err
	}

	serviceAccounts, err := rootClient.Management().ListServiceAccounts(ctx)
	if err != nil {
		return nil, err
	}

	if slices.IndexFunc(serviceAccounts, func(account *management.ListServiceAccountsResponse_ServiceAccount) bool {
		return account.Name == name
	}) != -1 {
		if err = rootClient.Management().DestroyServiceAccount(ctx, name); err != nil {
			return nil, err
		}
	}

	// create service account with the generated key
	_, err = rootClient.Management().CreateServiceAccount(ctx, name, armoredPublicKey, cacheKey.role, cacheKey.role == "")
	if err != nil {
		return nil, err
	}

	encodedKey, err := serviceaccount.Encode(name, key)
	if err != nil {
		return nil, err
	}

	return client.New(endpoint, client.WithServiceAccount(encodedKey))
}
