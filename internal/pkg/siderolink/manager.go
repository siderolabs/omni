// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/jxskiss/base62"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/go-retry/retry"
	eventsapi "github.com/siderolabs/siderolink/api/events"
	pb "github.com/siderolabs/siderolink/api/siderolink"
	"github.com/siderolabs/siderolink/pkg/events"
	"github.com/siderolabs/siderolink/pkg/wireguard"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/errgroup"
	"github.com/siderolabs/omni/internal/pkg/grpcutil"
	"github.com/siderolabs/omni/internal/pkg/logreceiver"
	"github.com/siderolabs/omni/internal/pkg/siderolink/trustd"
)

// LinkCounterDeltas represents a map of link counters deltas.
type LinkCounterDeltas = map[resource.ID]LinkCounterDelta

// LinkCounterDelta represents a delta of link counters, namely bytes sent and received.
type LinkCounterDelta struct {
	LastAlive     time.Time
	BytesSent     int64
	BytesReceived int64
}

// NewManager creates new Manager.
func NewManager(
	ctx context.Context,
	state state.State,
	wgHandler WireguardHandler,
	params Params,
	logger *zap.Logger,
	handler *LogHandler,
	deltaCh chan<- LinkCounterDeltas,
) (*Manager, error) {
	manager := &Manager{
		state:      state,
		wgHandler:  wgHandler,
		logger:     logger,
		logHandler: handler,
		deltaCh:    deltaCh,

		metricBytesReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omni_siderolink_received_bytes_total",
			Help: "Number of bytes received from the SideroLink interface.",
		}),
		metricBytesSent: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "omni_siderolink_sent_bytes_total",
			Help: "Number of bytes sent to the SideroLink interface.",
		}),
		metricLastHandshake: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "omni_siderolink_last_handshake_seconds",
			Help: "Time since the last handshake with the SideroLink interface.",
			Buckets: []float64{
				2 * 60,  // "normal" reconnect
				4 * 60,  // late, but within the bounds of "connected"
				8 * 60,  // considered disconnected now
				16 * 60, // super late
				32 * 60, // super duper late
				64 * 60, // more than hour... wth?
			},
		}),
	}

	nodePrefix := wireguard.NetworkPrefix("")
	manager.serverAddr = netip.PrefixFrom(nodePrefix.Addr().Next(), nodePrefix.Bits())

	cfg, err := manager.getOrCreateConfig(ctx, manager.serverAddr, nodePrefix, params)
	if err != nil {
		return nil, err
	}

	if err = manager.updateConnectionParams(ctx, cfg, params.EventSinkPort); err != nil {
		return nil, err
	}

	manager.config = cfg

	return manager, nil
}

// Manager sets up Siderolink server, manages it's state.
type Manager struct {
	pb.UnimplementedProvisionServiceServer
	config     *siderolink.Config
	logger     *zap.Logger
	state      state.State
	wgHandler  WireguardHandler
	logHandler *LogHandler

	metricBytesReceived prometheus.Counter
	metricBytesSent     prometheus.Counter
	metricLastHandshake prometheus.Histogram
	deltaCh             chan<- LinkCounterDeltas
	serverAddr          netip.Prefix
}

// JoinTokenLen number of random bytes to be encoded in the join token.
// The real length of the token will depend on the base62 encoding,
// whose lengths happpens to be non-deterministic.
const JoinTokenLen = 32

const siderolinkDevJoinTokenEnvVar = "SIDEROLINK_DEV_JOIN_TOKEN"

// getJoinToken returns the join token from the env or generates a new one.
func getJoinToken(logger *zap.Logger) (string, error) {
	joinToken := os.Getenv(siderolinkDevJoinTokenEnvVar)
	if joinToken == "" {
		return generateJoinToken()
	}

	if !constants.IsDebugBuild {
		logger.Sugar().Warnf("environment variable %s is set, but this is not a debug build, ignoring", siderolinkDevJoinTokenEnvVar)

		return generateJoinToken()
	}

	logger.Sugar().Warnf("using a static join token from environment variable %s. THIS IS NOT RECOMMENDED FOR PRODUCTION USE.", siderolinkDevJoinTokenEnvVar)

	return joinToken, nil
}

