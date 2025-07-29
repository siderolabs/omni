// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	pb "github.com/siderolabs/siderolink/api/siderolink"
	"github.com/siderolabs/siderolink/pkg/wireguard"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/siderolink"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/config"
)

var errUUIDConflict = fmt.Errorf("UUID conflict")

type provisionContext struct {
	siderolinkConfig       *siderolinkres.Config
	link                   *siderolinkres.Link
	pendingMachine         *siderolinkres.PendingMachine
	pendingMachineStatus   *siderolinkres.PendingMachineStatus
	token                  *jointoken.JoinToken
	requestNodeUniqueToken *jointoken.NodeUniqueToken
	linkNodeUniqueToken    *jointoken.NodeUniqueToken
	request                *pb.ProvisionRequest

	// flags
	hasValidJoinToken         bool
	hasValidNodeUniqueToken   bool
	nodeUniqueTokensEnabled   bool
	forceValidNodeUniqueToken bool
	supportsSecureJoinTokens  bool
	tokenWasWiped             bool
	useWireguardOverGRPC      bool
}

func (pc *provisionContext) isAuthorizedLegacyJoin() bool {
	// explicitly reject legacy machine if the link has node unique token set
	if !pc.supportsSecureJoinTokens && pc.linkNodeUniqueToken != nil && pc.nodeUniqueTokensEnabled {
		return false
	}

	return pc.hasValidJoinToken
}

func (pc *provisionContext) isAuthorizedSecureFlow() bool {
	return pc.hasValidJoinToken || pc.hasValidNodeUniqueToken
}

// NewProvisionHandler creates a new ProvisionHandler.
func NewProvisionHandler(logger *zap.Logger, state state.State, joinTokenMode string, forceWireguardOverGRPC bool) *ProvisionHandler {
	return &ProvisionHandler{
		logger:                 logger,
		state:                  state,
		joinTokenMode:          joinTokenMode,
		forceWireguardOverGRPC: forceWireguardOverGRPC,
	}
}

// ProvisionHandler is the gRPC service that handles provision responses coming from the Talos nodes.
type ProvisionHandler struct {
	pb.UnimplementedProvisionServiceServer

	logger                 *zap.Logger
	state                  state.State
	joinTokenMode          string
	forceWireguardOverGRPC bool
}

func (h *ProvisionHandler) runCleanup(ctx context.Context) error {
	ticker := time.NewTicker(time.Second * 30)

	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			pendingMachines, err := safe.ReaderListAll[*siderolinkres.PendingMachine](ctx, h.state)
			if err != nil {
				h.logger.Error("pending machine cleanup failed", zap.Error(err))
			}

			for machine := range pendingMachines.All() {
				if time.Since(machine.Metadata().Updated()) > time.Second*30 || machine.Metadata().Phase() == resource.PhaseTearingDown {
					if err = h.removePendingMachine(ctx, machine); err != nil {
						h.logger.Error("failed to remove pending machine", zap.Error(err), zap.String("id", machine.Metadata().ID()))
					}
				}
			}
		}
	}
}

// Provision handles the requests from Talos nodes.
func (h *ProvisionHandler) Provision(ctx context.Context, req *pb.ProvisionRequest) (*pb.ProvisionResponse, error) {
	ctx = actor.MarkContextAsInternalActor(ctx)

	provisionContext, err := h.buildProvisionContext(ctx, req)
	if err != nil {
		return nil, err
	}

	if !provisionContext.supportsSecureJoinTokens && h.joinTokenMode == config.JoinTokensModeStrict {
		return nil, status.Errorf(
			codes.FailedPrecondition,
			"Talos version %s is not supported on this Omni instance as '--join-tokens-mode' is set to 'strict'",
			pointer.SafeDeref(req.TalosVersion),
		)
	}

	resp, err := h.provision(ctx, provisionContext)
	if err != nil && status.Code(err) == codes.Unknown {
		h.logger.Error("failed to handle machine provision request", zap.Error(err))

		return nil, status.Error(codes.Internal, "internal server error")
	}

	return resp, err
}

