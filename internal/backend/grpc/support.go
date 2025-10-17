// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-talos-support/support"
	"github.com/siderolabs/go-talos-support/support/bundle"
	"github.com/siderolabs/go-talos-support/support/collectors"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/kubernetes"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/runtime"
	kubernetesruntime "github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	slink "github.com/siderolabs/omni/internal/pkg/siderolink"
)

func (s *managementServer) GetSupportBundle(req *management.GetSupportBundleRequest, serv grpc.ServerStreamingServer[management.GetSupportBundleResponse]) error {
	if _, err := auth.CheckGRPC(serv.Context(), auth.WithRole(role.Operator)); err != nil {
		return err
	}

	resources, err := s.collectClusterResources(serv.Context(), req.Cluster)
	if err != nil {
		return err
	}

	progress := make(chan bundle.Progress)

	var b bytes.Buffer

	f := bufio.NewWriter(&b)

	cols := make([]*collectors.Collector, 0, len(resources))

	var nodes []string

	for _, res := range resources {
		if res.Metadata().Type() == siderolink.LinkType {
			cols = append(cols, s.collectLogs(res.Metadata().ID()))

			info := s.dnsService.Resolve(req.Cluster, res.Metadata().ID())
			if info.GetAddress() != "" {
				nodes = append(nodes, info.GetAddress())
			}
		}

		cols = append(cols, s.writeResource(res))

		cols = collectors.WithSource(cols, "omni")
	}

	ctx := actor.MarkContextAsInternalActor(serv.Context())

	talosClient, err := s.getTalosClient(ctx, req.Cluster)
	if err != nil {
		if err = serv.Send(&management.GetSupportBundleResponse{
			Progress: &management.GetSupportBundleResponse_Progress{
				Source: collectors.Cluster,
				Error:  fmt.Sprintf("failed to get talos client %s", err.Error()),
			},
		}); err != nil {
			return err
		}
	}

	if talosClient != nil {
		defer talosClient.Close() //nolint:errcheck
	}

	kubernetesClient, err := s.getKubernetesClient(ctx, req.Cluster)
	if err != nil {
		if err = serv.Send(&management.GetSupportBundleResponse{
			Progress: &management.GetSupportBundleResponse_Progress{
				Source: collectors.Cluster,
				Error:  fmt.Sprintf("failed to get kubernetes client %s", err.Error()),
			},
		}); err != nil {
			return err
		}
	}

	options := bundle.NewOptions(
		bundle.WithArchiveOutput(f),
		bundle.WithKubernetesClient(kubernetesClient),
		bundle.WithTalosClient(talosClient),
		bundle.WithNodes(nodes...),
		bundle.WithNumWorkers(4),
		bundle.WithProgressChan(progress),
		bundle.WithLogOutput(io.Discard),
	)

	talosCollectors, err := collectors.GetForOptions(serv.Context(), options)
	if err != nil {
		if err = serv.Send(&management.GetSupportBundleResponse{
			Progress: &management.GetSupportBundleResponse_Progress{
				Source: collectors.Cluster,
				Error:  err.Error(),
			},
		}); err != nil {
			return err
		}
	}

	cols = append(cols, talosCollectors...)

	eg := panichandler.NewErrGroup()

	eg.Go(func() error {
		for p := range progress {
			var errString string
			if p.Error != nil {
				errString = p.Error.Error()
			}

			if err = serv.Send(&management.GetSupportBundleResponse{
				Progress: &management.GetSupportBundleResponse_Progress{
					Source: p.Source,
					State:  p.State,
					Total:  int32(p.Total),
					Error:  errString,
				},
			}); err != nil {
				return err
			}
		}

		return nil
	})

	eg.Go(func() error {
		defer close(progress)

		return support.CreateSupportBundle(serv.Context(), options, cols...)
	})

	if err = eg.Wait(); err != nil {
		return err
	}

	return serv.Send(&management.GetSupportBundleResponse{
		BundleData: b.Bytes(),
	})
}

func (s *managementServer) writeResource(res resource.Resource) *collectors.Collector {
	filename := fmt.Sprintf("omni/resources/%s-%s.yaml", res.Metadata().Type(), res.Metadata().ID())

	return collectors.NewCollector(filename, func(context.Context, *bundle.Options) ([]byte, error) {
		raw, err := resource.MarshalYAML(res)
		if err != nil {
			return nil, err
		}

		return yaml.Marshal(raw)
	})
}

func (s *managementServer) collectLogs(machineID string) *collectors.Collector {
	filename := fmt.Sprintf("omni/machine-logs/%s.log", machineID)

	return collectors.NewCollector(filename, func(context.Context, *bundle.Options) ([]byte, error) {
		r, err := s.logHandler.GetReader(slink.MachineID(machineID), false, optional.None[int32]())
		if err != nil {
			if slink.IsBufferNotFoundError(err) {
				return []byte{}, nil
			}

			return nil, err
		}

		defer r.Close() //nolint:errcheck

		var b bytes.Buffer

		w := bufio.NewWriter(&b)

		for {
			l, err := r.ReadLine()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return nil, err
			}

			_, err = w.WriteString(string(l) + "\n")
			if err != nil {
				return nil, err
			}
		}

		return b.Bytes(), nil
	})
}

