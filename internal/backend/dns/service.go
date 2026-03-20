// Copyright (c) 2026 Sidero Labs, Inc.
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

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

type record struct {
	cluster string
	name    string
}

// ErrNotFound is returned when a node cannot be resolved.
var ErrNotFound = errors.New("node not found, cannot resolve its management address")

// Info contains information about a node.
type Info struct {
	Cluster      string
	ID           string
	Name         string
	TalosVersion string

	// address is the node's cluster-internal IP (from ClusterMachineIdentity.NodeIPs).
	address string

	// ManagementEndpoint is the node's SideroLink address (from MachineStatus.ManagementAddress).
	// Only routable from Omni, not between nodes. Used as a fallback by GetAddress()
	// when the cluster-internal IP is not yet known.
	ManagementEndpoint string
}

// GetAddress returns the node's cluster-internal IP if known,
// falling back to the SideroLink management address.
// The fallback covers the race window during cluster bootstrap
// when ClusterMachineIdentity.NodeIPs is not yet populated.
func (i Info) GetAddress() string {
	if i.address != "" {
		return i.address
	}

	return i.ManagementEndpoint
}

type resolverMap map[string][]resource.ID

func (m resolverMap) get(key string) []resource.ID {
	return m[key]
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
	} else {
		m[key] = ids
	}
}

// Service is the DNS service.
type Service struct {
	omniState state.State
	logger    *zap.Logger

	recordToMachineID  map[record]resource.ID
	machineIDToInfo    map[resource.ID]Info
	machineIDToAddress map[resource.ID]string
	addressToID        resolverMap
	nodenameToID       resolverMap

	lock sync.Mutex
}

// NewService creates a new DNS service. It needs to be started before use.
func NewService(omniState state.State, logger *zap.Logger) *Service {
	return &Service{
		omniState:          omniState,
		logger:             logger,
		recordToMachineID:  make(map[record]string),
		machineIDToInfo:    make(map[string]Info),
		machineIDToAddress: make(map[string]string),
		addressToID:        make(resolverMap),
		nodenameToID:       make(resolverMap),
	}
}

// Start starts the DNS service.
func (d *Service) Start(ctx context.Context) error {
	ch := make(chan state.Event)

	if err := d.omniState.WatchKind(ctx, omni.NewClusterMachineIdentity("").Metadata(), ch, state.WithBootstrapContents(true)); err != nil {
		return err
	}

	if err := d.omniState.WatchKind(ctx, omni.NewMachineStatus("").Metadata(), ch, state.WithBootstrapContents(true)); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			d.logger.Debug("stopping DNS service")

			return nil
		case ev := <-ch:
			if err := d.handleEvent(ev); err != nil {
				return err
			}
		}
	}
}

func (d *Service) handleEvent(ev state.Event) error {
	switch ev.Type {
	case state.Errored:
		return fmt.Errorf("dns service received an error event: %w", ev.Error)
	case state.Bootstrapped, state.Noop:
		// ignore
	case state.Destroyed:
		switch r := ev.Resource.(type) {
		case *omni.ClusterMachineIdentity:
			d.deleteIdentityMappings(r.Metadata().ID())
		case *omni.MachineStatus:
			d.deleteMachineMappings(r.Metadata().ID())
		default:
			d.logger.Warn("dns service received a destroyed event with an unexpected resource type", zap.String("type", fmt.Sprintf("%T", r)))
		}
	case state.Created, state.Updated:
		switch r := ev.Resource.(type) {
		case *omni.ClusterMachineIdentity:
			d.updateEntryByIdentity(r)
		case *omni.MachineStatus:
			d.updateEntryByMachineStatus(r)
		default:
			d.logger.Warn("dns service received an event with an unexpected resource type", zap.String("type", fmt.Sprintf("%T", r)))
		}
	}

	return nil
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

	id := res.Metadata().ID()

	// create or update info
	info := d.machineIDToInfo[id]

	info.Cluster = clusterName
	info.ID = id

	previousAddress := d.machineIDToAddress[id]
	previousNodename := info.Name

	info.Name = nodeName

	var address string

	if nodeIPs := res.TypedSpec().Value.NodeIps; len(nodeIPs) > 0 {
		address = nodeIPs[0]
	}

	info.address = address

	d.machineIDToInfo[id] = info
	d.machineIDToAddress[id] = address

	// create entry by node name
	d.recordToMachineID[record{
		cluster: clusterName,
		name:    nodeName,
	}] = id

	// create entry by machine ID
	d.recordToMachineID[record{
		cluster: clusterName,
		name:    id,
	}] = id

	d.nodenameToID.add(nodeName, id)

	// create entry by address
	if address != "" {
		d.recordToMachineID[record{
			cluster: clusterName,
			name:    address,
		}] = id

		d.addressToID.add(address, id)
	}

	// cleanup old entry by address
	if previousAddress != "" && previousAddress != address {
		delete(d.recordToMachineID, record{
			cluster: clusterName,
			name:    previousAddress,
		})

		d.addressToID.remove(previousAddress, id)
	}

	// cleanup old entry by nodename
	if previousNodename != "" && previousNodename != info.Name {
		delete(d.recordToMachineID, record{
			cluster: clusterName,
			name:    previousNodename,
		})

		d.nodenameToID.remove(previousNodename, id)
	}

	d.logger.Debug(
		"set node DNS entry",
		zap.String("id", id),
		zap.String("cluster", clusterName),
		zap.String("node_name", nodeName),
		zap.String("address", address),
	)
}

