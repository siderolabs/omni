// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kernelargs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/kernelargs"
)

// Protected args as seen on a real machine.
const (
	siderolink = "siderolink.api=https://omni.example.com?jointoken=example"
	eventsSink = "talos.events.sink=[fdae:41e4:649b:9303::1]:8091"
	logging    = "talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092"
)

func machineStatusWithArgs(currentArgs []string) *omni.MachineStatus {
	ms := omni.NewMachineStatus("test-machine")
	ms.Metadata().Annotations().Set(omni.KernelArgsInitialized, "")
	ms.TypedSpec().Value.Schematic = &specs.MachineStatusSpec_Schematic{
		FullId:     "some-full-id",
		KernelArgs: currentArgs,
	}

	return ms
}

func kernelArgsResource(userArgs []string) *omni.KernelArgs {
	if userArgs == nil {
		return nil
	}

	ka := omni.NewKernelArgs("test-machine")
	ka.TypedSpec().Value.Args = userArgs

	return ka
}

// TestCalculateOrderStability verifies that Calculate preserves the current kernel args
// when they are logically equal to the calculated ones, preventing spurious schematic ID
// changes (and therefore unintended machine upgrades/reboots) caused by arg reordering.
func TestCalculateOrderStability(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		currentArgs []string
		userArgs    []string
		expected    []string
	}{
		{
			// Real-world interleaving: user and protected args fully mixed.
			// Logically equal → preserve the machine's current order.
			name:        "equal: fully interleaved",
			currentArgs: []string{"nomodeset", siderolink, "net.ifnames=0", eventsSink, logging},
			userArgs:    []string{"nomodeset", "net.ifnames=0"},
			expected:    []string{"nomodeset", siderolink, "net.ifnames=0", eventsSink, logging},
		},
		{
			// Another interleaving: protected args scattered around user args.
			name:        "equal: protected args scattered",
			currentArgs: []string{siderolink, "nomodeset", logging, "net.ifnames=0", eventsSink},
			userArgs:    []string{"nomodeset", "net.ifnames=0"},
			expected:    []string{siderolink, "nomodeset", logging, "net.ifnames=0", eventsSink},
		},
		{
			// Current is already in canonical order (protected first) → preserve current.
			name:        "equal: already in canonical order",
			currentArgs: []string{siderolink, eventsSink, logging, "nomodeset", "net.ifnames=0"},
			userArgs:    []string{"nomodeset", "net.ifnames=0"},
			expected:    []string{siderolink, eventsSink, logging, "nomodeset", "net.ifnames=0"},
		},
		{
			// No user args on the machine and no user args resource → preserve current.
			name:        "equal: only protected args, no user args",
			currentArgs: []string{siderolink, eventsSink, logging},
			userArgs:    nil,
			expected:    []string{siderolink, eventsSink, logging},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, initialized, err := kernelargs.Calculate(
				machineStatusWithArgs(tc.currentArgs),
				kernelArgsResource(tc.userArgs),
			)

			assert.NoError(t, err)
			assert.True(t, initialized)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestCalculateCanonicalOrderWhenNotEqual verifies that when the current kernel args
// are not logically equal to the calculated ones, Calculate returns the canonical
// ordering (calculatedArgs: protected first, then user args) instead of preserving
// the machine's current order.
func TestCalculateCanonicalOrderWhenNotEqual(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		currentArgs []string
		userArgs    []string
		expected    []string
	}{
		{
			// User changed their kernel args: "nomodeset" replaced by "quiet".
			// Not equal → return canonical order (protected first, then user args).
			name:        "not equal: different user args",
			currentArgs: []string{siderolink, eventsSink, logging, "nomodeset"},
			userArgs:    []string{"quiet"},
			expected:    []string{siderolink, eventsSink, logging, "quiet"},
		},
		{
			// User reordered their kernel args resource.
			// Not equal → canonical order follows the desired user arg order.
			name:        "not equal: user args reordered",
			currentArgs: []string{siderolink, eventsSink, logging, "b", "a"},
			userArgs:    []string{"a", "b"},
			expected:    []string{siderolink, eventsSink, logging, "a", "b"},
		},
		{
			// User removed a kernel arg from their resource.
			// Not equal → canonical order reflects only the desired user args.
			name:        "not equal: user arg removed",
			currentArgs: []string{siderolink, eventsSink, logging, "nomodeset", "net.ifnames=0"},
			userArgs:    []string{"nomodeset"},
			expected:    []string{siderolink, eventsSink, logging, "nomodeset"},
		},
		{
			// User added a new kernel arg to their resource.
			// Not equal → canonical order includes all desired user args.
			name:        "not equal: user arg added",
			currentArgs: []string{siderolink, eventsSink, logging, "nomodeset"},
			userArgs:    []string{"nomodeset", "net.ifnames=0"},
			expected:    []string{siderolink, eventsSink, logging, "nomodeset", "net.ifnames=0"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, initialized, err := kernelargs.Calculate(
				machineStatusWithArgs(tc.currentArgs),
				kernelArgsResource(tc.userArgs),
			)

			assert.NoError(t, err)
			assert.True(t, initialized)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestCalculateProtectedArgsInResource verifies that a protected arg the user put into the KernelArgs resource
// is not added on top of the one the schematic already carries, which used to grow the args on every upgrade.
func TestCalculateProtectedArgsInResource(t *testing.T) {
	t.Parallel()

	ms := machineStatusWithArgs([]string{siderolink, eventsSink, logging})
	ka := kernelArgsResource([]string{siderolink, eventsSink, logging})

	for range 3 {
		result, initialized, err := kernelargs.Calculate(ms, ka)
		assert.NoError(t, err)
		assert.True(t, initialized)
		assert.Equal(t, []string{siderolink, eventsSink, logging}, result)

		ms.TypedSpec().Value.Schematic.KernelArgs = result
	}
}

// TestCalculateStableToday pins the inputs that are stable today and must stay stable: Calculate returns the current args untouched.
func TestCalculateStableToday(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		currentArgs []string
		userArgs    []string
	}{
		{
			name:        "duplicated protected args, no extras",
			currentArgs: []string{siderolink, eventsSink, logging, siderolink, eventsSink, logging},
		},
		{
			name:        "duplicated protected args, same extras",
			currentArgs: []string{siderolink, eventsSink, logging, siderolink, eventsSink, logging, "nomodeset"},
			userArgs:    []string{"nomodeset"},
		},
		{
			name:        "forbidden arg in schematic and resource",
			currentArgs: []string{siderolink, eventsSink, logging, "talos.platform=metal"},
			userArgs:    []string{"talos.platform=metal"},
		},
		{
			name:        "mixed entry in schematic and resource",
			currentArgs: []string{"quiet " + siderolink, "console=tty0"},
			userArgs:    []string{"quiet " + siderolink, "console=tty0"},
		},
		{
			name:        "repeated extra keys",
			currentArgs: []string{siderolink, "ip=eth0:dhcp", "ip=eth1:dhcp"},
			userArgs:    []string{"ip=eth0:dhcp", "ip=eth1:dhcp"},
		},
		{
			name:        "no protected args at all",
			currentArgs: []string{"nomodeset"},
			userArgs:    []string{"nomodeset"},
		},
		{
			name:        "existing negation of a protected arg",
			currentArgs: []string{siderolink, logging, "-talos.logging.kernel"},
			userArgs:    []string{"-talos.logging.kernel"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ms := machineStatusWithArgs(tc.currentArgs)

			result, initialized, err := kernelargs.Calculate(ms, kernelArgsResource(tc.userArgs))

			assert.NoError(t, err)
			assert.True(t, initialized)
			assert.Same(t, &ms.TypedSpec().Value.Schematic.KernelArgs[0], &result[0], "current args must be returned as they are")
		})
	}
}

// TestCalculateCleanup verifies that duplicated protected args are removed once the extras change and the machine is upgraded anyway,
// and that a protected arg hiding in a mixed entry of the schematic is kept.
func TestCalculateCleanup(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		currentArgs []string
		userArgs    []string
		expected    []string
	}{
		{
			name:        "duplicates removed on extras change",
			currentArgs: []string{siderolink, eventsSink, logging, siderolink, eventsSink, logging, "nomodeset"},
			userArgs:    []string{"quiet"},
			expected:    []string{siderolink, eventsSink, logging, "quiet"},
		},
		{
			name:        "mixed entry in schematic kept on extras change",
			currentArgs: []string{"quiet " + siderolink, "console=tty0"},
			userArgs:    []string{"console=ttyS0"},
			expected:    []string{"quiet " + siderolink, "console=ttyS0"},
		},
		{
			name:        "mixed entry in resource dropped",
			currentArgs: []string{siderolink, "nomodeset"},
			userArgs:    []string{"quiet " + siderolink, "nomodeset"},
			expected:    []string{siderolink, "nomodeset"},
		},
		{
			name:        "existing negation of a protected arg is removable",
			currentArgs: []string{siderolink, logging, "-talos.logging.kernel"},
			userArgs:    []string{},
			expected:    []string{siderolink, logging},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, initialized, err := kernelargs.Calculate(machineStatusWithArgs(tc.currentArgs), kernelArgsResource(tc.userArgs))

			assert.NoError(t, err)
			assert.True(t, initialized)
			assert.Equal(t, tc.expected, result)
		})
	}
}
