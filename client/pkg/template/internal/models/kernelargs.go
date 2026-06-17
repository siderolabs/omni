// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package models

import (
	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func buildKernelArgsResource(id MachineID, args []string) *omni.KernelArgs {
	kernelArgsRes := omni.NewKernelArgs(resource.ID(id))

	kernelArgsRes.Metadata().Annotations().Set(omni.ResourceManagedByClusterTemplates, "")

	kernelArgsRes.TypedSpec().Value.Args = args

	return kernelArgsRes
}
