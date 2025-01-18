// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/api/omni/management"
	resapi "github.com/siderolabs/omni/client/api/omni/resources"
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/dns"
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/runtime"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/backend/workloadproxy"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/interceptor"
)

type GrpcSuite struct {
	suite.Suite
	runtime      *omniruntime.Runtime
	server       *grpc.Server
	conn         *grpc.ClientConn
	imageFactory *imageFactoryMock
	state        state.State
	eg           errgroup.Group

	ctx        context.Context //nolint:containedctx
	ctxCancel  context.CancelFunc
	socketPath string
}

func (suite *GrpcSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	var err error

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	logger := zaptest.NewLogger(suite.T())
	clientFactory := talos.NewClientFactory(suite.state, logger)
	dnsService := dns.NewService(suite.state, logger)
	discoveryServiceClientMock := &discoveryClientMock{}

	suite.imageFactory = &imageFactoryMock{}

	suite.Require().NoError(suite.imageFactory.run())
	suite.imageFactory.serve(suite.ctx)

	suite.T().Cleanup(func() {
		suite.Require().NoError(suite.imageFactory.eg.Wait())
	})

	imageFactoryClient, err := imagefactory.NewClient(suite.state, suite.imageFactory.address)
	suite.Require().NoError(err)

	workloadProxyReconciler := workloadproxy.NewReconciler(logger, zap.InfoLevel)

	suite.runtime, err = omniruntime.New(clientFactory, dnsService, workloadProxyReconciler, nil,
		imageFactoryClient, nil, nil, nil, suite.state, nil, prometheus.NewRegistry(), discoveryServiceClientMock, nil, logger)
	suite.Require().NoError(err)
	runtime.Install(omniruntime.Name, suite.runtime)

	suite.startRuntime()

	authConfigInterceptor := interceptor.NewAuthConfig(false, logger.With(logging.Component("interceptor")))

	err = suite.newServer(
		imageFactoryClient,
		logger,
		grpc.ChainUnaryInterceptor(authConfigInterceptor.Unary()),
		grpc.ChainStreamInterceptor(authConfigInterceptor.Stream()),
	)
	suite.Require().NoError(err)
}

func (suite *GrpcSuite) startRuntime() {
	suite.runtime.Run(actor.MarkContextAsInternalActor(suite.ctx), &suite.eg)
}

func (suite *GrpcSuite) TestGetDenied() {
	client := resapi.NewResourceServiceClient(suite.conn)

	ctx := metadata.AppendToOutgoingContext(suite.ctx, "runtime", common.Runtime_Omni.String())
	_, err := client.Get(ctx, &resapi.GetRequest{
		Id:        "1",
		Type:      omni.ClusterMachineConfigType,
		Namespace: resources.DefaultNamespace,
	})
	suite.Require().Error(err)
	suite.Assert().Equal(codes.PermissionDenied, status.Code(err))
}

func (suite *GrpcSuite) TestCreateDenied() {
	client := resapi.NewResourceServiceClient(suite.conn)

	rawSpec, err := runtime.MarshalJSON(&specs.ClusterStatusSpec{})
	suite.Assert().NoError(err)

	md := &v1alpha1.Metadata{
		Id:        "1",
		Type:      omni.ClusterStatusType,
		Namespace: resources.DefaultNamespace,
		Version:   resource.VersionUndefined.String(),
		Phase:     "running",
	}

	ctx := metadata.AppendToOutgoingContext(suite.ctx, "runtime", common.Runtime_Omni.String())
	_, err = client.Create(ctx, &resapi.CreateRequest{
		Resource: &resapi.Resource{
			Metadata: md,
			Spec:     rawSpec,
		},
	})

	suite.Require().Error(err)
	suite.Assert().Equal(codes.PermissionDenied, status.Code(err))
}