func (d *Service) updateEntryByMachineStatus(res *omni.MachineStatus) {
	version := res.TypedSpec().Value.TalosVersion
	if version == "" {
		d.logger.Warn("no Talos version in the machine status", zap.String("id", res.Metadata().ID()))
	}

	d.lock.Lock()
	defer d.lock.Unlock()

	info := d.machineIDToInfo[res.Metadata().ID()]

	info.ID = res.Metadata().ID()
	info.TalosVersion = version
	info.ManagementEndpoint = res.TypedSpec().Value.ManagementAddress

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
	address := d.machineIDToAddress[id]

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
			name:    address,
		})

		d.addressToID.remove(address, id)
		d.nodenameToID.remove(info.Name, id)
	}

	info.Cluster = ""
	info.Name = ""
	info.address = ""

	d.machineIDToInfo[id] = info
	delete(d.machineIDToAddress, id)

	d.logger.Debug(
		"deleted node identity DNS entry",
		zap.String("id", id),
		zap.String("cluster", info.Cluster),
		zap.String("node_name", info.Name),
		zap.String("address", address),
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
	delete(d.machineIDToAddress, id)

	d.logger.Debug(
		"deleted node machine status DNS entry",
		zap.String("id", id),
		zap.String("cluster", info.Cluster),
		zap.String("node_name", info.Name),
	)
}

func (d *Service) resolveByAddressOrNodename(name string) (Info, error) {
	for _, resolver := range []resolverMap{
		d.addressToID,
		d.nodenameToID,
	} {
		ids := resolver.get(name)
		if len(ids) > 1 {
			return Info{}, fmt.Errorf("name or address %q is ambiguous, please specify the cluster name explicitly", name)
		}

		if len(ids) > 0 {
			return d.machineIDToInfo[ids[0]], nil
		}
	}

	return Info{}, ErrNotFound
}

// Resolve resolves a node by name, address or machine UUID.
// When clusterName is provided, it scopes the lookup, prevents cross-cluster results and removes ambiguity.
// When clusterName is empty, the lookup falls back to global resolution by address, nodename or machine ID.
// For maintenance-mode machines that are not yet part of a cluster, clusterName must be empty.
func (d *Service) Resolve(clusterName, name string) (Info, error) {
	if name == "" {
		return Info{}, errors.New("node name must not be empty")
	}

	d.lock.Lock()
	defer d.lock.Unlock()

	nodeID, ok := d.recordToMachineID[record{
		cluster: clusterName,
		name:    name,
	}]

	if !ok {
		info, err := d.resolveByAddressOrNodename(name)
		if !errors.Is(err, ErrNotFound) {
			// Global fallback matched — guard against returning a node from a different cluster.
			if clusterName != "" && info.Cluster != "" && info.Cluster != clusterName {
				return Info{}, ErrNotFound
			}

			return info, err
		}
	}

	if !ok {
		info, found := d.machineIDToInfo[name]
		if !found {
			return Info{}, ErrNotFound
		}

		// Direct machine ID lookup is global — guard against returning a node from a different cluster.
		if clusterName != "" && info.Cluster != "" && info.Cluster != clusterName {
			return Info{}, ErrNotFound
		}

		return info, nil
	}

	info, found := d.machineIDToInfo[nodeID]
	if !found {
		return Info{}, ErrNotFound
	}

	return info, nil
}