// generateJoinToken generates a base62 encoded token.
func generateJoinToken() (string, error) {
	b := make([]byte, JoinTokenLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}

	token := base62.EncodeToString(b)

	return token, nil
}

// Params are the parameters for the Manager.
type Params struct {
	WireguardEndpoint  string
	AdvertisedEndpoint string
	APIEndpoint        string
	Cert               string
	Key                string
	EventSinkPort      string
}

// NewListener creates a new listener.
func (p *Params) NewListener() (net.Listener, error) {
	if p.APIEndpoint == "" {
		return nil, errors.New("no siderolink API endpoint specified")
	}

	switch {
	case p.Cert == "" && p.Key == "":
		// no key, no cert - use plain TCP
		return net.Listen("tcp", p.APIEndpoint)
	case p.Cert == "":
		return nil, errors.New("siderolink cert is required")
	case p.Key == "":
		return nil, errors.New("siderolink key is required")
	}

	cert, err := tls.LoadX509KeyPair(p.Cert, p.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to load siderolink cert/key: %w", err)
	}

	return tls.Listen("tcp", p.APIEndpoint, &tls.Config{
		Certificates: []tls.Certificate{cert},
	})
}

func createListener(ctx context.Context, host, port string) (net.Listener, error) {
	endpoint := net.JoinHostPort(host, port)

	var (
		listener net.Listener
		err      error
	)

	if err = retry.Constant(20*time.Second, retry.WithUnits(100*time.Millisecond)).RetryWithContext(
		ctx, func(context.Context) error {
			listener, err = net.Listen("tcp", endpoint)
			if errors.Is(err, syscall.EADDRNOTAVAIL) {
				return retry.ExpectedError(err)
			}

			return err
		}); err != nil {
		return nil, fmt.Errorf("error listening for endpoint %s: %w", endpoint, err)
	}

	return listener, nil
}

// Register implements controller.Manager interface.
func (manager *Manager) Register(
	server *grpc.Server,
) {
	pb.RegisterProvisionServiceServer(server, manager)
}

// Run implements controller.Manager interface.
//
// If eventSinkHost is empty, event sink API will only listen on the siderolink interface.
func (manager *Manager) Run(
	ctx context.Context,
	listenHost string,
	eventSinkPort string,
	trustdPort string,
	logServerPort string,
) error {
	eg, groupCtx := errgroup.WithContext(ctx)

	if err := manager.startWireguard(groupCtx, eg, manager.serverAddr); err != nil {
		return err
	}

	eg.Go(func() error { return manager.pollWireguardPeers(groupCtx) })

	if listenHost == "" {
		listenHost = manager.serverAddr.Addr().String()
	}

	eventSinkListener, err := createListener(ctx, listenHost, eventSinkPort)
	if err != nil {
		return err
	}

	trustdListener, err := createListener(ctx, listenHost, trustdPort)
	if err != nil {
		return err
	}

	manager.startEventsGRPC(groupCtx, eg, eventSinkListener)
	manager.startTrustdGRPC(groupCtx, eg, trustdListener, manager.serverAddr)

	if logServerPort != "" {
		// Check that the log server port is set, and we are actually running the server
		err := manager.startLogServer(groupCtx, eg, manager.serverAddr, logServerPort)
		if err != nil {
			return err
		}
	}

	return eg.Wait()
}

