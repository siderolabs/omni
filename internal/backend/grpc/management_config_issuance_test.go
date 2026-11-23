// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/go-jose/go-jose/v4"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/op"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/access/role"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

const (
	configIssuanceTestCluster  = "cluster-1"
	configIssuanceTestIdentity = "user@example.com"
)

// configIssuanceTestContext returns the context of a request for the test cluster, made by a user with the given role
// and no access policy rule for the cluster.
func configIssuanceTestContext(ctx context.Context, r role.Role) context.Context {
	ctx = managementPowerTestContext(ctx, configIssuanceTestIdentity, r)

	return metadata.NewIncomingContext(ctx, metadata.Pairs(message.ClusterHeaderKey, configIssuanceTestCluster))
}

// configIssuanceTestState returns a state with the test cluster in it, as far as the config requests need it.
func configIssuanceTestState(t *testing.T) state.State {
	st := newManagementPowerTestState(t, "other-cluster", configIssuanceTestIdentity, "machine-1")
	ctx := actor.MarkContextAsInternalActor(t.Context())

	clusterUUID := omnires.NewClusterUUID(configIssuanceTestCluster)
	clusterUUID.TypedSpec().Value.Uuid = "00000000-0000-0000-0000-000000000001"

	require.NoError(t, st.Create(ctx, clusterUUID))

	// a break-glass config taints the cluster in its status
	require.NoError(t, st.Create(ctx, omnires.NewClusterStatus(configIssuanceTestCluster), state.WithCreateOwner(omnictrl.ClusterStatusControllerName)))

	return st
}

// testSigningKeyProvider provides a fixed symmetric key to sign the service account tokens with.
type testSigningKeyProvider struct{}

func (testSigningKeyProvider) SigningKey(context.Context) (op.SigningKey, error) {
	return testSigningKey{}, nil
}

type testSigningKey struct{}

func (testSigningKey) SignatureAlgorithm() jose.SignatureAlgorithm { return jose.HS256 }

func (testSigningKey) Key() any { return []byte("test signing key") }

func (testSigningKey) ID() string { return "test-key" }

// TestKubeconfigIssuanceIsAudited checks that every kind of kubeconfig which is issued is recorded, and that it is not
// issued when the audit entry cannot be written.
func TestKubeconfigIssuanceIsAudited(t *testing.T) {
	for _, test := range []struct {
		request  *management.KubeconfigRequest
		name     string
		expected auditlog.KubeconfigIssuance
		// tokenTTL is the lifetime of the token in the kubeconfig, when it has one
		tokenTTL time.Duration
	}{
		{
			name:    "OIDC",
			request: &management.KubeconfigRequest{},
			expected: auditlog.KubeconfigIssuance{
				ClusterName: configIssuanceTestCluster,
				Type:        auditlog.KubeconfigTypeOIDC,
			},
		},
		{
			name: "service account",
			request: &management.KubeconfigRequest{
				ServiceAccount:       true,
				ServiceAccountUser:   "robot",
				ServiceAccountGroups: []string{"system:masters"},
				ServiceAccountTtl:    durationpb.New(time.Hour),
			},
			expected: auditlog.KubeconfigIssuance{
				ClusterName:          configIssuanceTestCluster,
				Type:                 auditlog.KubeconfigTypeServiceAccount,
				ServiceAccountUser:   "robot",
				ServiceAccountGroups: []string{"system:masters"},
				SigningKeyID:         "test-key",
			},
			tokenTTL: time.Hour,
		},
		{
			name:    "break-glass",
			request: &management.KubeconfigRequest{BreakGlass: true},
			expected: auditlog.KubeconfigIssuance{
				ClusterName: configIssuanceTestCluster,
				Type:        auditlog.KubeconfigTypeBreakGlass,
			},
		},
	} {
		for _, auditErr := range []error{nil, errors.New("audit write failure")} {
			name := test.name
			if auditErr != nil {
				name += ", audit write failure"
			}

			t.Run(name, func(t *testing.T) {
				auditor := &capturingAuditLogger{err: auditErr}

				server := grpcomni.NewManagementServer(configIssuanceTestState(t), nil, zaptest.NewLogger(t), true, noBackend{}, noBackend{},
					grpcomni.WithAuditLogger(auditor), grpcomni.WithJWTSigningKeyProvider(testSigningKeyProvider{}))

				start := time.Now()

				resp, err := server.Kubeconfig(configIssuanceTestContext(t.Context(), role.Admin), test.request)

				if auditErr != nil {
					require.ErrorIs(t, err, auditErr)
					require.Nil(t, resp)
				} else {
					require.NoError(t, err)
					require.NotEmpty(t, resp.GetKubeconfig())
				}

				require.Len(t, auditor.kubeconfigIssuances, 1)

				// the expiration depends on the time of the request, so it is checked apart from the rest of the entry
				issuance := auditor.kubeconfigIssuances[0]

				if test.tokenTTL == 0 {
					require.Empty(t, issuance.Expiration)
				} else {
					expiration, parseErr := time.Parse(time.RFC3339, issuance.Expiration)
					require.NoError(t, parseErr)
					require.WithinRange(t, expiration, start.Add(test.tokenTTL).Truncate(time.Second), time.Now().Add(test.tokenTTL))
				}

				issuance.Expiration = ""

				require.Equal(t, test.expected, issuance)
				require.Empty(t, auditor.talosconfigIssuances)
			})
		}
	}
}

