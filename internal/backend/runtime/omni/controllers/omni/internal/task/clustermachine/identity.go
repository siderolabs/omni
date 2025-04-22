// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package clustermachine

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-retry/retry"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

// IdentityCollectorTaskSpec scrapes identity, nodename and etcd members info from the nodes.
type IdentityCollectorTaskSpec struct {
	config            *clientconfig.Config
	configVersion     resource.Version
	machineSetName    string
	clusterName       string
	managementAddress string
	id                resource.ID
	isControlPlane    bool
}

// IdentityCollectorChan is a channel for sending cluster machine identity from tasks back to the controller.
type IdentityCollectorChan chan<- *omni.ClusterMachineIdentity

// NewIdentityCollectorTaskSpec creates new ClusterMachineCollector.
func NewIdentityCollectorTaskSpec(id resource.ID, config *clientconfig.Config, configVersion resource.Version, address string, isControlPlane bool,
	clusterName, machineSetName string,
) IdentityCollectorTaskSpec {
	return IdentityCollectorTaskSpec{
		id:                id,
		config:            config,
		configVersion:     configVersion,
		managementAddress: address,
		isControlPlane:    isControlPlane,
		clusterName:       clusterName,
		machineSetName:    machineSetName,
	}
}

// ID returns the ID of the IdentityCollectorTaskSpec.
func (spec IdentityCollectorTaskSpec) ID() string {
	return spec.clusterName + "/" + spec.id
}

// Equal checks if the IdentityCollectorTaskSpec is equal to the other IdentityCollectorTaskSpec.
func (spec IdentityCollectorTaskSpec) Equal(other IdentityCollectorTaskSpec) bool {
	if spec.id != other.id {
		return false
	}

	if spec.managementAddress != other.managementAddress {
		return false
	}

	if spec.isControlPlane != other.isControlPlane {
		return false
	}

	if spec.clusterName != other.clusterName {
		return false
	}

	if !spec.configVersion.Equal(other.configVersion) {
		return false
	}

	return true
}

// RunTask runs the identity collector task.
//
//nolint:gocyclo,cyclop
func (spec IdentityCollectorTaskSpec) RunTask(ctx context.Context, logger *zap.Logger, notify IdentityCollectorChan) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	clusterMachineIdentity := omni.NewClusterMachineIdentity(resources.DefaultNamespace, spec.id)
	clusterMachineIdentity.Metadata().Labels().Set(omni.LabelCluster, spec.clusterName)

	if spec.isControlPlane {
		clusterMachineIdentity.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
	} else {
		clusterMachineIdentity.Metadata().Labels().Set(omni.LabelWorkerRole, "")
	}

	if spec.machineSetName != "" {
		clusterMachineIdentity.Metadata().Labels().Set(omni.LabelMachineSet, spec.machineSetName)
	}

	client, err := spec.getClient(ctx)
	if err != nil {
		return err
	}

	defer client.Close() //nolint:errcheck

	watchCh := make(chan state.Event)

	// Talos 1.3.0+ supports etcd.MemberID resource, so we can use it to get the member ID
	// instead of running a loop which queries etcd directly.
	//
	// TODO: remove once Omni goes Talos 1.3.0+ only.
	runLegacyEtcdMemberIDCollector := false

	if spec.isControlPlane {
		runLegacyEtcdMemberIDCollector, err = spec.shouldRunLegacyEtcdMemberIDCollector(ctx, client)
		if err != nil {
			return err
		}
	}

	watchedResources := []resource.Resource{
		cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity),
		k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID),
		k8s.NewNodeIP(k8s.NamespaceName, k8s.KubeletID),
	}

	if spec.isControlPlane && !runLegacyEtcdMemberIDCollector {
		watchedResources = append(watchedResources, etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID))
	}

	for _, watchedResource := range watchedResources {
		if err = client.COSI.Watch(ctx, watchedResource.Metadata(), watchCh); err != nil {
			return fmt.Errorf("error watching %s: %w", watchedResource.Metadata().ID(), err)
		}
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	if runLegacyEtcdMemberIDCollector {
		wg.Add(1)

		panichandler.Go(func() {
			defer wg.Done()

			spec.runLegacyEtcdMemberIDCollector(ctx, client, watchCh) //nolint:errcheck
		}, logger)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-watchCh:
			switch event.Type {
			case state.Errored:
				return fmt.Errorf("watch failed: %w", event.Error)
			case state.Bootstrapped, state.Destroyed, state.Noop:
				// ignore
			case state.Created, state.Updated:
				switch r := event.Resource.(type) {
				case *cluster.Identity:
					clusterMachineIdentity.TypedSpec().Value.NodeIdentity = r.TypedSpec().NodeID
				case *k8s.Nodename:
					clusterMachineIdentity.TypedSpec().Value.Nodename = r.TypedSpec().Nodename
				case *k8s.NodeIP:
					clusterMachineIdentity.TypedSpec().Value.NodeIps = xslices.Map(
						r.TypedSpec().Addresses,
						netip.Addr.String,
					)
				case *etcd.Member:
					membID, err := etcd.ParseMemberID(r.TypedSpec().MemberID)
					if err != nil {
						return fmt.Errorf("error parsing member ID: %w", err)
					}

					clusterMachineIdentity.TypedSpec().Value.EtcdMemberId = membID
				}

				channel.SendWithContext(ctx, notify, clusterMachineIdentity)
			}
		}
	}
}