func (manager *Manager) getOrCreateConfig(ctx context.Context, serverAddr netip.Prefix, nodePrefix netip.Prefix, params Params) (*siderolink.Config, error) {
	// get the existing configuration, or create a new one if it doesn't exist
	cfg, err := safe.StateGet[*siderolink.Config](ctx, manager.state, resource.NewMetadata(siderolink.Namespace, siderolink.ConfigType, siderolink.ConfigID, resource.VersionUndefined))
	if err != nil {
		if !state.IsNotFoundError(err) {
			return nil, err
		}

		newConfig := siderolink.NewConfig(siderolink.Namespace)
		cfg = newConfig

		spec := newConfig.TypedSpec().Value

		var privateKey wgtypes.Key

		privateKey, err = wgtypes.GeneratePrivateKey()
		if err != nil {
			return nil, fmt.Errorf("error generating key: %w", err)
		}

		spec.WireguardEndpoint = params.WireguardEndpoint
		spec.AdvertisedEndpoint = params.AdvertisedEndpoint
		spec.PrivateKey = privateKey.String()
		spec.PublicKey = privateKey.PublicKey().String()
		spec.ServerAddress = serverAddr.Addr().String()
		spec.Subnet = nodePrefix.String()

		var joinToken string

		joinToken, err = getJoinToken(manager.logger)
		if err != nil {
			return nil, fmt.Errorf("error getting join token: %w", err)
		}

		spec.JoinToken = joinToken

		if err = manager.state.Create(ctx, cfg); err != nil {
			return nil, err
		}
	}

	// start siderolink
	if cfg, err = safe.StateUpdateWithConflicts(ctx, manager.state, cfg.Metadata(), func(cfg *siderolink.Config) error {
		spec := cfg.TypedSpec().Value
		spec.WireguardEndpoint = params.WireguardEndpoint
		spec.AdvertisedEndpoint = params.AdvertisedEndpoint

		return nil
	}); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (manager *Manager) wgConfig() *specs.SiderolinkConfigSpec {
	return manager.config.TypedSpec().Value
}

func (manager *Manager) startWireguard(ctx context.Context, eg *errgroup.Group, serverAddr netip.Prefix) error {
	key, err := wgtypes.ParseKey(manager.wgConfig().PrivateKey)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	if err = manager.wgHandler.SetupDevice(serverAddr, key, manager.wgConfig().WireguardEndpoint, manager.logger); err != nil {
		return err
	}

	eg.Go(func() error {
		defer func() {
			err := manager.wgHandler.Shutdown()
			if err != nil {
				manager.logger.Error("error shutting down wireguard handler", zap.Error(err))
			}
		}()

		return manager.wgHandler.Run(ctx, manager.logger)
	})

	return nil
}

func (manager *Manager) startEventsGRPC(ctx context.Context, eg *errgroup.Group, listener net.Listener) {
	server := grpc.NewServer(
		grpc.SharedWriteBuffer(true),
	)
	adapter := NewEventsAdapter(manager.state, manager.logger)
	sink := events.NewSink(adapter, []proto.Message{
		&machineapi.MachineStatusEvent{},
	})
	eventsapi.RegisterEventSinkServiceServer(server, sink)

	eg.Go(func() error {
		err := adapter.CleanupMachineStatuses(
			ctx,
			manager.logger.With(
				zap.String("component", "events_machine_status_cleanup"),
			),
		)
		if err != nil {
			err = fmt.Errorf("machine status cleanup failed: %w", err)
		}

		return err
	})

	grpcutil.RunServer(ctx, server, listener, eg)
}

func (manager *Manager) startTrustdGRPC(ctx context.Context, eg *errgroup.Group, listener net.Listener, serverAddr netip.Prefix) {
	server := trustd.NewServer(manager.logger, manager.state, serverAddr.Addr().AsSlice()) //nolint:contextcheck

	grpcutil.RunServer(ctx, server, listener, eg)
}

func (manager *Manager) startLogServer(ctx context.Context, eg *errgroup.Group, serverAddr netip.Prefix, logServerPort string) error {
	logServerBindAddr := net.JoinHostPort(serverAddr.Addr().String(), logServerPort)

	logServer, err := logreceiver.MakeServer(logServerBindAddr, manager.logHandler, manager.logger)
	if err != nil {
		return err
	}

	manager.logger.Info("started log server", zap.String("bind_address", logServerBindAddr))

	eg.Go(logServer.Serve)
	eg.Go(func() error {
		<-ctx.Done()
		logServer.Stop()

		return nil
	})

	return nil
}

//nolint:gocognit,gocyclo,cyclop
func (manager *Manager) pollWireguardPeers(ctx context.Context) error {
	links, err := safe.StateListAll[*siderolink.Link](ctx, manager.state)
	if err != nil {
		return err
	}

	for iter := links.Iterator(); iter.Next(); {
		spec := iter.Value().TypedSpec().Value

		if err = manager.wgHandler.PeerEvent(ctx, spec, false); err != nil {
			return err
		}
	}

	ticker := time.NewTicker(time.Second * 30)

	defer ticker.Stop()

	previous := map[string]struct {
		receiveBytes  int64
		transmitBytes int64
	}{}

	var (
		updateCounter       int
		checkPeerConnection bool
	)

	// Poll wireguard peers
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			peers, err := manager.wgHandler.Peers()
			if err != nil {
				return err
			}

			// waiting for some grace period after the manager just started
			// check runs every 30 seconds, 60 seconds will pass after the second tick
			// after 2 iterations we enable peer connectivity check
			if updateCounter < 2 {
				updateCounter++
			} else {
				checkPeerConnection = true
			}

			peersByKey := map[string]wgtypes.Peer{}

			for _, peer := range peers {
				peersByKey[peer.PublicKey.String()] = peer
			}

			links, err := safe.StateListAll[*siderolink.Link](ctx, manager.state)
			if err != nil {
				return err
			}

			counterDeltas := make(LinkCounterDeltas, links.Len())

			for iter := links.Iterator(); iter.Next(); {
				link := iter.Value()
				spec := link.TypedSpec().Value

				peer, ok := peersByKey[spec.NodePublicKey]
				if !ok {
					continue
				}

				deltaReceived := peer.ReceiveBytes - previous[spec.NodePublicKey].receiveBytes
				if deltaReceived > 0 {
					manager.metricBytesReceived.Add(float64(deltaReceived))
				}

				deltaSent := peer.TransmitBytes - previous[spec.NodePublicKey].transmitBytes
				if deltaSent > 0 {
					manager.metricBytesSent.Add(float64(deltaSent))
				}

				counterDeltas[link.Metadata().ID()] = LinkCounterDelta{
					BytesSent:     deltaSent,
					BytesReceived: deltaReceived,
					LastAlive:     peer.LastHandshakeTime,
				}

				previous[spec.NodePublicKey] = struct {
					receiveBytes  int64
					transmitBytes int64
				}{
					receiveBytes:  peer.ReceiveBytes,
					transmitBytes: peer.TransmitBytes,
				}

				sinceLastHandshake := time.Since(peer.LastHandshakeTime)

				manager.metricLastHandshake.Observe(sinceLastHandshake.Seconds())

				if _, err = safe.StateUpdateWithConflicts(ctx, manager.state, link.Metadata(), func(res *siderolink.Link) error {
					spec = res.TypedSpec().Value
					if checkPeerConnection || !peer.LastHandshakeTime.IsZero() {
						spec.Connected = sinceLastHandshake < wireguard.PeerDownInterval
					}

					if !config.Config.SiderolinkDisableLastEndpoint {
						spec.LastEndpoint = ""

						return nil
					}

					if peer.Endpoint != nil {
						spec.LastEndpoint = peer.Endpoint.String()
					}

					return nil
				}); err != nil && !state.IsNotFoundError(err) && !state.IsPhaseConflictError(err) {
					return fmt.Errorf("failed to update link %w", err)
				}
			}

			if manager.deltaCh != nil {
				channel.SendWithContext(ctx, manager.deltaCh, counterDeltas)
			}
		}
	}
}