//nolint:gocognit
func (s *managementServer) collectClusterResources(ctx context.Context, cluster string) ([]resource.Resource, error) {
	st := s.omniState

	//nolint:prealloc
	var resources []resource.Resource

	clusterQuery := []state.ListOption{
		state.WithLabelQuery(
			resource.LabelEqual(omni.LabelCluster, cluster),
		),
	}

	resourcesToGet := []struct {
		rt          resource.Type
		id          resource.ID
		listOptions []state.ListOption
	}{
		{
			rt: omni.ClusterType,
			id: cluster,
		},
		{
			rt: omni.ClusterTaintType,
			id: cluster,
		},
		{
			rt: omni.ClusterStatusType,
			id: cluster,
		},
		{
			rt: omni.KubernetesUpgradeStatusType,
			id: cluster,
		},
		{
			rt: omni.TalosUpgradeStatusType,
			id: cluster,
		},
		{
			rt: omni.LoadBalancerStatusType,
			id: cluster,
		},
		{
			rt: omni.KubernetesUpgradeManifestStatusType,
			id: cluster,
		},
		{
			rt: omni.ClusterBootstrapStatusType,
			id: cluster,
		},
		{
			rt:          omni.MachineSetType,
			listOptions: clusterQuery,
		},
		{
			rt:          omni.MachineSetStatusType,
			listOptions: clusterQuery,
		},
		{
			rt:          omni.MachineSetNodeType,
			listOptions: clusterQuery,
		},
		{
			rt:          omni.ClusterMachineType,
			listOptions: clusterQuery,
		},
		{
			rt:          omni.ClusterMachineStatusType,
			listOptions: clusterQuery,
		},
		{
			rt:          omni.RedactedClusterMachineConfigType,
			listOptions: clusterQuery,
		},
		{
			rt:          omni.ClusterMachineConfigStatusType,
			listOptions: clusterQuery,
		},
		{
			rt:          omni.ClusterMachineTalosVersionType,
			listOptions: clusterQuery,
		},
		{
			rt:          omni.ClusterMachineIdentityType,
			listOptions: clusterQuery,
		},
		{
			rt:          omni.SchematicConfigurationType,
			listOptions: clusterQuery,
		},
		{
			rt:          omni.ExtensionsConfigurationType,
			listOptions: clusterQuery,
		},
		{
			rt:          omni.ExposedServiceType,
			listOptions: clusterQuery,
		},
		{
			rt:          omni.DiscoveryAffiliateDeleteTaskType,
			listOptions: clusterQuery,
		},
		{
			rt:          omni.MachineConfigDiffType,
			listOptions: clusterQuery,
		},
		{
			rt:          infra.InfraMachineType,
			listOptions: clusterQuery,
		},
		{
			rt:          infra.InfraMachineStatusType,
			listOptions: clusterQuery,
		},
		{
			rt:          omni.ControlPlaneStatusType,
			listOptions: clusterQuery,
		},
	}

	machineIDs := map[string]struct{}{}

	for _, r := range resourcesToGet {
		rd, err := safe.ReaderGetByID[*meta.ResourceDefinition](ctx, st, strings.ToLower(r.rt))
		if err != nil {
			return nil, err
		}

		md := resource.NewMetadata(rd.TypedSpec().DefaultNamespace, r.rt, r.id, resource.VersionUndefined)

		if md.ID() == "" {
			var list resource.List

			list, err = st.List(ctx, md, r.listOptions...)
			if err != nil {
				return nil, err
			}

			resources = append(resources, list.Items...)

			switch md.Type() {
			case omni.ClusterMachineType:
				fallthrough
			case omni.MachineSetNodeType:
				for _, res := range list.Items {
					machineIDs[res.Metadata().ID()] = struct{}{}
				}
			}

			continue
		}

		res, err := st.Get(ctx, md)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return nil, err
		}

		resources = append(resources, res)
	}

	for id := range machineIDs {
		link, err := safe.ReaderGetByID[*siderolink.Link](ctx, st, id)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return nil, err
		}

		resources = append(resources, link)

		machineStatus, err := safe.ReaderGetByID[*omni.MachineStatus](ctx, st, id)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return nil, err
		}

		resources = append(resources, machineStatus)

		labels, err := safe.ReaderGetByID[*system.ResourceLabels[*omni.MachineStatus]](ctx, st, id)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return nil, err
		}

		resources = append(resources, labels)
	}

	return resources, nil
}

func (s *managementServer) getTalosClient(ctx context.Context, clusterName string) (*client.Client, error) {
	talosConfig, err := safe.ReaderGetByID[*omni.TalosConfig](ctx, s.omniState, clusterName)
	if err != nil {
		return nil, err
	}

	endpoints, err := safe.ReaderGetByID[*omni.ClusterEndpoint](ctx, s.omniState, clusterName)
	if err != nil {
		return nil, err
	}

	return client.New(ctx,
		client.WithCluster(clusterName),
		client.WithConfig(omni.NewTalosClientConfig(talosConfig, endpoints.TypedSpec().Value.ManagementAddresses...)),
	)
}

func (s *managementServer) getKubernetesClient(ctx context.Context, clusterName string) (*kubernetes.Clientset, error) {
	type kubeRuntime interface {
		GetClient(ctx context.Context, cluster string) (*kubernetesruntime.Client, error)
	}

	k8s, err := runtime.LookupInterface[kubeRuntime](kubernetesruntime.Name)
	if err != nil {
		return nil, err
	}

	client, err := k8s.GetClient(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	return client.Clientset(), nil
}
