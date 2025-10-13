// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package template

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// Canonical order of resources in the generated list.
var canonicalResourceOrder = map[resource.Type]int{
	omni.ClusterType:                      1,
	omni.ExtensionsConfigurationType:      2,
	omni.ExtraKernelArgsConfigurationType: 3,
	omni.ConfigPatchType:                  4,
	omni.MachineSetType:                   5,
	omni.MachineSetNodeType:               6,
}

func sortResources[T any](s []T, mapper func(T) resource.Metadata) {
	slices.SortStableFunc(s, func(a, b T) int {
		orderI := canonicalResourceOrder[mapper(a).Type()]
		orderJ := canonicalResourceOrder[mapper(b).Type()]

		if orderI == 0 {
			panic(fmt.Sprintf("unknown resource type %q", mapper(a).Type()))
		}

		if orderJ == 0 {
			panic(fmt.Sprintf("unknown resource type %q", mapper(b).Type()))
		}

		return cmp.Compare(orderI, orderJ)
	})
}
