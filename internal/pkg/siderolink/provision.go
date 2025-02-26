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

	"github.com/blang/semver/v4"
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
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/config"
)

var minSupportedSecureTokensVersion = semver.MustParse("1.6.0")

var errUUIDConflict = fmt.Errorf("UUID conflict")

type provisionContext struct {
	siderolinkConfig       *siderolink.Config
	link                   *siderolink.Link
	pendingMachine         *siderolink.PendingMachine
	pendingMachineStatus   *siderolink.PendingMachineStatus
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
func NewProvisionHandler(logger *zap.Logger, state state.State, joinTokenMode config.JoinTokensMode) *ProvisionHandler {
	return &ProvisionHandler{
		logger:        logger,
		state:         state,
		joinTokenMode: joinTokenMode,
	}
}

// ProvisionHandler is the gRPC service that handles provision responses coming from the Talos nodes.
type ProvisionHandler struct {
	pb.UnimplementedProvisionServiceServer
	logger        *zap.Logger
	state         state.State
	joinTokenMode config.JoinTokensMode
}

func (h *ProvisionHandler) runCleanup(ctx context.Context) error {
	ticker := time.NewTicker(time.Second * 30)

	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			pendingMachines, err := safe.ReaderListAll[*siderolink.PendingMachine](ctx, h.state)
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

// createResource creates the link resource (PendingMachine/Link) if it doesn't exist.
func createResource[T res](ctx context.Context, st state.State, provisionContext *provisionContext,
	annotations ...string,
) (T, error) {
	var (
		zero T
		id   string
	)

	rd := zero.ResourceDefinition()

	if rd.Type == siderolink.LinkType {
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
	}

	link.TypedSpec().Value, err = generateLinkSpec(provisionContext)
	if err != nil {
		return zero, err
	}

	if link.Metadata().Type() == siderolink.PendingMachineType {
		link.Metadata().Labels().Set(omni.MachineUUID, provisionContext.request.NodeUuid)
	}

	for _, annotation := range annotations {
		link.Metadata().Annotations().Set(annotation, "")
	}

	return link, st.Create(ctx, res)
}

func updateResourceWithMatchingToken[T res](ctx context.Context, logger *zap.Logger,
	st state.State, provisionContext *provisionContext, r T, annotations ...string,
) (T, error) {
	return safe.StateUpdateWithConflicts(ctx, st, r.Metadata(), func(link T) error {
		s := link.TypedSpec().Value

		if link.Metadata().Type() == siderolink.PendingMachineType {
			link.Metadata().Annotations().Set("timestamp", time.Now().String())
		}

		updateSpec := func() error {
			s.NodeUniqueToken = pointer.SafeDeref(provisionContext.request.NodeUniqueToken)

			if provisionContext.pendingMachine != nil {
				s.NodeSubnet = provisionContext.pendingMachine.TypedSpec().Value.NodeSubnet

				logger.Info("updated subnet", zap.String("subnet", s.NodeSubnet))
			}

			var err error

			s.NodePublicKey = provisionContext.request.NodePublicKey
			s.VirtualAddrport, err = generateVirtualAddrPort(pointer.SafeDeref(provisionContext.request.WireguardOverGrpc))

			for _, annotation := range annotations {
				link.Metadata().Annotations().Set(annotation, "")
			}

			return err
		}

		if s.NodeUniqueToken == "" {
			logger.Debug("set unique node token")

			return updateSpec()
		}

		linkNodeUniqueToken, err := jointoken.ParseNodeUniqueToken(s.NodeUniqueToken)
		if err != nil {
			return err
		}

		if linkNodeUniqueToken.Equal(provisionContext.requestNodeUniqueToken) ||
			!provisionContext.forceValidNodeUniqueToken && provisionContext.requestNodeUniqueToken.IsSameFingerprint(linkNodeUniqueToken) {
			logger.Debug("overwrite the existing node unique token")

			return updateSpec()
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
	})
}

func (h *ProvisionHandler) provision(ctx context.Context, provisionContext *provisionContext) (*pb.ProvisionResponse, error) {
	logger := h.logger.With(zap.String("machine", provisionContext.request.NodeUuid))

	// legacy flow, let it join unconditionally
	if !provisionContext.nodeUniqueTokensEnabled || !provisionContext.supportsSecureJoinTokens {
		if !provisionContext.isAuthorizedLegacyJoin() {
			return nil, status.Error(codes.PermissionDenied, "unauthorized")
		}

		return establishLink[*siderolink.Link](ctx, h.logger, h.state, provisionContext)
	}

	if !provisionContext.isAuthorizedSecureFlow() {
		return nil, status.Error(codes.PermissionDenied, "unauthorized")
	}

	// if the token is not generated and the node supports secure join tokens
	// put the machine into the limbo state by creating the pending machine resource
	// the controller will then pick it up and create a wireguard peer for it
	if provisionContext.requestNodeUniqueToken == nil {
		return establishLink[*siderolink.PendingMachine](ctx, h.logger, h.state, provisionContext)
	}

	annotations := []string{}

	// if we detected that Talos installed during provision
	// mark the link with the annotation to block using the link without the unique node token
	if provisionContext.forceValidNodeUniqueToken {
		annotations = append(annotations, siderolink.ForceValidNodeUniqueToken)
	}

	response, err := establishLink[*siderolink.Link](ctx, h.logger, h.state, provisionContext, annotations...)
	if err != nil {
		if errors.Is(err, errUUIDConflict) {
			logger.Info("detected UUID conflict", zap.String("peer", provisionContext.request.NodePublicKey))

			// link is there, but the token doesn't match and the fingerprint differs, keep the machine in the limbo state
			// mark pending machine as having the UUID conflict, PendingMachineStatus controller should inject the new UUID
			// and the machine will re-join
			return establishLink[*siderolink.PendingMachine](ctx, h.logger, h.state, provisionContext, siderolink.PendingMachineUUIDConflict)
		}

		return nil, err
	}

	return response, nil
}

func (h *ProvisionHandler) removePendingMachine(ctx context.Context, pendingMachine *siderolink.PendingMachine) error {
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

func establishLink[T res](ctx context.Context, logger *zap.Logger, st state.State, provisionContext *provisionContext, annotations ...string) (*pb.ProvisionResponse, error) {
	link, err := createResource[T](ctx, st, provisionContext, annotations...)
	if err != nil {
		if !state.IsConflictError(err) {
			return nil, err
		}

		link, err = updateResourceWithMatchingToken[T](ctx, logger, st, provisionContext, link, annotations...)
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
		siderolink.NewLinkStatus(link).Metadata(),
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

	virtualAddrPort, err := generateVirtualAddrPort(pointer.SafeDeref(provisionContext.request.WireguardOverGrpc))
	if err != nil {
		return nil, err
	}

	return &specs.SiderolinkSpec{
		NodeSubnet:      nodeAddress,
		NodePublicKey:   pubKey.String(),
		VirtualAddrport: virtualAddrPort,
		NodeUniqueToken: pointer.SafeDeref(provisionContext.request.NodeUniqueToken),
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
	link, err := safe.StateGetByID[*siderolink.Link](ctx, h.state, req.NodeUuid)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	// TODO: add support of several join tokens here
	siderolinkConfig, err := safe.ReaderGetByID[*siderolink.Config](ctx, h.state, siderolink.ConfigID)
	if err != nil {
		return nil, err
	}

	var (
		linkToken              *jointoken.JoinToken
		requestNodeUniqueToken *jointoken.NodeUniqueToken
		linkNodeUniqueToken    *jointoken.NodeUniqueToken
		pendingMachineStatus   *siderolink.PendingMachineStatus
		forceValidUniqueToken  bool
	)

	if req.JoinToken != nil {
		linkToken, err = h.getJoinToken(*req.JoinToken)
		if err != nil {
			return nil, err
		}
	}

	// if the version is not set, consider the machine be below 1.6
	talosVersion := semver.MustParse("1.5.0")

	if pointer.SafeDeref(req.TalosVersion) != "" {
		talosVersion, err = semver.ParseTolerant(*req.TalosVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Talos version %q from the provision request %w", *req.TalosVersion, err)
		}
	}

	pendingMachine, err := safe.ReaderGetByID[*siderolink.PendingMachine](ctx, h.state, req.NodePublicKey)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	if uniqueToken := pointer.SafeDeref(req.NodeUniqueToken); uniqueToken != "" {
		requestNodeUniqueToken, err = jointoken.ParseNodeUniqueToken(uniqueToken)
		if err != nil {
			return nil, err
		}
	}

	if link != nil {
		_, forceValidUniqueToken = link.Metadata().Annotations().Get(siderolink.ForceValidNodeUniqueToken)

		linkNodeUniqueToken, err = jointoken.ParseNodeUniqueToken(link.TypedSpec().Value.NodeUniqueToken)
		if err != nil {
			return nil, err
		}
	}

	machineStatus, err := safe.ReaderGetByID[*infra.MachineStatus](ctx, h.state, req.NodeUuid)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	if machineStatus != nil && link != nil {
		forceValidUniqueToken = link.TypedSpec().Value.NodeUniqueToken != machineStatus.TypedSpec().Value.WipedNodeUniqueToken
	}

	if pendingMachine != nil {
		pendingMachineStatus, err = safe.StateWatchFor[*siderolink.PendingMachineStatus](ctx,
			h.state,
			siderolink.NewPendingMachineStatus(pendingMachine.Metadata().ID()).Metadata(),
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

	supportsSecureJoinTokens := talosVersion.GTE(minSupportedSecureTokensVersion)

	return &provisionContext{
		siderolinkConfig:          siderolinkConfig,
		link:                      link,
		pendingMachine:            pendingMachine,
		pendingMachineStatus:      pendingMachineStatus,
		token:                     linkToken,
		request:                   req,
		requestNodeUniqueToken:    requestNodeUniqueToken,
		linkNodeUniqueToken:       linkNodeUniqueToken,
		forceValidNodeUniqueToken: forceValidUniqueToken,
		hasValidJoinToken:         linkToken != nil && linkToken.IsValid(siderolinkConfig.TypedSpec().Value.JoinToken),
		hasValidNodeUniqueToken:   linkNodeUniqueToken.Equal(requestNodeUniqueToken),
		nodeUniqueTokensEnabled:   h.joinTokenMode != config.JoinTokensModeLegacyOnly,
		supportsSecureJoinTokens:  supportsSecureJoinTokens,
	}, nil
}

func (h *ProvisionHandler) getJoinToken(tokenString string) (*jointoken.JoinToken, error) {
	var token jointoken.JoinToken

	token, err := jointoken.Parse(tokenString)
	if err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "invalid join token %s", err)
	}

	return &token, nil
}
