// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package k8sproxy_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/golang-jwt/jwt/v4"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"k8s.io/client-go/transport"

	"github.com/siderolabs/omni/internal/backend/k8sproxy"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

var mockClusterUUIDResolver = func(_ context.Context, clusterID resource.ID) (string, error) {
	upper := strings.ToUpper(clusterID)
	if upper == clusterID {
		return "", fmt.Errorf("invalid test cluster ID - does not contain lowercase: %s", clusterID)
	}

	return upper, nil
}

type mockClaims struct {
	ExpiresAt   *jwt.NumericDate `json:"exp,omitempty"`
	Cluster     string           `json:"cluster,omitempty"`
	ClusterUUID string           `json:"cluster_uuid,omitempty"`
	Subject     string           `json:"sub,omitempty"`
	Groups      []string         `json:"groups,omitempty"`
}

func (c *mockClaims) Valid() error {
	return nil
}

func TestAuthorize(t *testing.T) {
	reqCh := make(chan *http.Request, 1)

	coreHandler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		reqCh <- r.Clone(r.Context())
	})

	key1, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	key2, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	keyFunc := func(_ context.Context, keyID string) (any, error) {
		switch keyID {
		case "1":
			return &key1.PublicKey, nil
		case "2":
			return &key2.PublicKey, nil
		default:
			return nil, errors.New("unknown key")
		}
	}

	ts := httptest.NewServer(k8sproxy.AuthorizeRequest(coreHandler, keyFunc, mockClusterUUIDResolver))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)

	ctx = ctxzap.ToContext(ctx, logger)

	type testCase struct { //nolint:govet
		name string

		claims       mockClaims
		kid          string
		signingKey   *rsa.PrivateKey
		extraHeaders map[string]string

		expectedCode int

		expectedImpersonateUser   string
		expectedImpersonateGroups []string
		expectedCluster           string
	}

	testCases := []testCase{
		{
			name: "valid key - legacy with cluster name",
			claims: mockClaims{
				Cluster:   "cluster1",
				Subject:   "user1",
				Groups:    []string{"group1", "group2"},
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			kid:        "1",
			signingKey: key1,

			expectedCode: http.StatusOK,

			expectedImpersonateUser:   "user1",
			expectedImpersonateGroups: []string{"group1", "group2"},
			expectedCluster:           "cluster1",
		},
		{
			name: "valid key1",
			claims: mockClaims{
				Cluster:     "cluster1",
				ClusterUUID: "CLUSTER1",
				Subject:     "user1",
				Groups:      []string{"group1", "group2"},
				ExpiresAt:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			kid:        "1",
			signingKey: key1,

			expectedCode: http.StatusOK,

			expectedImpersonateUser:   "user1",
			expectedImpersonateGroups: []string{"group1", "group2"},
			expectedCluster:           "cluster1",
		},
		{
			name: "valid key2 + extra headers",
			claims: mockClaims{
				Cluster:     "cluster2",
				ClusterUUID: "CLUSTER2",
				Subject:     "user2",
				Groups:      []string{"group2"},
				ExpiresAt:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			kid:        "2",
			signingKey: key2,
			extraHeaders: map[string]string{
				transport.ImpersonateUserHeader:  "foo",
				transport.ImpersonateGroupHeader: "bar",
			},

			expectedCode: http.StatusOK,

			expectedImpersonateUser:   "user2",
			expectedImpersonateGroups: []string{"group2"},
			expectedCluster:           "cluster2",
		},
		{
			name: "cluster-uuid mismatch",
			claims: mockClaims{
				Cluster:     "cluster-1",
				ClusterUUID: "CLUSTER2",
				Subject:     "user2",
				Groups:      []string{"group2"},
				ExpiresAt:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			kid:        "2",
			signingKey: key1,

			expectedCode: http.StatusUnauthorized,
		},
		{
			name: "kid mismatch",
			claims: mockClaims{
				Cluster:     "cluster2",
				ClusterUUID: "CLUSTER2",
				Subject:     "user2",
				Groups:      []string{"group2"},
				ExpiresAt:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			kid:        "2",
			signingKey: key1,

			expectedCode: http.StatusUnauthorized,
		},
		{
			name: "wrong kid",
			claims: mockClaims{
				Cluster:     "cluster2",
				ClusterUUID: "CLUSTER2",
				Subject:     "user2",
				Groups:      []string{"group2"},
				ExpiresAt:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			kid:        "3",
			signingKey: key1,

			expectedCode: http.StatusUnauthorized,
		},
		{
			name: "malformed claims 1",
			claims: mockClaims{
				Subject:   "user2",
				Groups:    []string{"group2"},
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			kid:        "1",
			signingKey: key1,

			expectedCode: http.StatusUnauthorized,
		},
		{
			name: "malformed claims 2",
			claims: mockClaims{
				Cluster:     "cluster2",
				ClusterUUID: "CLUSTER2",
				Groups:      []string{"group2"},
				ExpiresAt:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			kid:        "1",
			signingKey: key1,

			expectedCode: http.StatusUnauthorized,
		},
		{
			name: "malformed claims 2",
			claims: mockClaims{
				Cluster:     "cluster2",
				ClusterUUID: "CLUSTER2",
				Subject:     "foo",
				ExpiresAt:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			kid:        "1",
			signingKey: key1,

			expectedCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
			require.NoError(t, err)

			token := jwt.NewWithClaims(jwt.SigningMethodRS256, &tc.claims)
			token.Header["kid"] = tc.kid

			auth, err := token.SignedString(tc.signingKey)
			require.NoError(t, err)

			req.Header["Authorization"] = []string{"Bearer " + auth}

			for k, v := range tc.extraHeaders {
				req.Header.Set(k, v)
			}

			resp, err := http.DefaultClient.Do(req) //nolint:bodyclose
			require.NoError(t, err)

			require.Equal(t, tc.expectedCode, resp.StatusCode)

			if tc.expectedCode != http.StatusOK {
				return
			}

			var receivedReq *http.Request

			select {
			case receivedReq = <-reqCh:
			case <-time.After(time.Second):
				t.Fatal("timeout")
			}

			assert.Equal(t, []string{tc.expectedImpersonateUser}, receivedReq.Header.Values(transport.ImpersonateUserHeader))
			assert.Equal(t, tc.expectedImpersonateGroups, receivedReq.Header.Values(transport.ImpersonateGroupHeader))
			assert.Nil(t, receivedReq.Header.Values("Authorization"))

			v, ok := ctxstore.Value[k8sproxy.ClusterContextKey](receivedReq.Context()) //nolint:contextcheck
			assert.True(t, ok)
			assert.Equal(t, tc.expectedCluster, v.ClusterName)
		})
	}
}
