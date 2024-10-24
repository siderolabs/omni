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
	"slices"
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
	TalosVersion string

	address            string
	managementEndpoint string

	Ambiguous bool
}

// NewInfo exports unexported.
func NewInfo(cluster, id, name, address string) Info {
	return Info{
		Cluster: cluster,
		ID:      id,
		Name:    name,
		address: address,
	}
}

// GetAddress reads node address from the DNS info.
func (i Info) GetAddress() string {
	if i.address != "" {
		return i.address
	}

	return i.managementEndpoint
}

type resolutionResult int

const (
	found = iota
	notFound
	ambiguous
)

type resolverMap map[string][]resource.ID

func (m resolverMap) get(key string) (resource.ID, resolutionResult) {
	ids, ok := m[key]
	if !ok {
		return "", notFound
	}

	if len(ids) > 1 {
		return "", ambiguous
	}

	return ids[0], found
}

func (m resolverMap) add(key string, id resource.ID) {
	if slices.Index(m[key], id) != -1 {
		return
	}

	m[key] = append(m[key], id)
}

func (m resolverMap) remove(key string, id resource.ID) {
	ids, ok := m[key]
	if !ok {
		return
	}

	ids = slices.DeleteFunc(ids, func(item string) bool { return item == id })
	if len(ids) == 0 {
		delete(m, key)
	}
}

// Service is the DNS service.
type Service struct {
	omniState state.State
	logger    *zap.Logger

	recordToMachineID map[record]resource.ID
	machineIDToInfo   map[resource.ID]Info
	addressToID       resolverMap
	nodenameToID      resolverMap

	lock sync.Mutex
}

// NewService creates a new DNS service. It needs to be started before use.
func NewService(omniState state.State, logger *zap.Logger) *Service {
	return &Service{
		omniState:         omniState,
		logger:            logger,
		recordToMachineID: make(map[record]string),
		machineIDToInfo:   make(map[string]Info),
		addressToID:       make(resolverMap),
		nodenameToID:      make(resolverMap),
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
		omni.NewMachineStatus(resources.DefaultNamespace, "").Metadata(), ch,
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

				switch r := ev.Resource.(type) {
				case *omni.ClusterMachineIdentity:
					d.deleteIdentityMappings(r.Metadata().ID())
				case *omni.MachineStatus:
					d.deleteMachineMappings(r.Metadata().ID())
				}
			case state.Created, state.Updated:
				switch r := ev.Resource.(type) {
				case *omni.ClusterMachineIdentity:
					d.updateEntryByIdentity(r)
				case *omni.MachineStatus:
					d.updateEntryByMachineStatus(r)
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
	info := d.machineIDToInfo[res.Metadata().ID()]

	info.Cluster = clusterName
	info.ID = res.Metadata().ID()

	previousAddress := info.address
	previousNodename := info.Name

	info.Name = nodeName

	nodeIPs := res.TypedSpec().Value.NodeIps
	if len(nodeIPs) == 0 {
		info.address = ""
	} else {
		info.address = nodeIPs[0]
	}

	d.machineIDToInfo[res.Metadata().ID()] = info

	// create entry by node name
	d.recordToMachineID[record{
		cluster: clusterName,
		name:    nodeName,
	}] = res.Metadata().ID()

	// create entry by machine ID
	d.recordToMachineID[record{
		cluster: clusterName,
		name:    res.Metadata().ID(),
	}] = res.Metadata().ID()

	d.nodenameToID.add(nodeName, res.Metadata().ID())

	// create entry by address
	if info.address != "" {
		d.recordToMachineID[record{
			cluster: clusterName,
			name:    info.address,
		}] = res.Metadata().ID()

		d.addressToID.add(info.address, res.Metadata().ID())
	}

	// cleanup old entry by address
	if previousAddress != "" && previousAddress != info.address {
		delete(d.recordToMachineID, record{
			cluster: clusterName,
			name:    previousAddress,
		})

		d.addressToID.remove(previousAddress, res.Metadata().ID())
	}

	// cleanup old entry by nodename
	if previousNodename != "" && previousNodename != info.Name {
		delete(d.recordToMachineID, record{
			cluster: clusterName,
			name:    previousNodename,
		})

		d.nodenameToID.remove(previousNodename, res.Metadata().ID())
	}

	d.logger.Debug(
		"set node DNS entry",
		zap.String("id", res.Metadata().ID()),
		zap.String("cluster", clusterName),
		zap.String("node_name", nodeName),
		zap.String("address", info.address),
	)
}

func (d *Service) updateEntryByMachineStatus(res *omni.MachineStatus) {
	version := res.TypedSpec().Value.TalosVersion
	if version == "" {
		d.logger.Warn("no Talos version in the machine status", zap.String("id", res.Metadata().ID()))

		return
	}

	d.lock.Lock()
	defer d.lock.Unlock()

	info := d.machineIDToInfo[res.Metadata().ID()]

	info.TalosVersion = version
	info.managementEndpoint = res.TypedSpec().Value.ManagementAddress

	d.machineIDToInfo[res.Metadata().ID()] = info

	d.logger.Debug(
		"update machine id -> address mapping",
		zap.String("id", res.Metadata().ID()),
		zap.String("talos_version", version),
	)
}

func (d *Service) deleteIdentityMappings(id resource.ID) {
	d.lock.Lock()
	defer d.lock.Unlock()

	info, infoOk := d.machineIDToInfo[id]
	if infoOk {
		delete(d.recordToMachineID, record{
			cluster: info.Cluster,
			name:    info.ID,
		})
		delete(d.recordToMachineID, record{
			cluster: info.Cluster,
			name:    info.Name,
		})
		delete(d.recordToMachineID, record{
			cluster: info.Cluster,
			name:    info.address,
		})

		d.addressToID.remove(info.address, id)
		d.nodenameToID.remove(info.Name, id)
	}

	info.address = ""

	d.machineIDToInfo[id] = info

	d.logger.Debug(
		"deleted node identity DNS entry",
		zap.String("id", id),
		zap.String("cluster", info.Cluster),
		zap.String("node_name", info.Name),
		zap.String("address", info.address),
	)
}

func (d *Service) deleteMachineMappings(id resource.ID) {
	d.lock.Lock()
	defer d.lock.Unlock()

	info, infoOk := d.machineIDToInfo[id]
	if !infoOk {
		return
	}

	delete(d.machineIDToInfo, id)

	d.logger.Debug(
		"deleted node machine status DNS entry",
		zap.String("id", id),
		zap.String("cluster", info.Cluster),
		zap.String("node_name", info.Name),
		zap.String("address", info.address),
	)
}

func (d *Service) resolveByAddressOrNodename(name string) (Info, bool) {
	for _, resolver := range []resolverMap{
		d.addressToID,
		d.nodenameToID,
	} {
		nodeID, result := resolver.get(name)
		if result == ambiguous {
			return Info{
				Name:      name,
				Ambiguous: true,
			}, true
		}

		if result == found {
			return d.machineIDToInfo[nodeID], true
		}
	}

	return Info{}, false
}

// Resolve returns the dns.Info for the given node name, address or machine UUID.
func (d *Service) Resolve(clusterName, name string) Info {
	d.lock.Lock()
	defer d.lock.Unlock()

	nodeID, ok := d.recordToMachineID[record{
		cluster: clusterName,
		name:    name,
	}]

	if !ok {
		var info Info

		info, ok = d.resolveByAddressOrNodename(name)

		if ok {
			return info
		}
	}

	if !ok {
		return d.machineIDToInfo[name]
	}

	return d.machineIDToInfo[nodeID]
}
