// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package dns provides node name to node IP lookups, similar to a Service service.
package dns

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

type record struct {
	cluster string
	name    string
}

// Info contains information about a node.
type Info struct {
	Cluster      string
	ID           string
	Name         string
	Address      string
	TalosVersion string
}

// Service is the DNS service.
type Service struct {
	omniState state.State
	logger    *zap.Logger

	recordToNodeID map[record]resource.ID
	nodeIDToInfo   map[resource.ID]Info

	lock sync.Mutex
}

// NewService creates a new DNS service. It needs to be started before use.
func NewService(omniState state.State, logger *zap.Logger) *Service {
	return &Service{
		omniState:      omniState,
		logger:         logger,
		recordToNodeID: make(map[record]string),
		nodeIDToInfo:   make(map[string]Info),
	}
}

// Start starts the DNS service.
func (d *Service) Start(ctx context.Context) error {
	ch := make(chan state.Event)

	if err := d.omniState.WatchKind(ctx,
		omni.NewClusterMachineIdentity(resources.DefaultNamespace, "").Metadata(), ch,
		state.WithBootstrapContents(true),
	); err != nil {
		return err
	}

	if err := d.omniState.WatchKind(ctx,
		omni.NewClusterMachineConfigStatus(resources.DefaultNamespace, "").Metadata(), ch,
		state.WithBootstrapContents(true),
	); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				d.logger.Debug("stopping DNS service")

				return nil
			}

			return fmt.Errorf("dns service context error: %w", ctx.Err())
		case ev := <-ch:
			switch ev.Type {
			case state.Errored:
				return fmt.Errorf("dns service received an error event: %w", ev.Error)
			case state.Bootstrapped:
				// ignore
			case state.Destroyed:
				if ev.Resource == nil {
					d.logger.Warn("dns service received a destroyed event without a resource")

					continue
				}

				if _, ok := ev.Resource.(*omni.ClusterMachineIdentity); ok {
					d.deleteByID(ev.Resource.Metadata().ID())
				}
			case state.Created, state.Updated:
				switch r := ev.Resource.(type) {
				case *omni.ClusterMachineIdentity:
					d.updateEntryByIdentity(r)
				case *omni.ClusterMachineConfigStatus:
					d.updateEntryByConfigStatus(r)
				default:
					d.logger.Warn(
						"dns service received an event with an unexpected resource type",
						zap.String("type", fmt.Sprintf("%T", r)),
					)

					continue
				}
			}
		}
	}
}

func (d *Service) updateEntryByIdentity(res *omni.ClusterMachineIdentity) {
	nodeName := res.TypedSpec().Value.Nodename
	if nodeName == "" {
		d.logger.Warn("received cluster machine identity without a node name", zap.String("id", res.Metadata().ID()))

		return
	}

	clusterName, ok := res.Metadata().Labels().Get(omni.LabelCluster)
	if !ok {
		d.logger.Warn("received cluster machine identity without cluster label", zap.String("id", res.Metadata().ID()))

		return
	}

	d.lock.Lock()
	defer d.lock.Unlock()

	// create or update info
	info := d.nodeIDToInfo[res.Metadata().ID()]

	info.Cluster = clusterName
	info.ID = res.Metadata().ID()
	info.Name = nodeName

	previousAddress := info.Address

	nodeIPs := res.TypedSpec().Value.NodeIps
	if len(nodeIPs) == 0 {
		info.Address = ""
	} else {
		info.Address = nodeIPs[0]
	}

	d.nodeIDToInfo[res.Metadata().ID()] = info

	// create entry by node name
	d.recordToNodeID[record{
		cluster: clusterName,
		name:    nodeName,
	}] = res.Metadata().ID()

	// create entry by machine ID
	d.recordToNodeID[record{
		cluster: clusterName,
		name:    res.Metadata().ID(),
	}] = res.Metadata().ID()

	// create entry by address
	if info.Address != "" {
		d.recordToNodeID[record{
			cluster: clusterName,
			name:    info.Address,
		}] = res.Metadata().ID()
	}

	// cleanup old entry by address
	if previousAddress != "" && previousAddress != info.Address {
		delete(d.recordToNodeID, record{
			cluster: clusterName,
			name:    previousAddress,
		})
	}

	d.logger.Debug(
		"set node DNS entry",
		zap.String("id", res.Metadata().ID()),
		zap.String("cluster", clusterName),
		zap.String("node_name", nodeName),
		zap.String("address", info.Address),
	)
}

func (d *Service) updateEntryByConfigStatus(res *omni.ClusterMachineConfigStatus) {
	version := res.TypedSpec().Value.GetTalosVersion()
	if version == "" {
		d.logger.Warn("received config status without a Talos version", zap.String("id", res.Metadata().ID()))

		return
	}

	d.lock.Lock()
	defer d.lock.Unlock()

	info := d.nodeIDToInfo[res.Metadata().ID()]

	info.TalosVersion = version

	d.nodeIDToInfo[res.Metadata().ID()] = info

	d.logger.Debug(
		"set node talos version in DNS entry",
		zap.String("id", res.Metadata().ID()),
		zap.String("talos_version", version),
	)
}

func (d *Service) deleteByID(id resource.ID) {
	d.lock.Lock()
	defer d.lock.Unlock()

	info, infoOk := d.nodeIDToInfo[id]
	if infoOk {
		delete(d.recordToNodeID, record{
			cluster: info.Cluster,
			name:    info.ID,
		})
		delete(d.recordToNodeID, record{
			cluster: info.Cluster,
			name:    info.Name,
		})
		delete(d.recordToNodeID, record{
			cluster: info.Cluster,
			name:    info.Address,
		})
	}

	delete(d.nodeIDToInfo, id)

	d.logger.Debug(
		"deleted node DNS entry",
		zap.String("id", id),
		zap.String("cluster", info.Cluster),
		zap.String("node_name", info.Name),
		zap.String("address", info.Address),
	)
}

// Resolve returns the dns.Info for the given node name, address or machine UUID.
func (d *Service) Resolve(clusterName, name string) Info {
	d.lock.Lock()
	defer d.lock.Unlock()

	nodeID := d.recordToNodeID[record{
		cluster: clusterName,
		name:    name,
	}]

	return d.nodeIDToInfo[nodeID]
}
