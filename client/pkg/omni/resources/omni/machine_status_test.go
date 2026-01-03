// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/runtime"
)

func TestMachineStatusReconcileLabels(t *testing.T) {
	t.Parallel()

	for _, test := range []struct { //nolint:govet
		name string
		spec *specs.MachineStatusSpec
		want map[string]string
	}{
		{
			name: "empty",
			spec: &specs.MachineStatusSpec{},
		},
		{
			name: "full",
			spec: &specs.MachineStatusSpec{
				Hardware: &specs.MachineStatusSpec_HardwareStatus{
					Arch: "amd64",
					Processors: []*specs.MachineStatusSpec_HardwareStatus_Processor{
						{
							Manufacturer: "Intel",
							CoreCount:    4,
						},
						{
							CoreCount: 2,
						},
					},
					MemoryModules: []*specs.MachineStatusSpec_HardwareStatus_MemoryModule{
						{
							SizeMb: 8192,
						},
						{
							SizeMb: 8192,
						},
					},
					Blockdevices: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
						{
							Size: 1024 * 1024,
						},
						{
							Size: 200 * 1000 * 1000 * 1000,
						},
						{
							Size: 400 * 1000 * 1000 * 1000,
						},
					},
				},
				Network: &specs.MachineStatusSpec_NetworkStatus{
					NetworkLinks: []*specs.MachineStatusSpec_NetworkStatus_NetworkLinkStatus{
						{
							SpeedMbps: 43333333,
						},
						{
							SpeedMbps: 10 * 1024,
						},
						{
							SpeedMbps: 20 * 1024,
						},
					},
				},
				PlatformMetadata: &specs.MachineStatusSpec_PlatformMetadata{
					Platform:     "aws",
					Region:       "us-east-1",
					Zone:         "us-east-1a",
					InstanceType: "c1.small",
				},
			},
			want: map[string]string{
				omni.MachineStatusLabelArch:     "amd64",
				omni.MachineStatusLabelCPU:      "intel",
				omni.MachineStatusLabelCores:    "6",
				omni.MachineStatusLabelMem:      "16GiB",
				omni.MachineStatusLabelStorage:  "600GB",
				omni.MachineStatusLabelNet:      "30Gbps",
				omni.MachineStatusLabelPlatform: "aws",
				omni.MachineStatusLabelRegion:   "us-east-1",
				omni.MachineStatusLabelZone:     "us-east-1a",
				omni.MachineStatusLabelInstance: "c1.small",
			},
		},
		{
			name: "full",
			spec: &specs.MachineStatusSpec{
				Hardware: &specs.MachineStatusSpec_HardwareStatus{
					Arch: "amd64",
					Processors: []*specs.MachineStatusSpec_HardwareStatus_Processor{
						{
							Manufacturer: "Intel(R) Corporation",
							CoreCount:    4,
						},
						{
							Manufacturer: "Intel(R) Corporation",
							CoreCount:    2,
						},
					},
				},
				Network: &specs.MachineStatusSpec_NetworkStatus{
					NetworkLinks: []*specs.MachineStatusSpec_NetworkStatus_NetworkLinkStatus{
						{
							SpeedMbps: 1000,
						},
					},
				},
			},
			want: map[string]string{
				omni.MachineStatusLabelArch:  "amd64",
				omni.MachineStatusLabelCores: "6",
				omni.MachineStatusLabelCPU:   "intel",
				omni.MachineStatusLabelNet:   "1Gbps",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ms := omni.NewMachineStatus("")

			ms.TypedSpec().Value = test.spec

			omni.MachineStatusReconcileLabels(ms)

			assert.Equal(t, test.want, ms.Metadata().Labels().Raw())
		})
	}
}

func TestLookup(t *testing.T) {
	ms := omni.NewMachineStatus("")
	ms.TypedSpec().Value = &specs.MachineStatusSpec{
		Cluster: "random-cluster",
		PlatformMetadata: &specs.MachineStatusSpec_PlatformMetadata{
			Platform: "aws",
			Hostname: "myhostname",
			Region:   "us-west",
		},
		Network: &specs.MachineStatusSpec_NetworkStatus{
			Hostname: "my-network-hostname",
			NetworkLinks: []*specs.MachineStatusSpec_NetworkStatus_NetworkLinkStatus{
				{
					HardwareAddress: "f2:64:e7:e0:0b:12",
				},
			},
		},
	}

	ext, ok := typed.LookupExtension[runtime.Matcher](ms)
	assert.True(t, ok)
	assert.True(t, ext.Match("random-cluster"))
	assert.True(t, ext.Match("myhostname"))
	assert.True(t, ext.Match("my-network-hostname"))
	assert.True(t, ext.Match("f2:64:e7:e0:0b:12"))
	assert.False(t, ext.Match("random-cluster-2"))
}
