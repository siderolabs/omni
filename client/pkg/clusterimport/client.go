// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package clusterimport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/image-factory/pkg/constants"
	"github.com/siderolabs/image-factory/pkg/schematic"
	clusterapi "github.com/siderolabs/talos/pkg/machinery/api/cluster"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	clusterres "github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/omni/client/pkg/infra/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// TalosClient is a minimal interface for Talos client used in cluster import.
type TalosClient interface {
	io.Closer
	state.State
	Version(ctx context.Context, callOptions ...grpc.CallOption) (*machineapi.VersionResponse, error)
	ClusterHealthCheck(ctx context.Context, waitTimeout time.Duration, clusterInfo *clusterapi.ClusterInfo) (clusterapi.ClusterService_HealthCheckClient, error)
	ApplyConfiguration(ctx context.Context, req *machineapi.ApplyConfigurationRequest, callOptions ...grpc.CallOption) (*machineapi.ApplyConfigurationResponse, error)
}

// ImageFactoryClient is a minimal interface for Image Factory client used in cluster import.
type ImageFactoryClient interface {
	EnsureSchematic(ctx context.Context, schematic schematic.Schematic) (string, error)
}

type talosClientWrapper struct {
	TalosClient
}

func (c *talosClientWrapper) getUUID(ctx context.Context, node string) (string, error) {
	nodeCtx := talosclient.WithNode(ctx, node)

	resp, err := safe.ReaderGetByID[*hardware.SystemInformation](nodeCtx, c,
		resource.NewMetadata(hardware.NamespaceName, hardware.SystemInformationType, hardware.SystemInformationID, resource.VersionUndefined).ID())
	if err != nil {
		return "", err
	}

	return resp.TypedSpec().UUID, nil
}

func (c *talosClientWrapper) getMembers(ctx context.Context, node string) ([]*clusterres.Member, error) {
	nodeCtx := talosclient.WithNode(ctx, node)

	resp, err := safe.StateList[*clusterres.Member](nodeCtx, c,
		resource.NewMetadata(clusterres.NamespaceName, clusterres.MemberType, "", resource.VersionUndefined))
	if err != nil {
		return nil, err
	}

	return slices.Collect(resp.All()), nil
}

func (c *talosClientWrapper) getHostnameStatus(ctx context.Context, node string) (*network.HostnameStatus, error) {
	nodeCtx := talosclient.WithNode(ctx, node)

	return safe.ReaderGetByID[*network.HostnameStatus](nodeCtx, c,
		resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, network.HostnameID, resource.VersionUndefined).ID())
}

func (c *talosClientWrapper) getMachineConfig(ctx context.Context, node string) (*configres.MachineConfig, error) {
	nodeCtx := talosclient.WithNode(ctx, node)

	return safe.ReaderGetByID[*configres.MachineConfig](nodeCtx, c,
		resource.NewMetadata(configres.NamespaceName, configres.MachineConfigType, configres.ActiveID, resource.VersionUndefined).ID())
}

func (c *talosClientWrapper) getKubeletConfig(ctx context.Context, node string) (*k8s.KubeletConfig, error) {
	nodeCtx := talosclient.WithNode(ctx, node)

	return safe.ReaderGetByID[*k8s.KubeletConfig](nodeCtx, c,
		resource.NewMetadata(k8s.NamespaceName, k8s.KubeletConfigType, k8s.KubeletID, resource.VersionUndefined).ID())
}

func (c *talosClientWrapper) getSchedulerConfig(ctx context.Context, node string) (*k8s.SchedulerConfig, error) {
	nodeCtx := talosclient.WithNode(ctx, node)

	return safe.ReaderGetByID[*k8s.SchedulerConfig](nodeCtx, c,
		resource.NewMetadata(k8s.NamespaceName, k8s.SchedulerConfigType, k8s.SchedulerConfigID, resource.VersionUndefined).ID())
}

func (c *talosClientWrapper) getControllerManagerConfig(ctx context.Context, node string) (*k8s.ControllerManagerConfig, error) {
	nodeCtx := talosclient.WithNode(ctx, node)

	return safe.ReaderGetByID[*k8s.ControllerManagerConfig](nodeCtx, c,
		resource.NewMetadata(k8s.NamespaceName, k8s.ControllerManagerConfigType, k8s.ControllerManagerConfigID, resource.VersionUndefined).ID())
}

func (c *talosClientWrapper) getAPIServerConfig(ctx context.Context, node string) (*k8s.APIServerConfig, error) {
	nodeCtx := talosclient.WithNode(ctx, node)

	return safe.ReaderGetByID[*k8s.APIServerConfig](nodeCtx, c,
		resource.NewMetadata(k8s.NamespaceName, k8s.APIServerConfigType, k8s.APIServerConfigID, resource.VersionUndefined).ID())
}

func (c *talosClientWrapper) getExtensionStatuses(ctx context.Context, node string) ([]*runtime.ExtensionStatus, error) {
	nodeCtx := talosclient.WithNode(ctx, node)

	resp, err := safe.StateList[*runtime.ExtensionStatus](nodeCtx, c,
		resource.NewMetadata(runtime.NamespaceName, runtime.ExtensionStatusType, "", resource.VersionUndefined))
	if err != nil {
		return nil, err
	}

	return slices.Collect(resp.All()), nil
}