func (manager *Manager) updateConnectionParams(ctx context.Context, siderolinkConfig *siderolink.Config, eventSinkPort string) error {
	connectionParams := siderolink.NewConnectionParams(
		resources.DefaultNamespace,
		siderolinkConfig.Metadata().ID(),
	)

	var err error

	if err = manager.state.Create(ctx, connectionParams); err != nil && !state.IsConflictError(err) {
		return err
	}

	if _, err = safe.StateUpdateWithConflicts(ctx, manager.state, connectionParams.Metadata(), func(res *siderolink.ConnectionParams) error {
		spec := res.TypedSpec().Value

		spec.ApiEndpoint = config.Config.SideroLinkAPIURL
		spec.JoinToken = siderolinkConfig.TypedSpec().Value.JoinToken
		spec.WireguardEndpoint = siderolinkConfig.TypedSpec().Value.AdvertisedEndpoint

		spec.Args = fmt.Sprintf("%s=%s?jointoken=%s %s=%s %s=tcp://%s",
			talosconstants.KernelParamSideroLink,
			spec.ApiEndpoint,
			siderolinkConfig.TypedSpec().Value.JoinToken,
			talosconstants.KernelParamEventsSink,
			net.JoinHostPort(
				siderolinkConfig.TypedSpec().Value.ServerAddress,
				eventSinkPort,
			),
			talosconstants.KernelParamLoggingKernel,
			net.JoinHostPort(
				siderolinkConfig.TypedSpec().Value.ServerAddress,
				strconv.Itoa(config.Config.LogServerPort),
			),
		)

		manager.logger.Info(fmt.Sprintf("add this kernel argument to a Talos instance you would like to connect: %s", spec.Args))

		return nil
	}); err != nil {
		return err
	}

	return nil
}