// TestTalosconfigIssuanceIsAudited checks that every kind of talosconfig which is issued is recorded, and that it is not
// issued when the audit entry cannot be written.
func TestTalosconfigIssuanceIsAudited(t *testing.T) {
	for _, test := range []struct {
		request  *management.TalosconfigRequest
		name     string
		expected auditlog.TalosconfigIssuance
	}{
		{
			name:    "Omni",
			request: &management.TalosconfigRequest{},
			expected: auditlog.TalosconfigIssuance{
				ClusterName: configIssuanceTestCluster,
				Type:        auditlog.TalosconfigTypeOmni,
			},
		},
		{
			name:    "break-glass",
			request: &management.TalosconfigRequest{BreakGlass: true},
			expected: auditlog.TalosconfigIssuance{
				ClusterName: configIssuanceTestCluster,
				Type:        auditlog.TalosconfigTypeBreakGlass,
			},
		},
	} {
		for _, auditErr := range []error{nil, errors.New("audit write failure")} {
			name := test.name
			if auditErr != nil {
				name += ", audit write failure"
			}

			t.Run(name, func(t *testing.T) {
				auditor := &capturingAuditLogger{err: auditErr}

				server := grpcomni.NewManagementServer(configIssuanceTestState(t), nil, zaptest.NewLogger(t), true, noBackend{}, noBackend{}, grpcomni.WithAuditLogger(auditor))

				resp, err := server.Talosconfig(configIssuanceTestContext(t.Context(), role.Admin), test.request)

				if auditErr != nil {
					require.ErrorIs(t, err, auditErr)
					require.Nil(t, resp)
				} else {
					require.NoError(t, err)
					require.NotEmpty(t, resp.GetTalosconfig())
				}

				require.Equal(t, []auditlog.TalosconfigIssuance{test.expected}, auditor.talosconfigIssuances)
				require.Empty(t, auditor.kubeconfigIssuances)
			})
		}
	}
}

// TestDeniedKubeconfigRequestIsAudited checks that a refused kubeconfig request is recorded as denied, and that the caller
// gets the refusal even when the audit entry cannot be written.
func TestDeniedKubeconfigRequestIsAudited(t *testing.T) {
	for _, test := range []struct {
		request  *management.KubeconfigRequest
		auditErr error
		name     string
		role     role.Role
		expected auditlog.KubeconfigIssuance
	}{
		{
			name:    "break-glass without the permission",
			role:    role.Reader,
			request: &management.KubeconfigRequest{BreakGlass: true},
			expected: auditlog.KubeconfigIssuance{
				ClusterName: configIssuanceTestCluster,
				Type:        auditlog.KubeconfigTypeBreakGlass,
				Denied:      true,
			},
		},
		{
			name:    "break-glass disabled",
			role:    role.Admin,
			request: &management.KubeconfigRequest{BreakGlass: true},
			expected: auditlog.KubeconfigIssuance{
				ClusterName: configIssuanceTestCluster,
				Type:        auditlog.KubeconfigTypeBreakGlass,
				Denied:      true,
			},
		},
		{
			name: "service account without access to the cluster",
			role: role.Reader,
			request: &management.KubeconfigRequest{
				ServiceAccount:       true,
				ServiceAccountUser:   "robot",
				ServiceAccountGroups: []string{"system:masters"},
				ServiceAccountTtl:    durationpb.New(time.Hour),
			},
			expected: auditlog.KubeconfigIssuance{
				ClusterName:          configIssuanceTestCluster,
				Type:                 auditlog.KubeconfigTypeServiceAccount,
				ServiceAccountUser:   "robot",
				ServiceAccountGroups: []string{"system:masters"},
				Denied:               true,
			},
		},
		{
			name:     "audit write failure",
			role:     role.Reader,
			request:  &management.KubeconfigRequest{BreakGlass: true},
			auditErr: errors.New("audit write failure"),
			expected: auditlog.KubeconfigIssuance{
				ClusterName: configIssuanceTestCluster,
				Type:        auditlog.KubeconfigTypeBreakGlass,
				Denied:      true,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			st := newManagementPowerTestState(t, "other-cluster", configIssuanceTestIdentity, "machine-1")
			auditor := &capturingAuditLogger{err: test.auditErr}

			// break-glass configs are disabled
			server := grpcomni.NewManagementServer(st, nil, zaptest.NewLogger(t), false, nil, nil, grpcomni.WithAuditLogger(auditor))

			_, err := server.Kubeconfig(configIssuanceTestContext(t.Context(), test.role), test.request)
			require.Equal(t, codes.PermissionDenied, status.Code(err), "unexpected error: %v", err)

			require.Equal(t, []auditlog.KubeconfigIssuance{test.expected}, auditor.kubeconfigIssuances)
			require.Empty(t, auditor.talosconfigIssuances)
		})
	}
}

