// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package models

import (
	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// KernelArgs is a wrapper type for the kernel args, which can be specified at a Cluster, MachineSet, or Machine level in cluster templates.
//
// This type needs to be included in the models as an inline YAML, e.g., via `yaml:",inline"`.
type KernelArgs struct {
	Value *[]string `yaml:"kernelArgs,omitempty"`
}

// Get returns the kernel args value and whether they are actually defined.
func (ka *KernelArgs) Get() (args []string, defined bool) {
	if ka.Value != nil {
		return *ka.Value, true
	}

	return nil, false
}

// Set sets the kernel args value.
func (ka *KernelArgs) Set(args []string) {
	ka.Value = &args
}

func buildKernelArgsResource(id MachineID, args []string) *omni.KernelArgs {
	kernelArgsRes := omni.NewKernelArgs(resource.ID(id))

	kernelArgsRes.Metadata().Annotations().Set(omni.ResourceManagedByClusterTemplates, "")

	kernelArgsRes.TypedSpec().Value.Args = args

	return kernelArgsRes
}