type res interface {
	generic.ResourceWithRD
	TypedSpec() *protobuf.ResourceSpec[specs.SiderolinkSpec, *specs.SiderolinkSpec]
}

func updateAnnotations(res resource.Resource, annotationsToAdd []string, annotationsToRemove []string) {
	for _, annotation := range annotationsToAdd {
		res.Metadata().Annotations().Set(annotation, "")
	}

	for _, annotation := range annotationsToRemove {
		res.Metadata().Annotations().Delete(annotation)
	}
}

// newLink creates the link resource (PendingMachine/Link).
func newLink[T res](provisionContext *provisionContext,
	annotationsToAdd []string, annotationsToRemove []string,
) (T, error) {
	var (
		zero T
		id   string
	)

	rd := zero.ResourceDefinition()

	if rd.Type == siderolinkres.LinkType {
		id = provisionContext.request.NodeUuid
	} else {
		id = provisionContext.request.NodePublicKey
	}

	var (
		res resource.Resource
		err error
	)

	res, err = protobuf.CreateResource(rd.Type)
	if err != nil {
		return zero, err
	}

	*res.Metadata() = resource.NewMetadata(rd.DefaultNamespace, rd.Type, id, resource.VersionUndefined)

	link, ok := res.(T)
	if !ok {
		return zero, fmt.Errorf("incorrect resource type")
	}

	if provisionContext.token != nil {
		if value, ok := provisionContext.token.ExtraData[omni.LabelInfraProviderID]; ok {
			link.Metadata().Annotations().Set(omni.LabelInfraProviderID, value)
		}

		if value, ok := provisionContext.token.ExtraData[omni.LabelMachineRequest]; ok {
			link.Metadata().Labels().Set(omni.LabelMachineRequest, value)
		}
	}

	link.TypedSpec().Value, err = generateLinkSpec(provisionContext)
	if err != nil {
		return zero, err
	}

	if link.Metadata().Type() == siderolinkres.PendingMachineType {
		link.Metadata().Labels().Set(omni.MachineUUID, provisionContext.request.NodeUuid)
	}

	updateAnnotations(res, annotationsToAdd, annotationsToRemove)

	return link, nil
}

func updateNodeUniqueToken(ctx context.Context, logger *zap.Logger, st state.State, provisionContext *provisionContext) error {
	return safe.StateModify(ctx, st, siderolinkres.NewNodeUniqueToken(provisionContext.request.NodeUuid),
		func(res *siderolinkres.NodeUniqueToken) error {
			defer func() {
				res.TypedSpec().Value.Token = pointer.SafeDeref(provisionContext.request.NodeUniqueToken)
			}()

			if res.TypedSpec().Value.Token == "" {
				return nil
			}

			linkNodeUniqueToken, err := jointoken.ParseNodeUniqueToken(res.TypedSpec().Value.Token)
			if err != nil {
				return err
			}

			if linkNodeUniqueToken.Equal(provisionContext.requestNodeUniqueToken) ||
				!provisionContext.forceValidNodeUniqueToken && provisionContext.requestNodeUniqueToken.IsSameFingerprint(linkNodeUniqueToken) {
				logger.Debug("overwrite the existing node unique token")

				return nil
			}

			// the token has the same fingerprint, but the random part doesn't match
			// return the error
			// this case might happen if there is a hardware failure, so that the node
			// has lost it's META partition contents
			if linkNodeUniqueToken.IsSameFingerprint(provisionContext.requestNodeUniqueToken) {
				logger.Warn("machine connection rejected: the machine has the correct fingerprint, but the random token part doesn't match")

				return status.Error(codes.PermissionDenied, "unauthorized")
			}

			return errUUIDConflict
		},
	)
}

