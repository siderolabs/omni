// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package authz provides the integration tests of the access checks in Omni.
package authz

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"sync"

	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"github.com/siderolabs/go-api-signature/pkg/serviceaccount"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/internal/pkg/auth"
)

// ClientFactory creates clients authenticated as service accounts with a given role, one per role.
type ClientFactory struct {
	rootCli  *client.Client
	clients  map[role.Role]*client.Client
	endpoint string
	mu       sync.Mutex
}

// NewClientFactory creates a ClientFactory, which creates the service accounts with the root client.
func NewClientFactory(endpoint string, rootCli *client.Client) *ClientFactory {
	return &ClientFactory{
		endpoint: endpoint,
		rootCli:  rootCli,
		clients:  make(map[role.Role]*client.Client),
	}
}

// Get returns the client for the role, creating its service account on the first call.
func (f *ClientFactory) Get(ctx context.Context, r role.Role) (*client.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if cli, ok := f.clients[r]; ok {
		return cli, nil
	}

	cli, err := f.createClientForRole(ctx, r)
	if err != nil {
		return nil, err
	}

	f.clients[r] = cli

	return cli, nil
}

func (f *ClientFactory) createClientForRole(ctx context.Context, r role.Role) (*client.Client, error) {
	name := fmt.Sprintf("%x", md5.Sum([]byte(string(r))))

	comment := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	suffix := access.ServiceAccountNameSuffix
	if r == role.InfraProvider {
		suffix = access.InfraProviderServiceAccountNameSuffix
	}

	serviceAccountEmail := name + suffix

	key, err := pgp.GenerateKey(name, comment, serviceAccountEmail, auth.ServiceAccountMaxAllowedLifetime)
	if err != nil {
		return nil, err
	}

	if r == role.InfraProvider {
		name = access.InfraProviderServiceAccountPrefix + name
	}

	armoredPublicKey, err := key.ArmorPublic()
	if err != nil {
		return nil, err
	}

	serviceAccounts, err := f.rootCli.Management().ListServiceAccounts(ctx)
	if err != nil {
		return nil, err
	}

	if slices.IndexFunc(serviceAccounts, func(account *management.ListServiceAccountsResponse_ServiceAccount) bool {
		return account.Name == name
	}) != -1 {
		if err = f.rootCli.Management().DestroyServiceAccount(ctx, name); err != nil {
			return nil, err
		}
	}

	_, err = f.rootCli.Management().CreateServiceAccount(ctx, name, armoredPublicKey, string(r), false)
	if err != nil {
		return nil, err
	}

	encodedKey, err := serviceaccount.Encode(name, key)
	if err != nil {
		return nil, err
	}

	return client.New(f.endpoint, client.WithServiceAccount(encodedKey))
}

// Close closes all clients.
func (f *ClientFactory) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var errs []error

	for _, cli := range f.clients {
		if err := cli.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