func (suite *GrpcSuite) TestCrud() {
	client := resapi.NewResourceServiceClient(suite.conn)

	resourceSpec := &specs.ConfigPatchSpec{}

	err := resourceSpec.SetUncompressedData(bytes.TrimSpace([]byte(`
{
  "machine": {
    "env": {
      "bla": "bla"
    }
  }
}
`)))
	suite.Require().NoError(err)

	rawSpec, err := runtime.MarshalJSON(resourceSpec)
	suite.Assert().NoError(err)

	md := &v1alpha1.Metadata{
		Id:        "1",
		Type:      omni.ConfigPatchType,
		Namespace: resources.DefaultNamespace,
		Version:   resource.VersionUndefined.String(),
		Phase:     "running",
	}

	ctx := metadata.AppendToOutgoingContext(suite.ctx, "runtime", common.Runtime_Omni.String())
	_, err = client.Create(ctx, &resapi.CreateRequest{
		Resource: &resapi.Resource{
			Metadata: md,
			Spec:     rawSpec,
		},
	})

	suite.Require().NoError(err)

	metadata, err := resource.NewMetadataFromProto(md)
	suite.Assert().NoError(err)

	res, err := safe.StateGet[*omni.ConfigPatch](ctx, suite.state, metadata)
	suite.Assert().NoError(err)

	suite.Require().True(proto.Equal(res.TypedSpec().Value, resourceSpec))

	resourceSpec = &specs.ConfigPatchSpec{}

	err = resourceSpec.SetUncompressedData([]byte("machine: {}"))
	suite.Require().NoError(err)

	rawSpec, err = runtime.MarshalJSON(resourceSpec)
	suite.Assert().NoError(err)

	_, err = client.Update(ctx, &resapi.UpdateRequest{
		CurrentVersion: res.Metadata().Version().String(),
		Resource: &resapi.Resource{
			Metadata: md,
			Spec:     rawSpec,
		},
	})
	suite.Assert().NoError(err)

	res, err = safe.StateGet[*omni.ConfigPatch](ctx, suite.state, metadata)
	suite.Assert().NoError(err)

	suite.Require().True(proto.Equal(res.TypedSpec().Value, resourceSpec))

	buffer, bufferErr := resourceSpec.GetUncompressedData()
	suite.Require().NoError(bufferErr)

	defer buffer.Free()

	patchData := string(buffer.Data())

	err = retry.Constant(time.Second, retry.WithUnits(time.Millisecond*50)).RetryWithContext(ctx, func(ctx context.Context) error {
		items, e := client.List(ctx, &resapi.ListRequest{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ConfigPatchType,
		})

		if e != nil {
			return e
		}

		for _, item := range items.Items {
			var data struct {
				Spec struct {
					Data string `json:"data"`
				} `json:"spec"`
			}

			e = json.Unmarshal([]byte(item), &data)
			if e != nil {
				return e
			}

			if patchData != data.Spec.Data {
				return retry.ExpectedErrorf("%s != %s", patchData, data.Spec.Data)
			}
		}

		return nil
	})
	suite.Require().NoError(err)

	_, err = client.Delete(ctx, &resapi.DeleteRequest{
		Namespace: resources.DefaultNamespace,
		Type:      omni.ConfigPatchType,
		Id:        "1",
	})
	suite.Require().NoError(err)

	_, err = suite.state.Get(ctx, metadata)
	suite.Assert().True(state.IsNotFoundError(err))
}

func (suite *GrpcSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	if suite.server != nil {
		suite.server.Stop()
	}

	if suite.conn != nil {
		suite.Require().NoError(suite.conn.Close())
	}

	suite.Require().NoError(suite.eg.Wait())
}

func (suite *GrpcSuite) TestConfigValidation() {
	client := management.NewManagementServiceClient(suite.conn)

	_, err := client.ValidateConfig(suite.ctx, &management.ValidateConfigRequest{
		Config: `machine:
				network:
				  hostname: abcd`,
	})
	suite.Require().Error(err)
	suite.Require().Equalf(codes.InvalidArgument, status.Code(err), err.Error())
}

func (suite *GrpcSuite) newServer(imageFactoryClient *imagefactory.Client, logger *zap.Logger, opts ...grpc.ServerOption) error {
	var err error

	suite.socketPath = filepath.Join(suite.T().TempDir(), "socket")

	listener, err := net.Listen("unix", suite.socketPath)
	if err != nil {
		return err
	}

	grpcAddress := fmt.Sprintf("unix://%s", suite.socketPath)

	suite.server = grpc.NewServer(opts...)

	resapi.RegisterResourceServiceServer(suite.server, &grpcomni.ResourceServer{})
	management.RegisterManagementServiceServer(suite.server, grpcomni.NewManagementServer(
		suite.state,
		imageFactoryClient,
		logger,
	))

	go func() {
		for {
			err = suite.server.Serve(listener)
			if err == nil || errors.Is(err, grpc.ErrServerStopped) {
				break
			}
		}
	}()

	suite.conn, err = grpc.NewClient(grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	return nil
}

type discoveryClientMock struct{}

// AffiliateDelete implements the omni.DiscoveryClient interface.
func (d *discoveryClientMock) AffiliateDelete(context.Context, string, string) error {
	return nil
}

func TestGrpcSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(GrpcSuite))
}
