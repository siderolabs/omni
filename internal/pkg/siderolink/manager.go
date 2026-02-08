// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/go-retry/retry"
	eventsapi "github.com/siderolabs/siderolink/api/events"
	pb "github.com/siderolabs/siderolink/api/siderolink"
	"github.com/siderolabs/siderolink/pkg/events"
	"github.com/siderolabs/siderolink/pkg/tun"
	"github.com/siderolabs/siderolink/pkg/wgtunnel/wgbind"
	"github.com/siderolabs/siderolink/pkg/wgtunnel/wggrpc"
	"github.com/siderolabs/siderolink/pkg/wireguard"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/errgroup"
	"github.com/siderolabs/omni/internal/pkg/grpcutil"
	"github.com/siderolabs/omni/internal/pkg/logreceiver"
	"github.com/siderolabs/omni/internal/pkg/machineevent"
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

// maxPendingClientMessages sets the maximum number of messages for queue "from peers" after which it will block.
const maxPendingClientMessages = 100

// NewManager creates new Manager.
func NewManager(
	ctx context.Context,
	state state.State,
	wgHandler WireguardHandler,
	params Params,
	logger *zap.Logger,
	handler *LogHandler,
	machineEventHandler *machineevent.Handler,
	deltaCh chan<- LinkCounterDeltas,
) (*Manager, error) {
	manager := &Manager{
		logger:              logger,
		state:               state,
		wgHandler:           wgHandler,
		logHandler:          handler,
		machineEventHandler: machineEventHandler,
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
		deltaCh:             deltaCh,
		allowedPeers:        wggrpc.NewAllowedPeers(),
		peerTraffic:         wgbind.NewPeerTraffic(maxPendingClientMessages),
		virtualPrefix:       wireguard.VirtualNetworkPrefix(),
		disableLastEndpoint: params.DisableLastEndpoint,
		provisionServer: NewProvisionHandler(
			logger,
			state,
			params.JoinTokensMode,
			params.UseGRPCTunnel,
		),
	}

	nodePrefix := wireguard.NetworkPrefix("")
	manager.serverAddr = netip.PrefixFrom(nodePrefix.Addr().Next(), nodePrefix.Bits())

	cfg, err := manager.getOrCreateConfig(ctx, manager.serverAddr, nodePrefix, params)
	if err != nil {
		return nil, err
	}

	manager.config = cfg

	return manager, nil
}

// Manager sets up Siderolink server, manages it's state.
type Manager struct {
	config              *siderolink.Config
	logger              *zap.Logger
	state               state.State
	wgHandler           WireguardHandler
	logHandler          *LogHandler
	machineEventHandler *machineevent.Handler
	provisionServer     *ProvisionHandler

	metricBytesReceived prometheus.Counter
	metricBytesSent     prometheus.Counter
	metricLastHandshake prometheus.Histogram
	deltaCh             chan<- LinkCounterDeltas
	serverAddr          netip.Prefix
	allowedPeers        *wggrpc.AllowedPeers
	peerTraffic         *wgbind.PeerTraffic
	virtualPrefix       netip.Prefix
	disableLastEndpoint bool
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
		return jointoken.Generate()
	}

	if !constants.IsDebugBuild {
		logger.Sugar().Warnf("environment variable %s is set, but this is not a debug build, ignoring", siderolinkDevJoinTokenEnvVar)

		return jointoken.Generate()
	}

	logger.Sugar().Warnf("using a static join token from environment variable %s. THIS IS NOT RECOMMENDED FOR PRODUCTION USE.", siderolinkDevJoinTokenEnvVar)

	return joinToken, nil
}

// Params are the parameters for the Manager.
type Params struct {
	WireguardEndpoint   string
	AdvertisedEndpoint  string
	MachineAPIEndpoint  string
	MachineAPITLSCert   string
	MachineAPITLSKey    string
	EventSinkPort       string
	JoinTokensMode      config.SiderolinkServiceJoinTokensMode
	UseGRPCTunnel       bool
	DisableLastEndpoint bool
}

