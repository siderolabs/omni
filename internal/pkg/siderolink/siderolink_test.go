// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	xmaps "github.com/siderolabs/gen/maps"
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
	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/errgroup"
	"github.com/siderolabs/omni/internal/pkg/grpcutil"
	"github.com/siderolabs/omni/internal/pkg/machineevent"
	sideromanager "github.com/siderolabs/omni/internal/pkg/siderolink"
)

type fakeWireguardHandler struct {
	peers   map[string]wgtypes.Peer
	peersMu sync.Mutex
}

func (h *fakeWireguardHandler) SetupDevice(wireguard.DeviceConfig) error {
	return nil
}

func (h *fakeWireguardHandler) Run(ctx context.Context, _ *zap.Logger) error {
	<-ctx.Done()

	return nil
}

func (h *fakeWireguardHandler) Shutdown() error {
	return nil
}

func (h *fakeWireguardHandler) PeerEvent(_ context.Context, spec *specs.SiderolinkSpec, deleted bool) error {
	h.peersMu.Lock()
	defer h.peersMu.Unlock()

	if deleted {
		delete(h.peers, spec.NodePublicKey)
	} else {
		if _, ok := h.peers[spec.NodePublicKey]; ok {
			return fmt.Errorf("peer already exists")
		}

		h.peers[spec.NodePublicKey] = wgtypes.Peer{}
	}

	return nil
}

func (h *fakeWireguardHandler) Peers() ([]wgtypes.Peer, error) {
	h.peersMu.Lock()
	defer h.peersMu.Unlock()

	return xmaps.Values(h.peers), nil
}

func (h *fakeWireguardHandler) GetPeersMap() map[string]wgtypes.Peer {
	h.peersMu.Lock()
	defer h.peersMu.Unlock()

	return maps.Clone(h.peers)
}

type SiderolinkSuite struct {
	suite.Suite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	state           state.State
	manager         *sideromanager.Manager
	runtime         *runtime.Runtime
	nodeUniqueToken *string
	address         string

	wg sync.WaitGroup
}

func (suite *SiderolinkSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(suite.T().Context(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	params := sideromanager.Params{
		WireguardEndpoint:  "127.0.0.1:0",
		AdvertisedEndpoint: config.Config.Services.Siderolink.WireGuard.AdvertisedEndpoint + "," + TestIP,
		MachineAPIEndpoint: "127.0.0.1:0",
	}

	nodeUniqueToken, err := jointoken.NewNodeUniqueToken("fingerprint", "test-token").Encode()

	suite.Require().NoError(err)

	suite.nodeUniqueToken = pointer.To(nodeUniqueToken)

	wgHandler := &fakeWireguardHandler{
		peers: map[string]wgtypes.Peer{},
	}

	eventHandler := machineevent.NewHandler(suite.state, zaptest.NewLogger(suite.T()), make(chan *omni.MachineStatusSnapshot), nil)

	suite.manager, err = sideromanager.NewManager(suite.ctx, suite.state, wgHandler, params, zaptest.NewLogger(suite.T()), nil, eventHandler, nil)
	suite.Require().NoError(err)

	suite.startManager(params)

	logger := zaptest.NewLogger(suite.T())

	suite.runtime, err = runtime.NewRuntime(suite.state, logger.WithOptions(zap.IncreaseLevel(zap.InfoLevel)))
	suite.Require().NoError(err)

	suite.wg.Add(1)

	peers := sideromanager.NewPeersPool(logger, wgHandler)

	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewLinkStatusController[*siderolink.PendingMachine](peers)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewLinkStatusController[*siderolink.Link](peers)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewConnectionParamsController()))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewSiderolinkAPIConfigController(&config.Config.Services)))
	suite.Require().NoError(suite.runtime.RegisterQController(omnictrl.NewJoinTokenStatusController()))

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
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
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*2)
	defer cancel()

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{
		siderolink.ConfigID,
	}, func(r *siderolink.Config, assertion *assert.Assertions) {
		assertion.NotEmpty(r.TypedSpec().Value.InitialJoinToken)
		assertion.NotEmpty(r.TypedSpec().Value.PrivateKey)
		assertion.NotEmpty(r.TypedSpec().Value.PublicKey)
	})

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{
		siderolink.ConfigID,
	}, func(r *siderolink.APIConfig, assertion *assert.Assertions) {
		assertion.NotEmpty(r.TypedSpec().Value.MachineApiAdvertisedUrl)
		assertion.NotEmpty(r.TypedSpec().Value.WireguardAdvertisedEndpoint)
	})

	var joinToken string

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{
		siderolink.DefaultJoinTokenID,
	}, func(r *siderolink.DefaultJoinToken, assertion *assert.Assertions) {
		assertion.NotEmpty(r.TypedSpec().Value.TokenId)

		joinToken = r.TypedSpec().Value.TokenId
	})

	conn, err := grpc.NewClient(suite.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	suite.Require().NoError(err)

	client := pb.NewProvisionServiceClient(conn)

	privateKey, err := wgtypes.GeneratePrivateKey()
	suite.Require().NoError(err)

	resp, err := client.Provision(suite.ctx, &pb.ProvisionRequest{
		NodeUuid:        "testnode",
		NodePublicKey:   privateKey.PublicKey().String(),
		JoinToken:       &joinToken,
		TalosVersion:    pointer.To("v1.9.0"),
		NodeUniqueToken: suite.nodeUniqueToken,
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
		NodeUuid:        "testnode",
		NodePublicKey:   privateKey.PublicKey().String(),
		JoinToken:       &joinToken,
		TalosVersion:    pointer.To("v1.9.0"),
		NodeUniqueToken: suite.nodeUniqueToken,
	})

	suite.Assert().NoError(err)
	suite.Require().True(proto.Equal(resp, reprovision))

	privateKey, err = wgtypes.GeneratePrivateKey()
	suite.Assert().NoError(err)

	reprovision, err = client.Provision(suite.ctx, &pb.ProvisionRequest{
		NodeUuid:        "testnode",
		NodePublicKey:   privateKey.PublicKey().String(),
		JoinToken:       &joinToken,
		TalosVersion:    pointer.To("v1.9.0"),
		NodeUniqueToken: suite.nodeUniqueToken,
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

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{
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
			NodeUuid:        "testnode",
			NodePublicKey:   privateKey.PublicKey().String(),
			JoinToken:       &spec.JoinToken,
			TalosVersion:    pointer.To("v1.9.0"),
			NodeUniqueToken: suite.nodeUniqueToken,
		},
	))(suite.T())

	require.Equal(suite.T(), []string{config.Config.Services.Siderolink.WireGuard.AdvertisedEndpoint, TestIP}, resp.GetEndpoints())
}