type isDirty bool

// getLink will return either the existing Link for the requesting node or a new Link.
func (manager *Manager) getLink(ctx context.Context, req *pb.ProvisionRequest, id string) (*siderolink.Link, isDirty, error) {
	res, err := safe.StateGet[*siderolink.Link](ctx, manager.state, resource.NewMetadata(siderolink.Namespace, siderolink.LinkType, id, resource.VersionUndefined))
	if state.IsNotFoundError(err) {
		if manager.wgConfig().JoinToken == "" {
			return nil, false, status.Error(codes.PermissionDenied, "cannot accept new nodes if no join token is set")
		}

		if req.JoinToken == nil || *req.JoinToken != manager.wgConfig().JoinToken {
			return nil, false, status.Error(codes.PermissionDenied, "invalid join token")
		}

		var spec *specs.SiderolinkSpec

		spec, err = manager.generateLinkSpec(req)
		if err != nil {
			return nil, false, err
		}

		spec.Connected = true

		link := siderolink.NewLink(siderolink.Namespace, id, spec)

		if err = manager.state.Create(ctx, link); err != nil {
			return nil, false, err
		}

		return link, true, nil
	}

	return res, false, err
}

// Provision the SideroLink.
func (manager *Manager) Provision(ctx context.Context, req *pb.ProvisionRequest) (*pb.ProvisionResponse, error) {
	ctx = actor.MarkContextAsInternalActor(ctx)

	link, dirty, err := manager.getLink(ctx, req, req.NodeUuid)
	if err != nil {
		return nil, err
	}

	spec := link.TypedSpec().Value
	if spec.NodePublicKey != req.NodePublicKey {
		if _, err = safe.StateUpdateWithConflicts(ctx, manager.state, link.Metadata(), func(r *siderolink.Link) error {
			s := r.TypedSpec().Value

			if err = manager.wgHandler.PeerEvent(ctx, s, true); err != nil {
				return err
			}

			s.NodePublicKey = req.NodePublicKey

			spec = s

			return nil
		}); err != nil {
			return nil, err
		}

		dirty = true
	}

	if dirty {
		if err = manager.wgHandler.PeerEvent(ctx, spec, false); err != nil {
			return nil, err
		}
	}

	endpoint := manager.wgConfig().WireguardEndpoint
	if manager.wgConfig().AdvertisedEndpoint != "" {
		endpoint = manager.wgConfig().AdvertisedEndpoint
	}

	return &pb.ProvisionResponse{
		ServerEndpoint:    []string{endpoint},
		ServerPublicKey:   manager.wgConfig().PublicKey,
		ServerAddress:     manager.wgConfig().ServerAddress,
		NodeAddressPrefix: spec.NodeSubnet,
	}, nil
}

func (manager *Manager) generateLinkSpec(req *pb.ProvisionRequest) (*specs.SiderolinkSpec, error) {
	nodePrefix := netip.MustParsePrefix(manager.wgConfig().Subnet)

	// generated random address for the node
	raw := nodePrefix.Addr().As16()
	salt := make([]byte, 8)

	_, err := io.ReadFull(rand.Reader, salt)
	if err != nil {
		return nil, err
	}

	copy(raw[8:], salt)

	nodeAddress := netip.PrefixFrom(netip.AddrFrom16(raw), nodePrefix.Bits())

	pubKey, err := wgtypes.ParseKey(req.NodePublicKey)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("error parsing Wireguard key: %s", err))
	}

	return &specs.SiderolinkSpec{
		NodeSubnet:    nodeAddress.String(),
		NodePublicKey: pubKey.String(),
	}, nil
}

// Describe implements prom.Collector interface.
func (manager *Manager) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(manager, ch)
}

// Collect implements prom.Collector interface.
func (manager *Manager) Collect(ch chan<- prometheus.Metric) {
	ch <- manager.metricBytesReceived
	ch <- manager.metricBytesSent
	ch <- manager.metricLastHandshake
}

var _ prometheus.Collector = &Manager{}
