// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-retry/retry"
	pb "github.com/siderolabs/siderolink/api/siderolink"
	"github.com/siderolabs/siderolink/pkg/wireguard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/errgroup"
	"github.com/siderolabs/omni/internal/pkg/grpcutil"
	"github.com/siderolabs/omni/internal/pkg/machineevent"
	sideromanager "github.com/siderolabs/omni/internal/pkg/siderolink"
)

type fakeWireguardHandler struct {
	logger   *zap.Logger
	loggerMu sync.Mutex
}

func (h *fakeWireguardHandler) SetupDevice(wireguard.DeviceConfig) error {
	return nil
}

func (h *fakeWireguardHandler) Run(ctx context.Context, logger *zap.Logger) error {
	h.loggerMu.Lock()

	unlock := sync.OnceFunc(h.loggerMu.Unlock)

	defer unlock()

	h.logger = logger

	unlock()
	<-ctx.Done()

	return nil
}

func (h *fakeWireguardHandler) Shutdown() error {
	return nil
}

func (h *fakeWireguardHandler) PeerEvent(_ context.Context, spec *specs.SiderolinkSpec, deleted bool) error {
	h.loggerMu.Lock()
	defer h.loggerMu.Unlock()

	msg := "updated peer"
	if deleted {
		msg = "removed peer"
	}

	h.logger.Info(msg, zap.String("public_key", spec.NodePublicKey), zap.String("address", spec.NodeSubnet))

	return nil
}

func (h *fakeWireguardHandler) Peers() ([]wgtypes.Peer, error) {
	return []wgtypes.Peer{}, nil
}

type SiderolinkSuite struct {
	suite.Suite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	state   state.State
	manager *sideromanager.Manager
	address string

	wg sync.WaitGroup
}

func (suite *SiderolinkSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	params := sideromanager.Params{
		WireguardEndpoint:  "127.0.0.1:0",
		AdvertisedEndpoint: config.Config.SiderolinkWireguardAdvertisedAddress + "," + TestIP,
		APIEndpoint:        "127.0.0.1:0",
	}

	var err error

	eventHandler := machineevent.NewHandler(suite.state, zaptest.NewLogger(suite.T()), make(chan *omni.MachineStatusSnapshot), nil)

	suite.manager, err = sideromanager.NewManager(suite.ctx, suite.state, &fakeWireguardHandler{}, params, zaptest.NewLogger(suite.T()), nil, eventHandler, nil)
	suite.Require().NoError(err)

	suite.startManager(params)
}

func (suite *SiderolinkSuite) startManager(params sideromanager.Params) {
	suite.wg.Add(1)

	lis, err := params.NewListener()
	suite.Require().NoError(err)

	suite.address = lis.Addr().String()

	go func() {
		defer suite.wg.Done()

		eg, groupCtx := errgroup.WithContext(suite.ctx)

		server := grpc.NewServer()

		suite.manager.Register(
			server,
		)

		eg.Go(func() error {
			return suite.manager.Run(
				groupCtx,
				"127.0.0.1",
				"0",
				"0",
				"",
			)
		})

		grpcutil.RunServer(groupCtx, server, lis, eg, zaptest.NewLogger(suite.T()))

		suite.Require().NoError(eg.Wait())
	}()
}

