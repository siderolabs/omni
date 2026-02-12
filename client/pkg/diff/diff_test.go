// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package diff_test

import (
	"bytes"
	_ "embed"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/diff"
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
			diff.Compute(nil, nil) //nolint:errcheck
		}
	})

	b.Run("EmptyToPopulated", func(b *testing.B) {
		for range b.N {
			diff.Compute(nil, baseConfigBytes) //nolint:errcheck
		}
	})

	b.Run("PopulatedToEmpty", func(b *testing.B) {
		for range b.N {
			diff.Compute(baseConfigBytes, nil) //nolint:errcheck
		}
	})

	b.Run("Identical", func(b *testing.B) {
		for range b.N {
			diff.Compute(baseConfigBytes, baseConfigBytes) //nolint:errcheck
		}
	})

	b.Run("RealDiff", func(b *testing.B) {
		for range b.N {
			diff.Compute(baseConfigBytes, modifiedConfigBytes) //nolint:errcheck
		}
	})

	b.Run("InstallSectionIgnored", func(b *testing.B) {
		for range b.N {
			diff.Compute(baseConfigBytes, installChangeConfigBytes) //nolint:errcheck
		}
	})

	largeYAML := generateLargeYAML(10000)

	b.Run("EmptyToLargeYAML", func(b *testing.B) {
		b.ReportAllocs()

		for range b.N {
			diff.Compute(nil, largeYAML) //nolint:errcheck
		}
	})

	b.Run("LargeYAMLToEmpty", func(b *testing.B) {
		b.ReportAllocs()

		for range b.N {
			diff.Compute(largeYAML, nil) //nolint:errcheck
		}
	})

	largeYAMLModified := generateLargeYAML(10001)

	b.Run("LargeYAMLToDifferentLargeYAML", func(b *testing.B) {
		b.ReportAllocs()

		for range b.N {
			diff.Compute(largeYAML, largeYAMLModified) //nolint:errcheck
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
			diff, err := diff.Compute(tt.previousData, tt.newData)

			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantDiff, diff)
		})
	}
}

func TestComputeDiff_LargeInput(t *testing.T) {
	t.Run("structural diff for inputs under threshold", func(t *testing.T) {
		small := generateLargeYAML(1000) // ~8000 lines, well under 50k

		diff, err := diff.Compute(nil, small)
		require.NoError(t, err)
		assert.NotEmpty(t, diff)
		assert.True(t, strings.HasPrefix(diff, "@@ -0,0 +1,"))
	})

	t.Run("bulk diff for inputs over threshold", func(t *testing.T) {
		largeYAML := generateLargeYAML(10000) // ~80k lines, over 50k threshold

		diff, err := diff.Compute(nil, largeYAML)
		require.NoError(t, err)
		assert.Contains(t, diff, "diff too large to display")
	})

	t.Run("bulk diff large to empty", func(t *testing.T) {
		largeYAML := generateLargeYAML(10000)

		diff, err := diff.Compute(largeYAML, nil)
		require.NoError(t, err)
		assert.Contains(t, diff, "diff too large to display")
	})

	t.Run("bulk diff large to different large", func(t *testing.T) {
		largeYAML := generateLargeYAML(10000)
		largeYAMLModified := generateLargeYAML(10001)

		diff, err := diff.Compute(largeYAML, largeYAMLModified)
		require.NoError(t, err)
		assert.Contains(t, diff, "diff too large to display")
	})
}