// NewListener creates a new listener.
func (p *Params) NewListener(ctx context.Context) (net.Listener, error) {
	if p.MachineAPIEndpoint == "" {
		return nil, errors.New("no siderolink API endpoint specified")
	}

	switch {
	case p.MachineAPITLSCert == "" && p.MachineAPITLSKey == "":
		// no key, no cert - use plain TCP
		return (&net.ListenConfig{}).Listen(ctx, "tcp", p.MachineAPIEndpoint)
	case p.MachineAPITLSCert == "":
		return nil, errors.New("siderolink cert is required")
	case p.MachineAPITLSKey == "":
		return nil, errors.New("siderolink key is required")
	}

	cert, err := tls.LoadX509KeyPair(p.MachineAPITLSCert, p.MachineAPITLSKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load siderolink cert/key: %w", err)
	}

	return tls.Listen("tcp", p.MachineAPIEndpoint, &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h2"},
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
			listener, err = (&net.ListenConfig{}).Listen(ctx, "tcp", endpoint)
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
func (manager *Manager) Register(server *grpc.Server) {
	pb.RegisterProvisionServiceServer(server, manager.provisionServer)
	pb.RegisterWireGuardOverGRPCServiceServer(server,
		wggrpc.NewService(manager.peerTraffic, manager.allowedPeers, manager.logger.WithOptions(zap.IncreaseLevel(zap.InfoLevel))))
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
	eg, ctx := errgroup.WithContext(ctx)

	if err := manager.runWireguard(ctx, eg, listenHost, eventSinkPort, trustdPort, logServerPort); err != nil {
		return err
	}

	return eg.Wait()
}

func (manager *Manager) runWireguard(ctx context.Context, eg *errgroup.Group, listenHost, eventSinkPort, trustdPort, logServerPort string) error {
	wireguardCtx, wireguardCancel := context.WithCancel(context.Background())
	defer wireguardCancel()

	if err := manager.startWireguard(wireguardCtx, eg, manager.serverAddr); err != nil { //nolint:contextcheck
		return err
	}

	if err := manager.runWireguardServices(ctx, listenHost, eventSinkPort, trustdPort, logServerPort); err != nil {
		return fmt.Errorf("error running wireguard services: %w", err)
	}

	return nil
}

func (manager *Manager) runWireguardServices(ctx context.Context, listenHost, eventSinkPort, trustdPort, logServerPort string) error {
	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error { return manager.pollWireguardPeers(ctx) })

	eg.Go(func() error { return manager.provisionServer.runCleanup(ctx) })

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

	manager.startEventsGRPC(ctx, eg, eventSinkListener)
	manager.startTrustdGRPC(ctx, eg, trustdListener, manager.serverAddr)

	if logServerPort != "" {
		// Check that the log server port is set, and we are actually running the server
		if err = manager.startLogServer(ctx, eg, manager.serverAddr, logServerPort); err != nil {
			return err
		}
	}

	err = eg.Wait()

	gracePeriod := 5 * time.Second
	manager.logger.Info("all wireguard services exited, waiting to allow transmission of final packages to kernel", zap.Duration("grace_pediod", gracePeriod))
	time.Sleep(gracePeriod)

	return err
}

func (manager *Manager) getOrCreateConfig(ctx context.Context, serverAddr netip.Prefix, nodePrefix netip.Prefix, params Params) (*siderolink.Config, error) {
	// get the existing configuration, or create a new one if it doesn't exist
	cfg, err := safe.StateGet[*siderolink.Config](ctx, manager.state, resource.NewMetadata(siderolink.Namespace, siderolink.ConfigType, siderolink.ConfigID, resource.VersionUndefined))
	if err != nil {
		if !state.IsNotFoundError(err) {
			return nil, err
		}

		newConfig := siderolink.NewConfig()
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

		spec.InitialJoinToken, err = getJoinToken(manager.logger)
		if err != nil {
			return nil, err
		}

		if err = manager.state.Create(ctx, cfg); err != nil {
			return nil, err
		}
	}

	if err = manager.ensureDefaultJoinToken(ctx, cfg.TypedSpec().Value.InitialJoinToken); err != nil {
		return nil, err
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

func (manager *Manager) ensureDefaultJoinToken(ctx context.Context, token string) error {
	joinTokens, err := safe.ReaderListAll[*siderolink.JoinToken](ctx, manager.state)
	if err != nil {
		return err
	}

	if joinTokens.Len() != 0 {
		return nil
	}

	manager.logger.Info("initializing the default join token")

	defaultJoinToken := siderolink.NewDefaultJoinToken()

	defaultJoinToken.TypedSpec().Value.TokenId = token

	if err = manager.state.Create(ctx, defaultJoinToken); err != nil {
		if !state.IsConflictError(err) {
			return err
		}

		if _, err = safe.StateUpdateWithConflicts(ctx, manager.state, defaultJoinToken.Metadata(), func(res *siderolink.DefaultJoinToken) error {
			res.TypedSpec().Value.TokenId = token

			return nil
		}); err != nil {
			return err
		}
	}

	joinToken := siderolink.NewJoinToken(token)

	joinToken.TypedSpec().Value.Name = "initial token"

	return manager.state.Create(ctx, joinToken)
}

func (manager *Manager) wgConfig() *specs.SiderolinkConfigSpec {
	return manager.config.TypedSpec().Value
}

// newBind creates a WireGuard bind. If host is a specific IP address, it creates
// a bound bind that listens only on that address instead of all interfaces.
func (manager *Manager) newBind(host string) conn.Bind {
	if host == "" || host == "0.0.0.0" || host == "::" {
		manager.logger.Info("binding wireguard to all interfaces", zap.String("host", host))

		if runtime.GOOS == "darwin" {
			manager.logger.Warn("MACOS DETECTED: binding to all interfaces may cause connectivity issues " +
				"if multiple network interfaces share the same subnet. " +
				"Consider setting a specific IP address in wireguard endpoint.")
		}

		return conn.NewDefaultBind()
	}

	manager.logger.Info("binding wireguard to specific address", zap.String("addr", host))

	return NewBoundBind(host)
}

func (manager *Manager) startWireguard(ctx context.Context, eg *errgroup.Group, serverAddr netip.Prefix) error {
	key, err := wgtypes.ParseKey(manager.wgConfig().PrivateKey)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	host, strPort, err := net.SplitHostPort(manager.wgConfig().WireguardEndpoint)
	if err != nil {
		return fmt.Errorf("invalid Wireguard endpoint: %w", err)
	}

	port, err := strconv.Atoi(strPort)
	if err != nil {
		return fmt.Errorf("invalid Wireguard endpoint port: %w", err)
	}

	peerHandler := &peerHandler{
		allowedPeers: manager.allowedPeers,
	}

	if err = manager.wgHandler.SetupDevice(wireguard.DeviceConfig{
		Bind:               wgbind.NewServerBind(manager.newBind(host), manager.virtualPrefix, manager.peerTraffic, manager.logger),
		PeerHandler:        peerHandler,
		Logger:             manager.logger,
		ServerPrefix:       serverAddr,
		PrivateKey:         key,
		ListenPort:         uint16(port),
		InputPacketFilters: []tun.InputPacketFilter{tun.FilterAllExceptIP(serverAddr.Addr())},
	}); err != nil {
		return err
	}

	eg.Go(func() error {
		defer func() {
			err := manager.wgHandler.Shutdown()
			if err != nil {
				manager.logger.Error("error shutting down wireguard handler", zap.Error(err))
			}
		}()

		return manager.wgHandler.Run(ctx, manager.logger.WithOptions(zap.IncreaseLevel(zap.InfoLevel)))
	})

	return nil
}

func (manager *Manager) startEventsGRPC(ctx context.Context, eg *errgroup.Group, listener net.Listener) {
	server := grpc.NewServer(
		grpc.SharedWriteBuffer(true),
	)
	sink := events.NewSink(manager.machineEventHandler, []proto.Message{
		&machineapi.MachineStatusEvent{},
		&machineapi.SequenceEvent{}, // used to detect if Talos was installed on the disk, which is used by static infra providers (e.g., bare-metal infra provider)
	})
	eventsapi.RegisterEventSinkServiceServer(server, sink)

	grpcutil.RunServer(ctx, server, listener, eg, manager.logger)
}

func (manager *Manager) startTrustdGRPC(ctx context.Context, eg *errgroup.Group, listener net.Listener, serverAddr netip.Prefix) {
	server := trustd.NewServer(manager.logger, manager.state, serverAddr.Addr().AsSlice()) //nolint:contextcheck

	grpcutil.RunServer(ctx, server, listener, eg, manager.logger)
}

func (manager *Manager) startLogServer(ctx context.Context, eg *errgroup.Group, serverAddr netip.Prefix, logServerPort string) error {
	logServerBindAddr := net.JoinHostPort(serverAddr.Addr().String(), logServerPort)

	logServer, err := logreceiver.MakeServer(ctx, logServerBindAddr, manager.logHandler, manager.logger)
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

//nolint:gocognit
func (manager *Manager) pollWireguardPeers(ctx context.Context) error {
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

			for link := range links.All() {
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

					if manager.disableLastEndpoint {
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

type peerHandler struct {
	allowedPeers *wggrpc.AllowedPeers
}

func (p *peerHandler) HandlePeerAdded(event wireguard.PeerEvent) error {
	if event.VirtualAddr.IsValid() {
		p.allowedPeers.AddToken(event.PubKey, event.VirtualAddr.String())
	}

	return nil
}

func (p *peerHandler) HandlePeerRemoved(pubKey wgtypes.Key) error {
	p.allowedPeers.RemoveToken(pubKey)

	return nil
}