func (suite *SiderolinkSuite) TestNodes() {
	var spec *specs.ConnectionParamsSpec

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*2)
	defer cancel()

	rtestutils.AssertResources[*siderolink.Config](ctx, suite.T(), suite.state, []string{
		siderolink.ConfigID,
	}, func(r *siderolink.Config, assertion *assert.Assertions) {
		assertion.NotEmpty(r.TypedSpec().Value.JoinToken)
		assertion.NotEmpty(r.TypedSpec().Value.PrivateKey)
		assertion.NotEmpty(r.TypedSpec().Value.PublicKey)
	})

	rtestutils.AssertResources[*siderolink.ConnectionParams](ctx, suite.T(), suite.state, []string{
		siderolink.ConfigID,
	}, func(r *siderolink.ConnectionParams, assertion *assert.Assertions) {
		assertion.NotEmpty(r.TypedSpec().Value.Args)
		assertion.NotEmpty(r.TypedSpec().Value.ApiEndpoint)
		assertion.NotEmpty(r.TypedSpec().Value.JoinToken)
		assertion.NotEmpty(r.TypedSpec().Value.WireguardEndpoint)

		spec = r.TypedSpec().Value
	})

	conn, err := grpc.NewClient(suite.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	suite.Require().NoError(err)

	client := pb.NewProvisionServiceClient(conn)

	privateKey, err := wgtypes.GeneratePrivateKey()
	suite.Require().NoError(err)

	resp, err := client.Provision(suite.ctx, &pb.ProvisionRequest{
		NodeUuid:      "testnode",
		NodePublicKey: privateKey.PublicKey().String(),
		JoinToken:     &spec.JoinToken,
	})

	suite.Require().NoError(err)

	suite.Assert().NoError(
		retry.Constant(time.Second * 2).Retry(func() error {
			list, err := suite.state.List(suite.ctx, resource.NewMetadata(siderolink.Namespace, siderolink.LinkType, "", resource.VersionUndefined)) //nolint:govet
			if err != nil {
				return err
			}

			if len(list.Items) == 0 {
				return retry.ExpectedErrorf("no links established yet")
			}

			for _, item := range list.Items {
				if item.Metadata().ID() == "" {
					return errors.New("empty id in the resource list")
				}
			}

			return nil
		}),
	)

	reprovision, err := client.Provision(suite.ctx, &pb.ProvisionRequest{
		NodeUuid:      "testnode",
		NodePublicKey: privateKey.PublicKey().String(),
		JoinToken:     &spec.JoinToken,
	})

	suite.Assert().NoError(err)
	suite.Require().True(proto.Equal(resp, reprovision))

	privateKey, err = wgtypes.GeneratePrivateKey()
	suite.Assert().NoError(err)

	reprovision, err = client.Provision(suite.ctx, &pb.ProvisionRequest{
		NodeUuid:      "testnode",
		NodePublicKey: privateKey.PublicKey().String(),
		JoinToken:     &spec.JoinToken,
	})

	suite.Assert().NoError(err)
	suite.Require().True(proto.Equal(resp, reprovision))

	res, err := safe.StateGet[*siderolink.Link](suite.ctx, suite.state, resource.NewMetadata(siderolink.Namespace, siderolink.LinkType, "testnode", resource.VersionUndefined))
	suite.Assert().NoError(err)
	suite.Require().Equal(privateKey.PublicKey().String(), res.TypedSpec().Value.NodePublicKey)
}

func (suite *SiderolinkSuite) TestNodeWithSeveralAdvertisedIPs() {
	var spec *specs.ConnectionParamsSpec

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*2)
	defer cancel()

	rtestutils.AssertResources[*siderolink.ConnectionParams](ctx, suite.T(), suite.state, []string{
		siderolink.ConfigID,
	}, func(r *siderolink.ConnectionParams, assertion *assert.Assertions) {
		assertion.NotEmpty(r.TypedSpec().Value.Args)
		assertion.NotEmpty(r.TypedSpec().Value.ApiEndpoint)
		assertion.NotEmpty(r.TypedSpec().Value.JoinToken)
		assertion.NotEmpty(r.TypedSpec().Value.WireguardEndpoint)

		spec = r.TypedSpec().Value
	})

	conn := must.Value(grpc.NewClient(suite.address, grpc.WithTransportCredentials(insecure.NewCredentials())))(suite.T())
	client := pb.NewProvisionServiceClient(conn)
	privateKey := must.Value(wgtypes.GeneratePrivateKey())(suite.T())
	resp := must.Value(client.Provision(
		suite.ctx,
		&pb.ProvisionRequest{
			NodeUuid:      "testnode",
			NodePublicKey: privateKey.PublicKey().String(),
			JoinToken:     &spec.JoinToken,
		},
	))(suite.T())

	require.Equal(suite.T(), []string{config.Config.SiderolinkWireguardAdvertisedAddress, TestIP}, resp.GetEndpoints())
}