// TestDeniedTalosconfigRequestIsAudited checks that a refused talosconfig request is recorded as denied, and that the
// caller gets the refusal even when the audit entry cannot be written.
func TestDeniedTalosconfigRequestIsAudited(t *testing.T) {
	for _, test := range []struct {
		request  *management.TalosconfigRequest
		auditErr error
		name     string
		expected auditlog.TalosconfigIssuance
	}{
		{
			name:    "break-glass without the permission",
			request: &management.TalosconfigRequest{BreakGlass: true},
			expected: auditlog.TalosconfigIssuance{
				ClusterName: configIssuanceTestCluster,
				Type:        auditlog.TalosconfigTypeBreakGlass,
				Denied:      true,
			},
		},
		{
			name:    "raw without the permission",
			request: &management.TalosconfigRequest{Raw: true},
			expected: auditlog.TalosconfigIssuance{
				ClusterName: configIssuanceTestCluster,
				Type:        auditlog.TalosconfigTypeRaw,
				Denied:      true,
			},
		},
		{
			name:     "audit write failure",
			request:  &management.TalosconfigRequest{BreakGlass: true},
			auditErr: errors.New("audit write failure"),
			expected: auditlog.TalosconfigIssuance{
				ClusterName: configIssuanceTestCluster,
				Type:        auditlog.TalosconfigTypeBreakGlass,
				Denied:      true,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			st := newManagementPowerTestState(t, "other-cluster", configIssuanceTestIdentity, "machine-1")
			auditor := &capturingAuditLogger{err: test.auditErr}

			server := grpcomni.NewManagementServer(st, nil, zaptest.NewLogger(t), true, nil, nil, grpcomni.WithAuditLogger(auditor))

			_, err := server.Talosconfig(configIssuanceTestContext(t.Context(), role.Reader), test.request)
			require.Equal(t, codes.PermissionDenied, status.Code(err), "unexpected error: %v", err)

			require.Equal(t, []auditlog.TalosconfigIssuance{test.expected}, auditor.talosconfigIssuances)
			require.Empty(t, auditor.kubeconfigIssuances)
		})
	}
}

// TestInvalidConfigRequestIsNotAuditedAsDenied checks that a request refused for a reason other than permissions leaves no
// denied entry.
func TestInvalidConfigRequestIsNotAuditedAsDenied(t *testing.T) {
	st := newManagementPowerTestState(t, "other-cluster", configIssuanceTestIdentity, "machine-1")
	auditor := &capturingAuditLogger{}

	server := grpcomni.NewManagementServer(st, nil, zaptest.NewLogger(t), true, nil, nil, grpcomni.WithAuditLogger(auditor))

	// an admin passes the permission checks, and the request has no cluster
	ctx := managementPowerTestContext(t.Context(), configIssuanceTestIdentity, role.Admin)

	_, err := server.Kubeconfig(ctx, &management.KubeconfigRequest{BreakGlass: true})
	require.Equal(t, codes.InvalidArgument, status.Code(err), "unexpected error: %v", err)

	require.Empty(t, auditor.kubeconfigIssuances)
	require.Empty(t, auditor.talosconfigIssuances)
}