func updateResource[T res](ctx context.Context, logger *zap.Logger,
	st state.State, provisionContext *provisionContext, r T, annotationsToAdd []string, annotationsToRemove []string,
) (T, error) {
	return safe.StateUpdateWithConflicts(ctx, st, r.Metadata(), func(link T) error {
		s := link.TypedSpec().Value

		if link.Metadata().Type() == siderolinkres.PendingMachineType {
			link.Metadata().Annotations().Set("timestamp", time.Now().String())
		}

		if provisionContext.pendingMachine != nil {
			s.NodeSubnet = provisionContext.pendingMachine.TypedSpec().Value.NodeSubnet

			logger.Info("updated subnet", zap.String("subnet", s.NodeSubnet))
		}

		var err error

		s.NodePublicKey = provisionContext.request.NodePublicKey

		s.VirtualAddrport, err = generateVirtualAddrPort(provisionContext.useWireguardOverGRPC)
		if err != nil {
			return err
		}

		updateAnnotations(link, annotationsToAdd, annotationsToRemove)

		return nil
	})
}

func (h *ProvisionHandler) provision(ctx context.Context, provisionContext *provisionContext) (*pb.ProvisionResponse, error) {
	logger := h.logger.With(
		zap.String("machine", provisionContext.request.NodeUuid),
		zap.Bool("node_unique_tokens_enabled", provisionContext.nodeUniqueTokensEnabled),
		zap.Bool("supports_secure_join_tokens", provisionContext.supportsSecureJoinTokens),
		zap.Bool("has_valid_join_token", provisionContext.hasValidJoinToken),
		zap.Bool("has_link_node_unique_token", provisionContext.linkNodeUniqueToken != nil),
	)

	// legacy flow, let it join unconditionally
	if !provisionContext.nodeUniqueTokensEnabled || !provisionContext.supportsSecureJoinTokens {
		if !provisionContext.isAuthorizedLegacyJoin() {
			logger.Warn("machine is not allowed to join using legacy join flow")

			return nil, status.Error(codes.PermissionDenied, "unauthorized")
		}

		return establishLink[*siderolinkres.Link](ctx, h.logger, h.state, provisionContext, nil, nil)
	}

	if !provisionContext.isAuthorizedSecureFlow() {
		logger.Warn("machine is not allowed to join using secure join flow")

		return nil, status.Error(codes.PermissionDenied, "unauthorized")
	}

	// if the token is not generated and the node supports secure join tokens
	// put the machine into the limbo state by creating the pending machine resource
	// the controller will then pick it up and create a wireguard peer for it
	if provisionContext.requestNodeUniqueToken == nil {
		return establishLink[*siderolinkres.PendingMachine](ctx, h.logger, h.state, provisionContext, nil, nil)
	}

	annotationsToAdd := []string{}
	annotationsToRemove := []string{}

	switch {
	// the token was wiped, reset the link annotation until Talos gets installed again
	case provisionContext.tokenWasWiped:
		annotationsToRemove = append(annotationsToRemove, siderolinkres.ForceValidNodeUniqueToken)
	// if we detected that Talos installed during provision
	// mark the link with the annotation to block using the link without the unique node token
	case provisionContext.forceValidNodeUniqueToken:
		annotationsToAdd = append(annotationsToAdd, siderolinkres.ForceValidNodeUniqueToken)
	}

	response, err := establishLink[*siderolinkres.Link](ctx, h.logger, h.state, provisionContext, annotationsToAdd, annotationsToRemove)
	if err != nil {
		if errors.Is(err, errUUIDConflict) {
			logger.Info("detected UUID conflict", zap.String("peer", provisionContext.request.NodePublicKey))

			// link is there, but the token doesn't match and the fingerprint differs, keep the machine in the limbo state
			// mark pending machine as having the UUID conflict, PendingMachineStatus controller should inject the new UUID
			// and the machine will re-join
			return establishLink[*siderolinkres.PendingMachine](ctx, h.logger, h.state, provisionContext, []string{siderolinkres.PendingMachineUUIDConflict}, nil)
		}

		return nil, err
	}

	return response, nil
}

func (h *ProvisionHandler) removePendingMachine(ctx context.Context, pendingMachine *siderolinkres.PendingMachine) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	ready, err := h.state.Teardown(ctx, pendingMachine.Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return nil
	}

	if !ready {
		return nil
	}

	err = h.state.Destroy(ctx, pendingMachine.Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	h.logger.Info("cleaned up the pending machine link after grace period", zap.String("id", pendingMachine.Metadata().ID()))

	return nil
}