func (spec IdentityCollectorTaskSpec) shouldRunLegacyEtcdMemberIDCollector(ctx context.Context, client *client.Client) (bool, error) {
	versionResp, err := client.Version(ctx)
	if err != nil {
		return false, fmt.Errorf("error getting version: %w", err)
	}

	if len(versionResp.Messages) != 1 {
		return false, fmt.Errorf("unexpected number of version messages: %d", len(versionResp.Messages))
	}

	talosVersion := versionResp.Messages[0].Version.Tag
	if parsedVersion, err := semver.ParseTolerant(talosVersion); err == nil {
		if parsedVersion.LT(semver.MustParse("1.3.0")) {
			return true, nil
		}
	}

	return false, nil
}

func (spec IdentityCollectorTaskSpec) runLegacyEtcdMemberIDCollector(ctx context.Context, client *client.Client, watchCh chan state.Event) error {
	return retry.Constant(time.Hour, retry.WithUnits(time.Second*5)).RetryWithContext(ctx, func(ctx context.Context) error {
		memberID, err := spec.getEtcdMemberID(ctx, client)
		if errors.Is(err, context.Canceled) {
			return nil
		}

		if err != nil {
			return retry.ExpectedError(err)
		}

		member := etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID)
		member.TypedSpec().MemberID = etcd.FormatMemberID(memberID)

		channel.SendWithContext(ctx, watchCh, state.Event{
			Type:     state.Created,
			Resource: member,
		})

		return nil
	})
}

func (spec IdentityCollectorTaskSpec) getEtcdMemberID(ctx context.Context, client *client.Client) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)

	defer cancel()

	var etcdClient *clientv3.Client

	etcdClient, err := spec.getEtcdClient(ctx, client)
	if err != nil {
		return 0, err
	}

	defer etcdClient.Close() //nolint:errcheck

	members, err := etcdClient.MemberList(ctx)
	if err != nil {
		return 0, err
	}

	return members.Header.MemberId, nil
}

func (spec IdentityCollectorTaskSpec) getEtcdClient(ctx context.Context, client *client.Client) (*clientv3.Client, error) {
	etcdResource, err := safe.StateGet[*secrets.Etcd](ctx, client.COSI, resource.NewMetadata(secrets.NamespaceName, secrets.EtcdType, secrets.EtcdID, resource.VersionUndefined))
	if err != nil {
		return nil, err
	}

	etcd := etcdResource.TypedSpec()

	caCertPool := x509.NewCertPool()

	serverCert, err := etcd.Etcd.GetCert()
	if err != nil {
		return nil, err
	}

	caCertPool.AddCert(serverCert)

	clientCert, err := tls.X509KeyPair(etcd.EtcdAdmin.Crt, etcd.EtcdAdmin.Key)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
		Certificates: []tls.Certificate{
			clientCert,
		},
	}

	c, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{net.JoinHostPort(spec.managementAddress, strconv.Itoa(constants.EtcdClientPort))},
		DialTimeout: 5 * time.Second,
		TLS:         tlsConfig,
		Logger:      zap.NewNop(),
	})
	if err != nil {
		return nil, fmt.Errorf("error building etcd client: %w", err)
	}

	return c, nil
}

func (spec IdentityCollectorTaskSpec) getClient(ctx context.Context) (*client.Client, error) {
	opts := talos.GetSocketOptions(spec.managementAddress)

	if opts == nil {
		opts = append(opts, client.WithEndpoints(spec.managementAddress))
	}

	opts = append(opts, client.WithConfig(spec.config))

	c, err := client.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return c, nil
}