func (suite *SiderolinkSuite) TestVirtualNodes() {
	var spec *specs.ConnectionParamsSpec

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*2)
	defer cancel()

	rtestutils.AssertResources[*siderolink.Config](ctx, suite.T(), suite.state, []string{
		siderolink.ConfigID,
	}, func(r *siderolink.Config, assertion *assert.Assertions) {
		assertion.NotEmpty(r.TypedSpec().Value.JoinToken)
		assertion.NotEmpty(r.TypedSpec().Value.PrivateKey)
		assertion.NotEmpty(r.TypedSpec().Value.PublicKey)
	})

	rtestutils.AssertResources[*siderolink.ConnectionParams](ctx, suite.T(), suite.state, []string{
		siderolink.ConfigID,
	}, func(r *siderolink.ConnectionParams, assertion *assert.Assertions) {
		assertion.NotEmpty(r.TypedSpec().Value.Args)
		assertion.NotEmpty(r.TypedSpec().Value.ApiEndpoint)
		assertion.NotEmpty(r.TypedSpec().Value.JoinToken)
		assertion.NotEmpty(r.TypedSpec().Value.WireguardEndpoint)

		spec = r.TypedSpec().Value
	})

	conn, err := grpc.NewClient(suite.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	suite.Require().NoError(err)

	client := pb.NewProvisionServiceClient(conn)

	privateKey, err := wgtypes.GeneratePrivateKey()
	suite.Require().NoError(err)

	resp, err := client.Provision(suite.ctx, &pb.ProvisionRequest{
		NodeUuid:          "testnode",
		NodePublicKey:     privateKey.PublicKey().String(),
		JoinToken:         &spec.JoinToken,
		WireguardOverGrpc: pointer.To(true),
	})

	suite.Require().NoError(err)

	suite.Assert().NoError(
		retry.Constant(time.Second * 2).Retry(func() error {
			list, err := safe.ReaderList[*siderolink.Link](suite.ctx, suite.state, resource.NewMetadata(siderolink.Namespace, siderolink.LinkType, "", resource.VersionUndefined)) //nolint:govet
			if err != nil {
				return err
			}

			if list.Len() == 0 {
				return retry.ExpectedErrorf("no links established yet")
			}

			for link := range list.All() {
				if link.Metadata().ID() == "" {
					return errors.New("empty id in the resource list")
				}

				if link.TypedSpec().Value.VirtualAddrport == "" {
					return errors.New("empty virtual address in the resource list")
				}
			}

			return nil
		}),
	)

	reprovision, err := client.Provision(suite.ctx, &pb.ProvisionRequest{
		NodeUuid:      "testnode",
		NodePublicKey: privateKey.PublicKey().String(),
		JoinToken:     &spec.JoinToken,
	})

	expectedResp := resp.CloneVT()
	expectedResp.GrpcPeerAddrPort = ""
	expectedResp.ServerEndpoint = pb.MakeEndpoints(config.Config.SiderolinkWireguardAdvertisedAddress, TestIP)

	suite.Assert().NoError(err)

	suite.Require().Equal(expectedResp.String(), reprovision.String())

	privateKey, err = wgtypes.GeneratePrivateKey()
	suite.Assert().NoError(err)

	reprovision, err = client.Provision(suite.ctx, &pb.ProvisionRequest{
		NodeUuid:      "testnode",
		NodePublicKey: privateKey.PublicKey().String(),
		JoinToken:     &spec.JoinToken,
	})

	suite.Assert().NoError(err)
	suite.Require().Equal(expectedResp.String(), reprovision.String())

	res, err := safe.StateGet[*siderolink.Link](suite.ctx, suite.state, resource.NewMetadata(siderolink.Namespace, siderolink.LinkType, "testnode", resource.VersionUndefined))
	suite.Assert().NoError(err)
	suite.Require().Equal(privateKey.PublicKey().String(), res.TypedSpec().Value.NodePublicKey)
	suite.Require().Zero(res.TypedSpec().Value.VirtualAddrport)

	reprovision, err = client.Provision(suite.ctx, &pb.ProvisionRequest{
		NodeUuid:          "testnode",
		NodePublicKey:     privateKey.PublicKey().String(),
		JoinToken:         &spec.JoinToken,
		WireguardOverGrpc: pointer.To(true),
	})

	resp.GrpcPeerAddrPort = reprovision.GrpcPeerAddrPort
	resp.ServerEndpoint = reprovision.ServerEndpoint

	suite.Assert().NoError(err)
	suite.Require().Equal(resp.String(), reprovision.String())

	res, err = safe.StateGet[*siderolink.Link](suite.ctx, suite.state, resource.NewMetadata(siderolink.Namespace, siderolink.LinkType, "testnode", resource.VersionUndefined))
	suite.Assert().NoError(err)
	suite.Require().Equal(privateKey.PublicKey().String(), res.TypedSpec().Value.NodePublicKey)
	suite.Require().NotZero(res.TypedSpec().Value.VirtualAddrport)
	suite.Require().Equal(reprovision.GrpcPeerAddrPort, res.TypedSpec().Value.VirtualAddrport)
}

func (suite *SiderolinkSuite) TestGenerateJoinToken() {
	token, err := sideromanager.GenerateJoinToken()

	suite.Assert().NoError(err)

	tokenLen := len(token)
	suite.Assert().Less(tokenLen, 52)
	suite.Assert().Greater(tokenLen, 42)
}

func (suite *SiderolinkSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestSiderolinkSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SiderolinkSuite))
}

// TestIP from TEST-NET-1 network which can never be used.
const TestIP = "192.2.0.2"