func establishLink[T res](ctx context.Context, logger *zap.Logger, st state.State, provisionContext *provisionContext,
	annotationsToAdd []string, annotationsToRemove []string,
) (*pb.ProvisionResponse, error) {
	link, err := newLink[T](provisionContext, annotationsToAdd, annotationsToRemove)
	if err != nil {
		return nil, err
	}

	if provisionContext.requestNodeUniqueToken != nil {
		link.Metadata().Annotations().Set(omni.CreatedWithUniqueToken, "")

		if link.Metadata().Type() == siderolinkres.LinkType {
			if err = updateNodeUniqueToken(ctx, logger, st, provisionContext); err != nil {
				return nil, err
			}
		}
	}

	if err = st.Create(ctx, link); err != nil {
		if !state.IsConflictError(err) {
			return nil, err
		}

		link, err = updateResource(ctx, logger, st, provisionContext, link, annotationsToAdd, annotationsToRemove)
		if err != nil {
			if state.IsPhaseConflictError(err) {
				return nil, status.Errorf(codes.AlreadyExists, "the machine with the same UUID is already registered in Omni and is in the tearing down phase")
			}

			return nil, err
		}
	}

	if link.Metadata().Type() == siderolinkres.LinkType {
		err = safe.StateModify(ctx, st, siderolinkres.NewJoinTokenUsage(link.Metadata().ID()), func(res *siderolinkres.JoinTokenUsage) error {
			res.TypedSpec().Value.TokenId = pointer.SafeDeref(provisionContext.request.JoinToken)

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return genProvisionResponse(ctx, logger, st, provisionContext, link, link.TypedSpec().Value)
}

func genProvisionResponse(ctx context.Context, logger *zap.Logger, st state.State,
	provisionContext *provisionContext, link resource.Resource, spec *specs.SiderolinkSpec,
) (*pb.ProvisionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	logger.Debug("waiting for the Wireguard peer to be created", zap.String("link", link.Metadata().String()))

	_, err := st.WatchFor(ctx,
		siderolinkres.NewLinkStatus(link).Metadata(),
		state.WithPhases(resource.PhaseRunning),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			return !resource.IsTombstone(r), nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("timed out waiting for the wireguard peer to be created on Omni side %w", err)
	}

	wgConfig := provisionContext.siderolinkConfig.TypedSpec().Value

	endpoint := wgConfig.WireguardEndpoint
	if wgConfig.AdvertisedEndpoint != "" {
		endpoint = wgConfig.AdvertisedEndpoint
	}

	// If the virtual address is set, use it as the endpoint to prevent the client from connecting to the actual WG endpoint
	if spec.VirtualAddrport != "" {
		endpoint = spec.VirtualAddrport
	}

	endpoints := strings.Split(endpoint, ",")

	logger.Debug("generated response",
		zap.String("node_address", spec.NodeSubnet),
		zap.String("public_key", wgConfig.PublicKey),
		zap.String("grpc_addr_port", spec.VirtualAddrport),
	)

	return &pb.ProvisionResponse{
		ServerEndpoint:    pb.MakeEndpoints(endpoints...),
		ServerPublicKey:   wgConfig.PublicKey,
		NodeAddressPrefix: spec.NodeSubnet,
		ServerAddress:     wgConfig.ServerAddress,
		GrpcPeerAddrPort:  spec.VirtualAddrport,
	}, nil
}

func generateLinkSpec(provisionContext *provisionContext) (*specs.SiderolinkSpec, error) {
	nodePrefix := netip.MustParsePrefix(provisionContext.siderolinkConfig.TypedSpec().Value.Subnet)

	var nodeAddress string

	switch {
	case provisionContext.link != nil:
		nodeAddress = provisionContext.link.TypedSpec().Value.NodeSubnet
	case provisionContext.pendingMachine != nil:
		nodeAddress = provisionContext.pendingMachine.TypedSpec().Value.NodeSubnet
	default:
		// generated random address for the node
		addr, err := wireguard.GenerateRandomNodeAddr(nodePrefix)
		if err != nil {
			return nil, fmt.Errorf("error generating random node address: %w", err)
		}

		nodeAddress = addr.String()
	}

	pubKey, err := wgtypes.ParseKey(provisionContext.request.NodePublicKey)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("error parsing Wireguard key: %s", err))
	}

	virtualAddrPort, err := generateVirtualAddrPort(provisionContext.useWireguardOverGRPC)
	if err != nil {
		return nil, err
	}

	return &specs.SiderolinkSpec{
		NodeSubnet:      nodeAddress,
		NodePublicKey:   pubKey.String(),
		VirtualAddrport: virtualAddrPort,
		Connected:       true,
	}, nil
}

func generateVirtualAddrPort(generate bool) (string, error) {
	if !generate {
		return "", nil
	}

	generated, err := wireguard.GenerateRandomNodeAddr(wireguard.VirtualNetworkPrefix())
	if err != nil {
		return "", fmt.Errorf("error generating random virtual node address: %w", err)
	}

	return net.JoinHostPort(generated.Addr().String(), "50889"), nil
}

//nolint:gocyclo,cyclop
func (h *ProvisionHandler) buildProvisionContext(ctx context.Context, req *pb.ProvisionRequest) (*provisionContext, error) {
	link, err := safe.StateGetByID[*siderolinkres.Link](ctx, h.state, req.NodeUuid)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	siderolinkConfig, err := safe.ReaderGetByID[*siderolinkres.Config](ctx, h.state, siderolinkres.ConfigID)
	if err != nil {
		return nil, err
	}

	var (
		requestJoinToken       *jointoken.JoinToken
		requestNodeUniqueToken *jointoken.NodeUniqueToken
		linkNodeUniqueToken    *jointoken.NodeUniqueToken
		pendingMachineStatus   *siderolinkres.PendingMachineStatus
		nodeUniqueToken        *siderolinkres.NodeUniqueToken
		forceValidUniqueToken  bool
		tokenWasWiped          bool
	)

	nodeUniqueToken, err = safe.ReaderGetByID[*siderolinkres.NodeUniqueToken](ctx, h.state, req.NodeUuid)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	requestJoinToken, err = h.getJoinToken(ctx, pointer.SafeDeref(req.JoinToken))
	if err != nil {
		return nil, err
	}

	talosVersion := "1.5.0"

	if req.TalosVersion != nil {
		talosVersion = *req.TalosVersion
	}

	pendingMachine, err := safe.ReaderGetByID[*siderolinkres.PendingMachine](ctx, h.state, req.NodePublicKey)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	if uniqueToken := pointer.SafeDeref(req.NodeUniqueToken); uniqueToken != "" {
		requestNodeUniqueToken, err = jointoken.ParseNodeUniqueToken(uniqueToken)
		if err != nil {
			return nil, err
		}
	}

	if pendingMachine != nil {
		pendingMachineStatus, err = safe.StateWatchFor[*siderolinkres.PendingMachineStatus](ctx,
			h.state,
			siderolinkres.NewPendingMachineStatus(pendingMachine.Metadata().ID()).Metadata(),
			state.WithPhases(resource.PhaseRunning),
			state.WithCondition(func(r resource.Resource) (bool, error) {
				return !resource.IsTombstone(r), nil
			}),
		)
		if err != nil {
			return nil, err
		}

		forceValidUniqueToken = pendingMachineStatus.TypedSpec().Value.TalosInstalled
	}

	if nodeUniqueToken != nil {
		if link != nil {
			_, forceValidUniqueToken = link.Metadata().Annotations().Get(siderolinkres.ForceValidNodeUniqueToken)
		}

		linkNodeUniqueToken, err = jointoken.ParseNodeUniqueToken(nodeUniqueToken.TypedSpec().Value.Token)
		if err != nil {
			return nil, err
		}

		var machineStatus *infra.MachineStatus

		machineStatus, err = safe.ReaderGetByID[*infra.MachineStatus](ctx, h.state, req.NodeUuid)
		if err != nil && !state.IsNotFoundError(err) {
			return nil, err
		}

		if machineStatus != nil && nodeUniqueToken.TypedSpec().Value.Token == machineStatus.TypedSpec().Value.WipedNodeUniqueToken {
			forceValidUniqueToken = false

			tokenWasWiped = true
		}
	}

	return &provisionContext{
		siderolinkConfig:          siderolinkConfig,
		link:                      link,
		pendingMachine:            pendingMachine,
		pendingMachineStatus:      pendingMachineStatus,
		token:                     requestJoinToken,
		request:                   req,
		requestNodeUniqueToken:    requestNodeUniqueToken,
		linkNodeUniqueToken:       linkNodeUniqueToken,
		forceValidNodeUniqueToken: forceValidUniqueToken,
		tokenWasWiped:             tokenWasWiped,
		hasValidJoinToken:         requestJoinToken != nil,
		hasValidNodeUniqueToken:   linkNodeUniqueToken.Equal(requestNodeUniqueToken),
		nodeUniqueTokensEnabled:   h.joinTokenMode != config.JoinTokensModeLegacyOnly,
		supportsSecureJoinTokens:  siderolink.SupportsSecureJoinTokens(talosVersion),
		useWireguardOverGRPC:      h.forceWireguardOverGRPC || pointer.SafeDeref(req.WireguardOverGrpc),
	}, nil
}

func (h *ProvisionHandler) validateTokenWithExtraData(ctx context.Context, linkToken jointoken.JoinToken) (*jointoken.JoinToken, error) {
	var joinToken string

	// if the token version is V2, we should validate against the individual join token
	if providerID, ok := linkToken.ExtraData[omni.LabelInfraProviderID]; ok && linkToken.Version == jointoken.Version2 {
		providerJoinConfig, err := safe.ReaderGetByID[*siderolinkres.ProviderJoinConfig](ctx, h.state, providerID)
		if err != nil {
			if state.IsNotFoundError(err) {
				h.logger.Warn("machine join token rejected: the provider is not registered in the system", zap.Error(err))

				return nil, nil //nolint:nilnil
			}

			return nil, err
		}

		joinToken = providerJoinConfig.TypedSpec().Value.JoinToken
	} else {
		defaultJoinToken, err := safe.ReaderGetByID[*siderolinkres.DefaultJoinToken](ctx, h.state, siderolinkres.DefaultJoinTokenID)
		if err != nil {
			return nil, err
		}

		var joinTokenStatus *siderolinkres.JoinTokenStatus

		joinTokenStatus, err = safe.ReaderGetByID[*siderolinkres.JoinTokenStatus](ctx, h.state, defaultJoinToken.TypedSpec().Value.TokenId)
		if err != nil {
			return nil, err
		}

		if joinTokenStatus.TypedSpec().Value.State != specs.JoinTokenStatusSpec_ACTIVE {
			h.logger.Warn("machine join token rejected: the default join token is not active")

			return nil, nil //nolint:nilnil
		}

		joinToken = joinTokenStatus.Metadata().ID()
	}

	if !linkToken.IsValid(joinToken) {
		return nil, nil //nolint:nilnil
	}

	return &linkToken, nil
}

func (h *ProvisionHandler) getJoinToken(ctx context.Context, tokenString string) (*jointoken.JoinToken, error) {
	if tokenString == "" {
		return nil, nil //nolint:nilnil
	}

	var linkToken jointoken.JoinToken

	linkToken, err := jointoken.Parse(tokenString)
	if err != nil {
		h.logger.Warn("machine join token rejected: invalid join token", zap.Error(err))

		return nil, status.Errorf(codes.PermissionDenied, "invalid join token %s", err)
	}

	// verify the token against the default token or provider token if using v1 version
	if linkToken.Version != jointoken.VersionPlain {
		return h.validateTokenWithExtraData(ctx, linkToken)
	}

	var tokenStatus *siderolinkres.JoinTokenStatus

	tokenStatus, err = safe.ReaderGetByID[*siderolinkres.JoinTokenStatus](ctx, h.state, tokenString)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	if tokenStatus == nil || tokenStatus.TypedSpec().Value.State != specs.JoinTokenStatusSpec_ACTIVE {
		return nil, nil //nolint:nilnil
	}

	return &linkToken, nil
}
