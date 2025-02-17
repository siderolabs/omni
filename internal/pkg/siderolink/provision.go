// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink

import (
	"context"
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
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

var minSupportedSecureTokensVersion = semver.MustParse("1.6.0")

type provisionContext struct {
	siderolinkConfig *siderolink.Config
	link             *siderolink.Link
	pendingMachine   *siderolink.PendingMachine
	token            *jointoken.JoinToken
	request          *pb.ProvisionRequest

	// flags
	hasValidJoinToken        bool
	hasValidNodeUniqueToken  bool
	supportsSecureJoinTokens bool
}

// isAuthorized returns true if the request has valid creds for at least one of the auth flows.
func (pc *provisionContext) isAuthorized() bool {
	// explicitly return unauthorized if the machine has the unique token
	// and it doesn't match with what is stored in the link resource
	if pc.link != nil &&
		pointer.SafeDeref(pc.request.NodeUniqueToken) != "" &&
		pc.link.TypedSpec().Value.NodeUniqueToken != "" &&
		*pc.request.NodeUniqueToken != pc.link.TypedSpec().Value.NodeUniqueToken {
		return false
	}

	// TODO: can add tokenless flow here
	return pc.hasValidJoinToken || pc.hasValidNodeUniqueToken
}

// NewProvisionHandler creates a new ProvisionHandler.
func NewProvisionHandler(logger *zap.Logger, state state.State, disableLegacyJoinTokens bool) *ProvisionHandler {
	return &ProvisionHandler{
		logger:                  logger,
		state:                   state,
		disableLegacyJoinTokens: disableLegacyJoinTokens,
	}
}

// ProvisionHandler is the gRPC service that handles provision responses coming from the Talos nodes.
type ProvisionHandler struct {
	pb.UnimplementedProvisionServiceServer
	logger                  *zap.Logger
	state                   state.State
	disableLegacyJoinTokens bool
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

	if !provisionContext.isAuthorized() {
		return nil, status.Error(codes.PermissionDenied, "unauthorized")
	}

	if !provisionContext.supportsSecureJoinTokens && h.disableLegacyJoinTokens {
		return nil, status.Errorf(
			codes.FailedPrecondition,
			"Talos version %s is not supported on this Omni instance as '--disable-legacy-join-tokens' is set",
			pointer.SafeDeref(req.TalosVersion),
		)
	}

	return h.updateLink(ctx, provisionContext)
}

type res interface {
	generic.ResourceWithRD
	TypedSpec() *protobuf.ResourceSpec[specs.SiderolinkSpec, *specs.SiderolinkSpec]
}

func ensureResource[T res](ctx context.Context, st state.State, provisionContext *provisionContext) (T, error) {
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

	r, err := safe.ReaderGetByID[T](ctx, st, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			var res resource.Resource

			res, err = protobuf.CreateResource(rd.Type)
			if err != nil {
				return zero, err
			}

			*res.Metadata() = resource.NewMetadata(rd.DefaultNamespace, rd.Type, id, resource.VersionUndefined)

			link, ok := res.(T)
			if !ok {
				return zero, fmt.Errorf("incorrect resource type")
			}

			if value, ok := provisionContext.token.ExtraData[omni.LabelInfraProviderID]; ok {
				link.Metadata().Annotations().Set(omni.LabelInfraProviderID, value)
			}

			link.TypedSpec().Value, err = generateLinkSpec(provisionContext)
			if err != nil {
				return zero, err
			}

			if link.Metadata().Type() == siderolink.PendingMachineType {
				link.Metadata().Annotations().Set("timestamp", time.Now().String())
				link.Metadata().Labels().Set(omni.MachineUUID, provisionContext.request.NodeUuid)
			}

			return link, st.Create(ctx, res)
		}

		return zero, err
	}

	return r, nil
}

func (h *ProvisionHandler) updateLink(ctx context.Context, provisionContext *provisionContext) (*pb.ProvisionResponse, error) {
	// if the token is not generated and the node supports secure join tokens
	// put the machine into the limbo state by creating the pending machine resource
	// the controller will then pick it up and create a wireguard peer for it
	if pointer.SafeDeref(provisionContext.request.NodeUniqueToken) == "" && provisionContext.supportsSecureJoinTokens {
		link, err := ensureResource[*siderolink.PendingMachine](ctx, h.state, provisionContext)
		if err != nil {
			return nil, err
		}

		if provisionContext.link == nil {
			return h.genProvisionResponse(ctx, provisionContext, link, link.TypedSpec().Value)
		}
	}

	link, err := ensureResource[*siderolink.Link](ctx, h.state, provisionContext)
	if err != nil {
		return nil, err
	}

	if link, err = safe.StateUpdateWithConflicts(ctx, h.state, link.Metadata(), func(r *siderolink.Link) error {
		s := r.TypedSpec().Value

		s.NodePublicKey = provisionContext.request.NodePublicKey
		s.VirtualAddrport, err = generateVirtualAddrPort(pointer.SafeDeref(provisionContext.request.WireguardOverGrpc))
		if err != nil {
			return err
		}

		if s.NodeUniqueToken == "" && provisionContext.request.NodeUniqueToken != nil {
			s.NodeUniqueToken = *provisionContext.request.NodeUniqueToken
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return h.genProvisionResponse(ctx, provisionContext, link, link.TypedSpec().Value)
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

func (h *ProvisionHandler) genProvisionResponse(ctx context.Context, provisionContext *provisionContext, link resource.Resource, spec *specs.SiderolinkSpec) (*pb.ProvisionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	h.logger.Debug("waiting for the Wireguard peer to be created", zap.String("link", link.Metadata().String()))

	_, err := h.state.WatchFor(ctx,
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

func (h *ProvisionHandler) buildProvisionContext(ctx context.Context, req *pb.ProvisionRequest) (*provisionContext, error) {
	link, err := safe.StateGetByID[*siderolink.Link](ctx, h.state, req.NodeUuid)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	// if the node unique token is empty in the incoming request and not empty in the existing link
	// treat the link as not existing for this machine
	// this is necessary for the correct UUID conflict resolution
	if link != nil && link.TypedSpec().Value.NodeUniqueToken != "" && pointer.SafeDeref(req.NodeUniqueToken) == "" {
		link = nil
	}

	// TODO: add support of several join tokens here
	siderolinkConfig, err := safe.ReaderGetByID[*siderolink.Config](ctx, h.state, siderolink.ConfigID)
	if err != nil {
		return nil, err
	}

	var token *jointoken.JoinToken

	if req.JoinToken != nil {
		token, err = h.getJoinToken(*req.JoinToken)
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

	return &provisionContext{
		siderolinkConfig:         siderolinkConfig,
		link:                     link,
		pendingMachine:           pendingMachine,
		token:                    token,
		request:                  req,
		hasValidJoinToken:        token != nil && token.IsValid(siderolinkConfig.TypedSpec().Value.JoinToken),
		hasValidNodeUniqueToken:  link != nil && req.NodeUniqueToken != nil && link.TypedSpec().Value.NodeUniqueToken == *req.NodeUniqueToken,
		supportsSecureJoinTokens: talosVersion.GTE(minSupportedSecureTokensVersion),
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
