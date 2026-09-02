// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machine_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/task/machine"
)

func cpuCore(id, socket, vendor, model string, logicalCPUs ...uint32) *hardware.CPUCore {
	core := hardware.NewCPUCore(id)
	core.TypedSpec().Socket = socket
	core.TypedSpec().VendorID = vendor
	core.TypedSpec().ModelName = model
	core.TypedSpec().LogicalCPUs = logicalCPUs

	return core
}

func smbiosProcessor(id, manufacturer, product string, cores, threads, speed uint32) *hardware.Processor {
	processor := hardware.NewProcessorInfo(id)
	processor.TypedSpec().Manufacturer = manufacturer
	processor.TypedSpec().ProductName = product
	processor.TypedSpec().CoreCount = cores
	processor.TypedSpec().ThreadCount = threads
	processor.TypedSpec().MaxSpeed = speed

	return processor
}

func TestPollCPUCores(t *testing.T) {
	for _, test := range []struct {
		name      string
		resources []resource.Resource
		want      []*specs.MachineStatusSpec_HardwareStatus_Processor
	}{
		{
			name: "single socket named by smbios",
			resources: []resource.Resource{
				cpuCore("0-0", "0", "GenuineIntel", "Intel(R) Xeon(R) CPU", 0, 2),
				cpuCore("0-1", "0", "GenuineIntel", "Intel(R) Xeon(R) CPU", 1, 3),
				smbiosProcessor("CPU-0", "Intel", "Xeon", 2, 4, 2400),
			},
			want: []*specs.MachineStatusSpec_HardwareStatus_Processor{
				{CoreCount: 2, ThreadCount: 4, Frequency: 2400, Manufacturer: "Intel", Description: "Intel Xeon"},
			},
		},
		{
			name: "two sockets",
			resources: []resource.Resource{
				cpuCore("0-0", "0", "AuthenticAMD", "AMD EPYC", 0),
				cpuCore("0-1", "0", "AuthenticAMD", "AMD EPYC", 1),
				cpuCore("1-0", "1", "AuthenticAMD", "AMD EPYC", 2),
				smbiosProcessor("CPU-0", "AMD", "EPYC", 2, 2, 3000),
				smbiosProcessor("CPU-1", "AMD", "EPYC", 2, 2, 3000),
			},
			want: []*specs.MachineStatusSpec_HardwareStatus_Processor{
				{CoreCount: 2, ThreadCount: 2, Frequency: 3000, Manufacturer: "AMD", Description: "AMD EPYC"},
				{CoreCount: 1, ThreadCount: 1, Frequency: 3000, Manufacturer: "AMD", Description: "AMD EPYC"},
			},
		},
		{
			name: "no smbios keeps the kernel identity",
			resources: []resource.Resource{
				cpuCore("0-0", "0", "GenuineIntel", "Intel(R) Xeon(R) CPU", 0, 1),
			},
			want: []*specs.MachineStatusSpec_HardwareStatus_Processor{
				{CoreCount: 1, ThreadCount: 2, Manufacturer: "GenuineIntel", Description: "Intel(R) Xeon(R) CPU"},
			},
		},
		{
			name: "arm64 without socket or vendor info",
			resources: []resource.Resource{
				cpuCore("0", "", "", "", 0),
				cpuCore("1", "", "", "", 1),
				cpuCore("2", "", "", "", 2),
				smbiosProcessor("CPU-0", "QEMU", "virt-11.1", 3, 3, 2000),
			},
			want: []*specs.MachineStatusSpec_HardwareStatus_Processor{
				{CoreCount: 3, ThreadCount: 3, Frequency: 2000, Manufacturer: "QEMU", Description: "QEMU virt-11.1"},
			},
		},
		{
			name: "empty smbios socket does not name the populated one",
			resources: []resource.Resource{
				cpuCore("1-0", "1", "GenuineIntel", "Intel(R) Xeon(R) CPU", 0, 1),
				smbiosProcessor("CPU-0", "", "", 0, 0, 4000),
				smbiosProcessor("CPU-1", "Intel", "Xeon", 1, 2, 2400),
			},
			want: []*specs.MachineStatusSpec_HardwareStatus_Processor{
				{CoreCount: 1, ThreadCount: 2, Frequency: 2400, Manufacturer: "Intel", Description: "Intel Xeon"},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			st := state.WrapCore(namespaced.NewState(inmem.Build))

			for _, r := range test.resources {
				require.NoError(t, st.Create(t.Context(), r))
			}

			var info machine.Info

			require.NoError(t, machine.PollCPUCores(t.Context(), &client.Client{COSI: st}, &info))
			require.Equal(t, test.want, info.Processors)
		})
	}
}