func (suite *SiderolinkSuite) TestVirtualNodes() {
	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*2)
	defer cancel()

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{
		siderolink.ConfigID,
	}, func(r *siderolink.Config, assertion *assert.Assertions) {
		assertion.NotEmpty(r.TypedSpec().Value.InitialJoinToken)
		assertion.NotEmpty(r.TypedSpec().Value.PrivateKey)
		assertion.NotEmpty(r.TypedSpec().Value.PublicKey)
	})

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{
		siderolink.ConfigID,
	}, func(r *siderolink.APIConfig, assertion *assert.Assertions) {
		assertion.NotEmpty(r.TypedSpec().Value.MachineApiAdvertisedUrl)
		assertion.NotEmpty(r.TypedSpec().Value.WireguardAdvertisedEndpoint)
	})

	var joinToken string

	rtestutils.AssertResources(ctx, suite.T(), suite.state, []string{
		siderolink.DefaultJoinTokenID,
	}, func(r *siderolink.DefaultJoinToken, assertion *assert.Assertions) {
		assertion.NotEmpty(r.TypedSpec().Value.TokenId)

		joinToken = r.TypedSpec().Value.TokenId
	})

	conn, err := grpc.NewClient(suite.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	suite.Require().NoError(err)

	client := pb.NewProvisionServiceClient(conn)

	privateKey, err := wgtypes.GeneratePrivateKey()
	suite.Require().NoError(err)

	resp, err := client.Provision(suite.ctx, &pb.ProvisionRequest{
		NodeUuid:          "testnode",
		NodePublicKey:     privateKey.PublicKey().String(),
		JoinToken:         &joinToken,
		WireguardOverGrpc: pointer.To(true),
		TalosVersion:      pointer.To("v1.9.0"),
		NodeUniqueToken:   suite.nodeUniqueToken,
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
		NodeUuid:        "testnode",
		NodePublicKey:   privateKey.PublicKey().String(),
		JoinToken:       &joinToken,
		TalosVersion:    pointer.To("v1.9.0"),
		NodeUniqueToken: suite.nodeUniqueToken,
	})

	expectedResp := resp.CloneVT()
	expectedResp.GrpcPeerAddrPort = ""
	expectedResp.ServerEndpoint = pb.MakeEndpoints(config.Config.Services.Siderolink.WireGuard.AdvertisedEndpoint, TestIP)

	suite.Assert().NoError(err)

	suite.Require().Equal(expectedResp.String(), reprovision.String())

	privateKey, err = wgtypes.GeneratePrivateKey()
	suite.Assert().NoError(err)

	reprovision, err = client.Provision(suite.ctx, &pb.ProvisionRequest{
		NodeUuid:        "testnode",
		NodePublicKey:   privateKey.PublicKey().String(),
		JoinToken:       &joinToken,
		TalosVersion:    pointer.To("v1.9.0"),
		NodeUniqueToken: suite.nodeUniqueToken,
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
		JoinToken:         &joinToken,
		WireguardOverGrpc: pointer.To(true),
		TalosVersion:      pointer.To("v1.9.0"),
		NodeUniqueToken:   suite.nodeUniqueToken,
	})

	suite.Require().NoError(err)

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
