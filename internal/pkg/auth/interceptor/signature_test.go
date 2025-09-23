// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package interceptor_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/siderolabs/go-api-signature/pkg/message"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/interceptor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/test"
)

type testServer struct {
	resources.UnimplementedResourceServiceServer

	t *testing.T
}

func (s testServer) Get(context.Context, *resources.GetRequest) (*resources.GetResponse, error) {
	return &resources.GetResponse{}, nil
}

type SignatureTestSuite struct {
	testServiceClient resources.ResourceServiceClient
	clientConn        *grpc.ClientConn
	key               *pgp.Key
	test.GRPCSuite
}

func (suite *SignatureTestSuite) SetupSuite() {
	var err error

	suite.key, err = pgp.GenerateKey("", "", "test@example.org", time.Minute)
	suite.Require().NoError(err)

	authenticatorFunc := func(context.Context, string) (*auth.Authenticator, error) {
		return &auth.Authenticator{
			Verifier: suite.key,
			Identity: "user@example.com",
			UserID:   "user-id",
			Role:     role.Operator,
		}, nil
	}

	logger := zaptest.NewLogger(suite.T())

	authConfigInterceptor := interceptor.NewAuthConfig(true, logger)

	signatureInterceptor := interceptor.NewSignature(authenticatorFunc, logger)

	suite.InitServer(
		grpc.ChainUnaryInterceptor(
			authConfigInterceptor.Unary(),
			signatureInterceptor.Unary(),
		),
		grpc.ChainStreamInterceptor(
			authConfigInterceptor.Stream(),
			signatureInterceptor.Stream(),
		),
	)

	resources.RegisterResourceServiceServer(suite.Server, testServer{
		t: suite.T(),
	})

	suite.StartServer()

	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	suite.clientConn, err = grpc.NewClient(suite.Target, dialOptions...)
	suite.Require().NoError(err)

	suite.testServiceClient = resources.NewResourceServiceClient(suite.clientConn)
}

func (suite *SignatureTestSuite) TearDownSuite() {
	suite.clientConn.Close() //nolint:errcheck
	suite.StopServer()
}

func (suite *SignatureTestSuite) TestMissingSignaturePassthrough() {
	_, err := suite.testServiceClient.Get(suite.T().Context(), &resources.GetRequest{
		Type: authres.AuthConfigType,
	})

	suite.Assert().NoError(err)
}

func (suite *SignatureTestSuite) TestInvalidSignatureVersion() {
	ctx := metadata.NewOutgoingContext(suite.T().Context(), metadata.Pairs(
		message.SignatureHeaderKey, "invalid",
	))

	_, err := suite.testServiceClient.Get(ctx, &resources.GetRequest{})

	suite.Assert().Error(err)
	suite.Assert().Equal(codes.Unauthenticated, status.Code(err), "error code should be codes.Unauthenticated")
	suite.Assert().ErrorContains(err, "invalid signature")
}

func (suite *SignatureTestSuite) TestMissingTimestamp() {
	payload := base64.StdEncoding.EncodeToString([]byte("payload"))

	ctx := metadata.NewOutgoingContext(suite.T().Context(), metadata.Pairs(
		message.SignatureHeaderKey, fmt.Sprintf("%s test@example.org signer-1 %s", message.SignatureVersionV1, payload),
	))

	_, err := suite.testServiceClient.Get(ctx, &resources.GetRequest{})

	suite.Assert().Error(err)
	suite.Assert().Equal(codes.Unauthenticated, status.Code(err), "error code should be codes.Unauthenticated")
	suite.Assert().ErrorContains(err, "invalid signature")
}

func (suite *SignatureTestSuite) TestValidSignature() {
	epochTimestamp := strconv.FormatInt(time.Now().Unix(), 10)

	payload := message.GRPCPayload{
		Headers: map[string][]string{
			message.TimestampHeaderKey: {epochTimestamp},
		},
		Method: resources.ResourceService_Get_FullMethodName,
	}

	payloadJSON, err := json.Marshal(payload)
	suite.Require().NoError(err)

	signature, err := suite.key.Sign(payloadJSON)
	suite.Require().NoError(err)

	signatureBase64 := base64.StdEncoding.EncodeToString(signature)

	ctx := metadata.NewOutgoingContext(suite.T().Context(), metadata.Pairs(
		message.SignatureHeaderKey, fmt.Sprintf(
			"%s test@example.org %s %s",
			message.SignatureVersionV1,
			suite.key.Fingerprint(),
			signatureBase64,
		),
		message.TimestampHeaderKey, epochTimestamp,
		message.PayloadHeaderKey, string(payloadJSON),
	))

	_, err = suite.testServiceClient.Get(ctx, &resources.GetRequest{})

	assert.NoError(suite.T(), err)
}

func TestSignatureTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SignatureTestSuite))
}