func (c *talosClientWrapper) isControlPlane(ctx context.Context, node string) (bool, error) {
	nodeCtx := talosclient.WithNode(ctx, node)

	resp, err := safe.ReaderGetByID[*configres.MachineType](nodeCtx, c,
		resource.NewMetadata(configres.NamespaceName, configres.MachineTypeType, configres.MachineTypeID, resource.VersionUndefined).ID())
	if err != nil {
		return false, err
	}

	return resp.MachineType().IsControlPlane(), nil
}

func (c *talosClientWrapper) getTalosVersion(ctx context.Context, node string) (*machineapi.VersionInfo, error) {
	nodeCtx := talosclient.WithNode(ctx, node)

	resp, err := c.Version(nodeCtx)
	if err != nil {
		return nil, err
	}

	if len(resp.Messages) == 0 {
		return nil, errors.New("no version info returned from node")
	}

	return resp.Messages[0].Version, nil
}

func (c *talosClientWrapper) checkClusterHealth(ctx context.Context, node string, controlPlanes, workers []string, waitTimeout time.Duration, logWriter io.Writer) error {
	nodeCtx := talosclient.WithNode(ctx, node)

	healthCheckClient, err := c.ClusterHealthCheck(nodeCtx, waitTimeout, &clusterapi.ClusterInfo{
		ControlPlaneNodes: controlPlanes,
		WorkerNodes:       workers,
	})
	if err != nil {
		return err
	}

	if err = healthCheckClient.CloseSend(); err != nil {
		return err
	}

	for {
		msg, healthErr := healthCheckClient.Recv()
		if healthErr != nil {
			if errors.Is(healthErr, io.EOF) || talosclient.StatusCode(healthErr) == codes.Canceled {
				return nil
			}

			return healthErr
		}

		if msg.GetMetadata().GetError() != "" {
			return fmt.Errorf("healthcheck error: %s", msg.GetMetadata().GetError())
		}

		fmt.Fprintf(logWriter, " > %s\n", msg.GetMessage()) //nolint:errcheck
	}
}

func (c *talosClientWrapper) getSchematic(extensionStatuses []*runtime.ExtensionStatus) (string, *schematic.Schematic, error) {
	var schematicSpec *runtime.ExtensionStatus

	for _, status := range extensionStatuses {
		if status.TypedSpec().Metadata.Name == constants.SchematicIDExtensionName {
			schematicSpec = status

			break
		}
	}

	if schematicSpec != nil && schematicSpec.TypedSpec().Metadata.Version != "" && schematicSpec.TypedSpec().Metadata.ExtraInfo == "" {
		return schematicSpec.TypedSpec().Metadata.Version, nil, nil
	}

	if schematicSpec == nil || schematicSpec.TypedSpec().Metadata.ExtraInfo == "" {
		return "", nil, nil
	}

	schematicInfo, err := schematic.Unmarshal([]byte(schematicSpec.TypedSpec().Metadata.ExtraInfo))
	if err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal schematic extra info: %w", err)
	}

	return schematicSpec.TypedSpec().Metadata.Version, schematicInfo, nil
}

type talosClient struct {
	state.State
	*talosclient.Client
}

// BuildTalosClient builds a Talos client for the given cluster using the provided parameters.
func BuildTalosClient(ctx context.Context, config, context, sideroV1KeysDir string, endpoints []string) (TalosClient, error) {
	cfg, err := clientconfig.Open(config)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %q: %w", config, err)
	}

	opts := []talosclient.OptionFunc{
		talosclient.WithConfig(cfg),
		talosclient.WithDefaultGRPCDialOptions(),
		talosclient.WithSideroV1KeysDir(clientconfig.CustomSideroV1KeysDirPath(sideroV1KeysDir)),
	}

	if context != "" {
		opts = append(opts, talosclient.WithContextName(context))
	}

	if len(endpoints) > 0 {
		// override endpoints from command-line flags
		opts = append(opts, talosclient.WithEndpoints(endpoints...))
	}

	c, err := talosclient.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("error constructing client: %w", err)
	}

	return &talosClient{
		State:  c.COSI,
		Client: c,
	}, nil
}

// BuildImageFactoryClient builds an Image Factory client using the Image Factory base url from Omni configuration.
func BuildImageFactoryClient(ctx context.Context, omniState state.State) (*imagefactory.Client, error) {
	featuresConfig, err := safe.ReaderGetByID[*omni.FeaturesConfig](ctx, omniState, omni.FeaturesConfigID)
	if err != nil {
		return nil, fmt.Errorf("error reading features config %q: %w", omniState, err)
	}

	if featuresConfig.TypedSpec().Value.ImageFactoryBaseUrl == "" {
		return nil, fmt.Errorf("image factory base URL is empty")
	}

	c, err := imagefactory.NewClient(imagefactory.ClientOptions{FactoryEndpoint: featuresConfig.TypedSpec().Value.ImageFactoryBaseUrl})
	if err != nil {
		return nil, fmt.Errorf("failed to set up image factory client: %w", err)
	}

	return c, nil
}
