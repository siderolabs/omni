// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package omni provides resources describing the Machines, Clusters, etc.
package omni

const (
	// SystemLabelPrefix is the prefix of all labels which are managed by the COSI controllers.
	// tsgen:SystemLabelPrefix.
	SystemLabelPrefix = "omni.sidero.dev/"
)

const (
	// Global Labels.

	// LabelControlPlaneRole indicates that the machine is a control plane.
	// tsgen:LabelControlPlaneRole
	LabelControlPlaneRole = SystemLabelPrefix + "role-controlplane"

	// LabelWorkerRole indicates that the machine is a worker.
	// tsgen:LabelWorkerRole
	LabelWorkerRole = SystemLabelPrefix + "role-worker"

	// LabelCluster defines the cluster relation label.
	// tsgen:LabelCluster
	LabelCluster = SystemLabelPrefix + LabelSuffixCluster

	// LabelClusterUUID defines the cluster UUID relation label.
	// tsgen:LabelClusterUUID
	LabelClusterUUID = SystemLabelPrefix + "cluster-uuid"

	// LabelHostname defines machine hostname.
	// tsgen:LabelHostname
	LabelHostname = SystemLabelPrefix + LabelSuffixHostname

	// LabelMachineSet defines the machine set relation label.
	// tsgen:LabelMachineSet
	LabelMachineSet = SystemLabelPrefix + "machine-set"

	// LabelClusterMachine defines the cluster machine relation label.
	// tsgen:LabelClusterMachine
	LabelClusterMachine = SystemLabelPrefix + "cluster-machine"

	// LabelMachine defines the machine relation label.
	// tsgen:LabelMachine
	LabelMachine = SystemLabelPrefix + "machine"

	// LabelSystemPatch marks the patch as the system patch, so it shouldn't be editable by the user.
	// tsgen:LabelSystemPatch
	LabelSystemPatch = SystemLabelPrefix + "system-patch"

	// LabelExposedServiceAlias is the alias of the exposed service.
	// tsgen:LabelExposedServiceAlias
	LabelExposedServiceAlias = SystemLabelPrefix + "exposed-service-alias"

	// LabelCloudProviderID is the cloud provider ID for the resources managed by cloud providers, e.g., cloud.MachineRequest, cloud.MachineRequestStatus.
	LabelCloudProviderID = SystemLabelPrefix + "cloud-provider-id"

	// LabelMachineClassName is the name of the machine class.
	LabelMachineClassName = SystemLabelPrefix + "machine-class-name"
)

const (
	// LabelSuffixPlatform is the suffix of the platform label.
	LabelSuffixPlatform = "platform"
	// LabelSuffixArch is the suffix of the arch label.
	LabelSuffixArch = "arch"
	// LabelSuffixHostname is the suffix of the hostname label.
	LabelSuffixHostname = "hostname"
	// LabelSuffixCluster is the suffix of the cluster label.
	LabelSuffixCluster = "cluster"
)

const (
	// MachineStatus labels.

	// MachineStatusLabelConnected is set if the machine is connected.
	// tsgen:MachineStatusLabelConnected
	MachineStatusLabelConnected = SystemLabelPrefix + "connected"

	// MachineStatusLabelDisconnected is set if the machine is disconnected.
	// tsgen:MachineStatusLabelDisconnected
	MachineStatusLabelDisconnected = SystemLabelPrefix + "disconnected"

	// MachineStatusLabelInvalidState is set if Omni can access Talos apid, but has no access.
	// tsgen:MachineStatusLabelInvalidState
	MachineStatusLabelInvalidState = SystemLabelPrefix + "invalid-state"

	// MachineStatusLabelReportingEvents is set if the machine is reporting events.
	// tsgen:MachineStatusLabelReportingEvents
	MachineStatusLabelReportingEvents = SystemLabelPrefix + "reporting-events"

	// MachineStatusLabelAvailable is set if the machine is available to be added to a cluster.
	// tsgen:MachineStatusLabelAvailable
	MachineStatusLabelAvailable = SystemLabelPrefix + "available"

	// MachineStatusLabelArch describes the machine architecture.
	// tsgen:MachineStatusLabelArch
	MachineStatusLabelArch = SystemLabelPrefix + LabelSuffixArch

	// MachineStatusLabelCPU describes the machine CPU.
	// tsgen:MachineStatusLabelCPU
	MachineStatusLabelCPU = SystemLabelPrefix + "cpu"

	// MachineStatusLabelCores describes the number of machine cores.
	// tsgen:MachineStatusLabelCores
	MachineStatusLabelCores = SystemLabelPrefix + "cores"

	// MachineStatusLabelMem describes the total memory available on the machine.
	// tsgen:MachineStatusLabelMem
	MachineStatusLabelMem = SystemLabelPrefix + "mem"

	// MachineStatusLabelStorage describes the total storage capacity of the machine.
	// tsgen:MachineStatusLabelStorage
	MachineStatusLabelStorage = SystemLabelPrefix + "storage"

	// MachineStatusLabelNet describes the machine network adapters speed.
	// tsgen:MachineStatusLabelNet
	MachineStatusLabelNet = SystemLabelPrefix + "net"

	// MachineStatusLabelPlatform describes the machine platform.
	// tsgen:MachineStatusLabelPlatform
	MachineStatusLabelPlatform = SystemLabelPrefix + LabelSuffixPlatform

	// MachineStatusLabelRegion describes the machine region (for machines running in the clouds).
	// tsgen:MachineStatusLabelRegion
	MachineStatusLabelRegion = SystemLabelPrefix + "region"

	// MachineStatusLabelZone describes the machine zone (for machines running in the clouds).
	// tsgen:MachineStatusLabelZone
	MachineStatusLabelZone = SystemLabelPrefix + "zone"

	// MachineStatusLabelInstance describes the machine instance type (for machines running in the clouds).
	// tsgen:MachineStatusLabelInstance
	MachineStatusLabelInstance = SystemLabelPrefix + "instance"

	// MachineStatusLabelTalosVersion describes the machine talos version.
	// tsgen:MachineStatusLabelTalosVersion
	MachineStatusLabelTalosVersion = SystemLabelPrefix + "talos-version"
)

const (
	// ClusterMachineStatus labels.

	// ClusterMachineStatusLabelNodeName is set to the node name.
	// tsgen:ClusterMachineStatusLabelNodeName
	ClusterMachineStatusLabelNodeName = SystemLabelPrefix + "node-name"
)

const (
	// Machine labels.

	// MachineAddressLabel is used for faster lookup of the machine by address.
	MachineAddressLabel = SystemLabelPrefix + "address"
)

const (
	// MachineExtensions labels.

	// ExtensionsConfigurationLabel defines the source ExtensionConfiguration resource
	// from which MachineExtensions resource was generated.
	// tsgen:ExtensionsConfigurationLabel
	ExtensionsConfigurationLabel = SystemLabelPrefix + "root-configuration"
)
