// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package constants

import (
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/perf"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// Copy Talos constants to generate them for Typescript.
const (
	// Types.
	// tsgen:TalosServiceType
	_ = v1alpha1.ServiceType
	// tsgen:TalosCPUType
	_ = perf.CPUType
	// tsgen:TalosMemoryType
	_ = perf.MemoryType
	// tsgen:TalosNodenameType
	_ = k8s.NodenameType
	// tsgen:TalosMemberType
	_ = cluster.MemberType
	// tsgen:TalosNodeAddressType
	_ = network.NodeAddressType
	// tsgen:TalosMountStatusType
	_ = runtime.MountStatusType

	// Resource ids.
	// tsgen:TalosNodenameID
	_ = k8s.NodenameID
	// tsgen:TalosAddressRoutedNoK8s
	_ = "routed-no-k8s"
	// tsgen:TalosCPUID
	_ = perf.CPUID
	// tsgen:TalosMemoryID
	_ = perf.MemoryID

	// Namespaces.
	// tsgen:TalosPerfNamespace
	_ = perf.NamespaceName
	// tsgen:TalosClusterNamespace
	_ = cluster.NamespaceName
	// tsgen:TalosRuntimeNamespace
	_ = v1alpha1.NamespaceName
	// tsgen:TalosK8sNamespace
	_ = k8s.NamespaceName
	// tsgen:TalosNetworkNamespace
	_ = network.NamespaceName
)
