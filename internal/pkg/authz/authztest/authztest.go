// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package authztest provides the callers which the golden tests of the access checks run as.
package authztest

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-api-signature/pkg/plain"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/access/role"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/accesspolicy"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/authenticator"
)

// The clusters, the machines and the infra providers which the subjects and the tests refer to.
const (
	ClusterA = "cluster-a"
	ClusterB = "cluster-b"
	// MemberMachine is a machine in ClusterA.
	MemberMachine = "machine-a"
	// OtherMachine is a machine in ClusterB.
	OtherMachine = "machine-b"
	// FreeMachine is a machine in no cluster.
	FreeMachine = "machine-free"
	// OwnProvider is the infra provider of the infra-provider subject.
	OwnProvider   = "own"
	OtherProvider = "other"
)

// Subject is a caller, set up as data the way an administrator sets one up today.
type Subject struct {
	labels map[string]string
	Name   string
	// Description says how the subject is set up, for the header of its golden file.
	Description string
	role        role.Role
	suspended   bool
}

// signedIn reports whether the subject has a user. The anonymous subject has none.
func (s Subject) signedIn() bool {
	return s.role != ""
}

// identity returns the ID of the identity of the subject.
func (s Subject) identity() string {
	if s.role == role.InfraProvider {
		return OwnProvider + access.InfraProviderServiceAccountNameSuffix
	}

	return s.Name + "@example.org"
}

// Subjects are the callers which the access checks are tested with.
var Subjects = []Subject{
	{Name: "anonymous", Description: "It is a request without a signature."},
	{Name: "none", role: role.None, Description: "It is a user with the role None."},
	{Name: "reader", role: role.Reader, Description: "It is a user with the role Reader."},
	{Name: "auditor", role: role.Auditor, Description: "It is a user with the role Auditor."},
	{Name: "operator", role: role.Operator, Description: "It is a user with the role Operator."},
	{Name: "admin", role: role.Admin, Description: "It is a user with the role Admin."},
	{Name: "infra-provider", role: role.InfraProvider, Description: `It is the service account of the infra provider "own".`},
	{
		Name: "acl-reader-a", role: role.None,
		Description: "It is a user with the role None, who gets Reader on cluster-a through the access policy. The user and the cluster are matched by name.",
	},
	{
		Name: "acl-operator-a", role: role.Reader, labels: map[string]string{"team": "a"},
		Description: "It is a user with the role Reader, who gets Operator on the clusters matching *-a through the access policy. The user is matched by the identity label team=a.",
	},
	{
		Name: "acl-operator-all", role: role.None,
		Description: "It is a user with the role None, who gets Operator on every cluster through the access policy. The user is matched by a pattern.",
	},
	{Name: "suspended-admin", role: role.Admin, suspended: true, Description: "It is a user with the role Admin on a suspended instance."},
	{
		Name: "suspended-acl-operator-a", role: role.Reader, labels: map[string]string{"team": "a"}, suspended: true,
		Description: "It is acl-operator-a on a suspended instance.",
	},
}

// accessPolicy returns the access policy of the subjects.
func accessPolicy() *authres.AccessPolicy {
	policy := authres.NewAccessPolicy()
	policy.TypedSpec().Value = &specs.AccessPolicySpec{
		UserGroups: map[string]*specs.AccessPolicyUserGroup{
			"team-a":    {Users: []*specs.AccessPolicyUserGroup_User{{LabelSelectors: []string{"team=a"}}}},
			"operators": {Users: []*specs.AccessPolicyUserGroup_User{{Match: "acl-operator-all@*"}}},
		},
		ClusterGroups: map[string]*specs.AccessPolicyClusterGroup{
			"a-clusters": {Clusters: []*specs.AccessPolicyClusterGroup_Cluster{{Match: "*-a"}}},
			"all":        {Clusters: []*specs.AccessPolicyClusterGroup_Cluster{{Match: "*"}}},
		},
		Rules: []*specs.AccessPolicyRule{
			{Users: []string{"acl-reader-a@example.org"}, Clusters: []string{ClusterA}, Role: string(role.Reader)},
			{Users: []string{"group/team-a"}, Clusters: []string{"group/a-clusters"}, Role: string(role.Operator)},
			{Users: []string{"group/operators"}, Clusters: []string{"group/all"}, Role: string(role.Operator)},
		},
	}

	return policy
}

// Keys are the public keys of the subjects, using the name of the subject as the key.
type Keys map[string]key

type key struct {
	fingerprint string
	pem         []byte
}

// GenerateKeys creates a key for each subject which is signed in.
func GenerateKeys(t *testing.T) Keys {
	keys := Keys{}

	for _, s := range Subjects {
		if !s.signedIn() {
			continue
		}

		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
		require.NoError(t, err)

		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})

		parsed, err := plain.ParseKey(keyPEM)
		require.NoError(t, err)

		keys[s.Name] = key{pem: keyPEM, fingerprint: parsed.ID()}
	}

	return keys
}

// Seed creates the subjects in the state: their users, identities and public keys, the access policy,
// the infra providers, and the machines of ClusterA and ClusterB.
func Seed(t *testing.T, st state.State, keys Keys) {
	ctx := actor.MarkContextAsInternalActor(t.Context())

	policy := accessPolicy()
	require.NoError(t, accesspolicy.Validate(policy))

	fixtures := []resource.Resource{policy, infra.NewProvider(OwnProvider), infra.NewProvider(OtherProvider)}

	for machine, cluster := range map[string]string{MemberMachine: ClusterA, OtherMachine: ClusterB} {
		clusterMachine := omni.NewClusterMachine(machine)
		clusterMachine.Metadata().Labels().Set(omni.LabelCluster, cluster)

		fixtures = append(fixtures, clusterMachine)
	}

	for _, s := range Subjects {
		if !s.signedIn() {
			continue
		}

		user := authres.NewUser(s.Name)
		user.TypedSpec().Value.Role = string(s.role)

		identity := authres.NewIdentity(s.identity())
		identity.TypedSpec().Value.UserId = s.Name

		for k, v := range s.labels {
			identity.Metadata().Labels().Set(k, v)
		}

		publicKey := authres.NewPublicKey(keys[s.Name].fingerprint)
		publicKey.Metadata().Labels().Set(authres.LabelPublicKeyUserID, s.Name)
		publicKey.TypedSpec().Value.PublicKey = keys[s.Name].pem
		publicKey.TypedSpec().Value.Type = specs.PublicKeySpec_PLAIN
		publicKey.TypedSpec().Value.Confirmed = true //nolint:staticcheck
		publicKey.TypedSpec().Value.Expiration = timestamppb.New(time.Now().Add(time.Hour))
		publicKey.TypedSpec().Value.Identity = &specs.Identity{Email: s.identity()}

		fixtures = append(fixtures, user, identity, publicKey)
	}

	for _, res := range fixtures {
		require.NoError(t, st.Create(ctx, res))
	}
}

// SignIn returns the context of a request of the subject, signed in through the production authenticator.
func SignIn(t *testing.T, st state.State, keys Keys, s Subject) context.Context {
	if !s.signedIn() {
		return t.Context()
	}

	fingerprint := keys[s.Name].fingerprint

	authn, err := authenticator.New(st, s.suspended)(t.Context(), fingerprint)
	require.NoError(t, err)

	return auth.ContextWithAuthenticator(t.Context(), authn, fingerprint)
}