// TestComputeDiff_MemoryBudget asserts that diff.Compute stays within a memory
// budget across a range of input sizes and patterns.
func TestComputeDiff_MemoryBudget(t *testing.T) {
	const memoryBudget = 50 * 1024 * 1024 // 50 MB hard ceiling

	// generateLines builds a YAML-like blob with exactly n newline-terminated lines.
	// When variant > 0 every variant-th line is changed so the diff is non-trivial.
	generateLines := func(n, variant int) []byte {
		var sb strings.Builder

		for i := range n {
			if variant > 0 && i%variant == 0 {
				fmt.Fprintf(&sb, "line-%d-variant-%d\n", i, variant)
			} else {
				fmt.Fprintf(&sb, "line-%d\n", i)
			}
		}

		return []byte(sb.String())
	}

	tests := []struct {
		old         func() []byte
		new         func() []byte
		name        string
		maxMemBytes uint64
	}{
		{
			name:        "small similar configs (100 lines, few changes)",
			old:         func() []byte { return generateLines(100, 0) },
			new:         func() []byte { return generateLines(100, 10) },
			maxMemBytes: 1 * 1024 * 1024, // 1 MB
		},
		{
			name:        "small completely different (100 lines)",
			old:         func() []byte { return generateLines(100, 0) },
			new:         func() []byte { return generateLines(100, 1) },
			maxMemBytes: 1 * 1024 * 1024,
		},
		{
			name:        "medium similar (5K lines, few changes)",
			old:         func() []byte { return generateLines(5000, 0) },
			new:         func() []byte { return generateLines(5000, 50) },
			maxMemBytes: 10 * 1024 * 1024, // 10 MB
		},
		{
			name:        "medium completely different (5K lines)",
			old:         func() []byte { return generateLines(5000, 0) },
			new:         func() []byte { return generateLines(5000, 1) },
			maxMemBytes: 10 * 1024 * 1024,
		},
		{
			name:        "large similar (30K lines, few changes)",
			old:         func() []byte { return generateLines(30000, 0) },
			new:         func() []byte { return generateLines(30000, 100) },
			maxMemBytes: 50 * 1024 * 1024, // 50 MB
		},
		{
			name:        "large completely different (30K lines)",
			old:         func() []byte { return generateLines(30000, 0) },
			new:         func() []byte { return generateLines(30000, 1) },
			maxMemBytes: 100 * 1024 * 1024, // 100 MB
		},
		{
			name:        "asymmetric: empty to 30K lines",
			old:         func() []byte { return nil },
			new:         func() []byte { return generateLines(30000, 0) },
			maxMemBytes: 50 * 1024 * 1024,
		},
		{
			name:        "asymmetric: 50 lines to 30K lines",
			old:         func() []byte { return generateLines(50, 0) },
			new:         func() []byte { return generateLines(30000, 0) },
			maxMemBytes: 50 * 1024 * 1024,
		},
		{
			name:        "near threshold (total ~74K lines)",
			old:         func() []byte { return generateLines(37000, 0) },
			new:         func() []byte { return generateLines(37000, 1) },
			maxMemBytes: memoryBudget,
		},
		{
			name:        "over threshold (total ~76K lines, returns summary)",
			old:         func() []byte { return generateLines(38000, 0) },
			new:         func() []byte { return generateLines(38000, 1) },
			maxMemBytes: 1 * 1024 * 1024, // summary path allocates almost nothing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldData := tt.old()
			newData := tt.new()

			oldLines := bytes.Count(oldData, []byte("\n"))
			newLines := bytes.Count(newData, []byte("\n"))

			// Measure allocations.
			runtime.GC()

			var before, after runtime.MemStats

			runtime.ReadMemStats(&before)

			result, err := diff.Compute(oldData, newData)
			require.NoError(t, err)

			runtime.ReadMemStats(&after)

			allocated := after.TotalAlloc - before.TotalAlloc

			t.Logf("oldData=%d lines, newData=%d lines, allocated=%s, result=%d bytes",
				oldLines, newLines, formatBytes(allocated), len(result))

			assert.Less(t, allocated, tt.maxMemBytes,
				"memory budget exceeded: allocated %s, limit %s",
				formatBytes(allocated), formatBytes(tt.maxMemBytes))
		})
	}
}

func formatBytes(b uint64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(b)/(1024*1024*1024))
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// generateLargeYAML generates a YAML document with the given number of
// K8s-manifest-like entries to simulate large inline manifests configs.
func generateLargeYAML(n int) []byte {
	var sb strings.Builder

	for i := range n {
		if i > 0 {
			sb.WriteString("---\n")
		}

		fmt.Fprintf(&sb, "apiVersion: v1\n")
		fmt.Fprintf(&sb, "kind: ConfigMap\n")
		fmt.Fprintf(&sb, "metadata:\n")
		fmt.Fprintf(&sb, "  name: manifest-%d\n", i)
		fmt.Fprintf(&sb, "  namespace: default\n")
		fmt.Fprintf(&sb, "data:\n")
		fmt.Fprintf(&sb, "  key: value-%d\n", i)
	}

	return []byte(sb.String())
}
