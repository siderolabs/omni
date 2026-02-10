// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machineconfig_test

import (
	_ "embed"
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/machineconfig"
)

// BenchmarkComputeDiff tests various state transitions of the diff logic.
func BenchmarkComputeDiff(b *testing.B) {
	modifiedConfigBytes := modifyConfig(b, baseConfigBytes, func(c *v1alpha1.Config) {
		c.MachineConfig.MachineFiles = append(c.MachineConfig.MachineFiles,
			&v1alpha1.MachineFile{
				FileContent:     "aaa",
				FilePermissions: 0o777,
				FilePath:        "/var/f",
				FileOp:          "create",
			},
		)
	})

	installChangeConfigBytes := modifyConfig(b, baseConfigBytes, func(c *v1alpha1.Config) {
		c.MachineConfig.MachineInstall.InstallDisk = "/dev/sdb"
	})

	b.ResetTimer()

	b.Run("EmptyToEmpty", func(b *testing.B) {
		for range b.N {
			machineconfig.ComputeDiff(nil, nil) //nolint:errcheck
		}
	})

	b.Run("EmptyToPopulated", func(b *testing.B) {
		for range b.N {
			machineconfig.ComputeDiff(nil, baseConfigBytes) //nolint:errcheck
		}
	})

	b.Run("PopulatedToEmpty", func(b *testing.B) {
		for range b.N {
			machineconfig.ComputeDiff(baseConfigBytes, nil) //nolint:errcheck
		}
	})

	b.Run("Identical", func(b *testing.B) {
		for range b.N {
			machineconfig.ComputeDiff(baseConfigBytes, baseConfigBytes) //nolint:errcheck
		}
	})

	b.Run("RealDiff", func(b *testing.B) {
		for range b.N {
			machineconfig.ComputeDiff(baseConfigBytes, modifiedConfigBytes) //nolint:errcheck
		}
	})

	b.Run("InstallSectionIgnored", func(b *testing.B) {
		for range b.N {
			machineconfig.ComputeDiff(baseConfigBytes, installChangeConfigBytes) //nolint:errcheck
		}
	})
}

func modifyConfig(t testing.TB, data []byte, update func(*v1alpha1.Config)) []byte {
	cfg, err := configloader.NewFromBytes(data)
	require.NoError(t, err)

	c, err := container.New(cfg.Documents()...)
	require.NoError(t, err)

	v1Cfg := c.RawV1Alpha1()
	require.NotNil(t, v1Cfg)

	update(v1Cfg)

	newData, err := c.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	require.NoError(t, err)

	return newData
}

//go:embed testdata/base-config.yaml
var baseConfigBytes []byte

//go:embed testdata/empty-to-populated.diff
var emptyToPopulatedDiff string

//go:embed testdata/populated-to-empty.diff
var populatedToEmptyDiff string

// TestComputeDiff tests the ComputeDiff function with various state transitions.
func TestComputeDiff(t *testing.T) {
	modifiedConfigBytes := modifyConfig(t, baseConfigBytes, func(c *v1alpha1.Config) {
		c.MachineConfig.MachineFiles = append(c.MachineConfig.MachineFiles,
			&v1alpha1.MachineFile{
				FileContent:     "aaa",
				FilePermissions: 0o777,
				FilePath:        "/var/f",
				FileOp:          "create",
			},
		)
	})

	tests := []struct {
		name         string
		wantDiff     string
		previousData []byte
		newData      []byte
		wantErr      bool
	}{
		{
			name:         "empty to empty",
			previousData: nil,
			newData:      nil,
		},
		{
			name:         "empty to populated",
			previousData: nil,
			newData:      baseConfigBytes,
			wantDiff:     emptyToPopulatedDiff,
		},
		{
			name:         "populated to empty",
			previousData: baseConfigBytes,
			newData:      nil,
			wantDiff:     populatedToEmptyDiff,
		},
		{
			name:         "identical",
			previousData: baseConfigBytes,
			newData:      baseConfigBytes,
			wantDiff:     "",
		},
		{
			name:         "real diff",
			previousData: baseConfigBytes,
			newData:      modifiedConfigBytes,
			wantDiff: `@@ -15,6 +15,11 @@
     install:
         wipe: false
         grubUseUKICmdline: true
+    files:
+        - content: aaa
+          permissions: 0o777
+          path: /var/f
+          op: create
     features:
         diskQuotaSupport: true
         kubePrism:
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff, err := machineconfig.ComputeDiff(tt.previousData, tt.newData)

			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantDiff, diff)
		})
	}
}
