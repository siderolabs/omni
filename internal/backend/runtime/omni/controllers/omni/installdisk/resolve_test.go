// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package installdisk_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/installdisk"
)

// TestResolveDiskEvaluations pins the full per-disk evaluation shape: the grouping and order,
// the eligibility flags, the reasons, and the member lists the UI builds its entries from.
func TestResolveDiskEvaluations(t *testing.T) {
	t.Parallel()

	disks := []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
		{DevPath: "/dev/sda", Size: 8e9},
		{DevPath: "/dev/sdb", Size: 10e9},
		// an md array carrying a dm volume: built on top of disks AND a member of one itself
		{DevPath: "/dev/md0", Size: 18e9, SecondaryDisks: []string{"sda", "sdb"}},
		// sda is also a member of this second array: the smallest parent dev path must win, so
		// the reason cannot flip with the unstable block device order
		{DevPath: "/dev/md1", Size: 8e9, SecondaryDisks: []string{"sda"}},
		{DevPath: "/dev/dm-0", Size: 18e9, BusPath: "/virtual", SecondaryDisks: []string{"md0"}},
		{DevPath: "/dev/dm-1", Size: 18e9, BusPath: "/virtual", Readonly: true},
		{DevPath: "/dev/sdc", Size: 10e9, Transport: "usb"},
		{DevPath: "/dev/sdd", Size: 8e9},
		{DevPath: "/dev/sde", Size: 8e9, Readonly: true},
		{DevPath: "/dev/sr0", Size: 8e9, Type: "CD"},
		{DevPath: "/dev/sdf", Size: 4e9},
		{LinuxName: "/dev/sdg", Size: 8e9},
	}

	resolution := installdisk.Resolve(machineStatusWithDisks(disks...), nil)

	expected := []*specs.MachineInstallDiskStatusSpec_Disk{
		// the candidates (empty skip reason) come first, smallest first with USB disks last
		{DevPath: "/dev/sdd", Selectable: true},
		{DevPath: "/dev/sdc", Selectable: true},
		// then the disks only an explicit selection can target
		{DevPath: "/dev/dm-0", Selectable: true, SkipReason: "built on top of other disks", Members: []string{"md0"}},
		{DevPath: "/dev/md1", Selectable: true, SkipReason: "built on top of other disks", Members: []string{"sda"}},
		// Then the ineligible rest, by dev path. The membership of md0 wins over it being built
		// on top of other disks itself, and the read-only dm-1 is rejected before it could
		// count as explicitly selectable.
		{DevPath: "/dev/dm-1", SkipReason: "read-only"},
		{DevPath: "/dev/md0", SkipReason: "a member of /dev/dm-0", Members: []string{"sda", "sdb"}},
		{DevPath: "/dev/sda", SkipReason: "a member of /dev/md0"},
		{DevPath: "/dev/sdb", SkipReason: "a member of /dev/md0"},
		{DevPath: "/dev/sde", SkipReason: "read-only"},
		{DevPath: "/dev/sdf", SkipReason: "smaller than the minimum size for an install disk"},
		{DevPath: "/dev/sdg", SkipReason: "reported without the disk details, waiting for the machine to be polled again"},
		{DevPath: "/dev/sr0", SkipReason: "a CD-ROM drive"},
	}

	assert.Len(t, resolution.Disks, len(expected))

	for i, disk := range resolution.Disks {
		assert.Equal(t, expected[i].DevPath, disk.DevPath)
		assert.Equal(t, expected[i].Selectable, disk.Selectable, disk.DevPath)
		assert.Equal(t, expected[i].SkipReason, disk.SkipReason, disk.DevPath)
		assert.Equal(t, expected[i].Members, disk.Members, disk.DevPath)
	}

	assert.Equal(t, "/dev/sdd", resolution.Disk)
}

// candidatePathsOf derives the automatic candidate dev paths from the per-disk evaluations of
// an install disk resolution.
func candidatePathsOf(resolution installdisk.Resolution) []string {
	var paths []string

	for _, disk := range resolution.Disks {
		if disk.SkipReason == "" {
			paths = append(paths, disk.DevPath)
		}
	}

	return paths
}

func machineStatusWithDisks(disks ...*specs.MachineStatusSpec_HardwareStatus_BlockDevice) *omni.MachineStatus {
	machineStatus := omni.NewMachineStatus("test-machine")

	if disks != nil {
		machineStatus.TypedSpec().Value.Hardware = &specs.MachineStatusSpec_HardwareStatus{Blockdevices: disks}
	}

	return machineStatus
}

func diskConfig(selector string) *omni.MachineInstallDiskConfig {
	config := omni.NewMachineInstallDiskConfig("test-machine")
	config.TypedSpec().Value.DiskSelector = selector

	return config
}

func staticDiskConfig(disk string) *omni.MachineInstallDiskConfig {
	config := omni.NewMachineInstallDiskConfig("test-machine")
	config.TypedSpec().Value.Disk = disk

	return config
}

func TestResolveAutomatic(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name               string
		disks              []*specs.MachineStatusSpec_HardwareStatus_BlockDevice
		expectedDisk       string
		expectedCandidates []string
		expectMessage      bool
	}{
		{
			name:          "no hardware",
			expectMessage: true,
		},
		{
			name:          "no eligible disks",
			disks:         []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{},
			expectMessage: true,
		},
		{
			name: "single disk",
			disks: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
				{DevPath: "/dev/sda", Size: 8e9},
			},
			expectedDisk:       "/dev/sda",
			expectedCandidates: []string{"/dev/sda"},
		},
		{
			name: "all filtered out",
			disks: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
				{DevPath: "/dev/sda", Size: 4e9},                 // too small
				{DevPath: "/dev/sr0", Size: 8e9, Type: "CD"},     // CD
				{DevPath: "/dev/sdb", Size: 8e9, Readonly: true}, // readonly
			},
			expectMessage: true,
		},
		{
			name: "stacked disks and their members are excluded",
			disks: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
				{DevPath: "/dev/sda", Size: 8e9, Transport: "usb"},
				{DevPath: "/dev/dm-0", Size: 8e9, BusPath: "/virtual"},
				{DevPath: "/dev/sdb", Size: 10e9},
				{DevPath: "/dev/sdc", Size: 14e9},
				{DevPath: "/dev/md0", Size: 7e9, SecondaryDisks: []string{"sdb", "sdc"}},
			},
			expectedDisk:       "/dev/sda",
			expectedCandidates: []string{"/dev/sda"},
		},
		{
			name: "no candidates when every disk is an array or a member of one",
			disks: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
				{DevPath: "/dev/sda", Size: 8e9},
				{DevPath: "/dev/sdb", Size: 8e9},
				{DevPath: "/dev/md0", Size: 8e9, SecondaryDisks: []string{"sda", "sdb"}},
			},
			expectMessage: true,
		},
		{
			name: "smallest eligible wins",
			disks: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
				{DevPath: "/dev/sda", Size: 25165824000, Transport: "sata"},
				{DevPath: "/dev/vdb", Size: 6442450944, Transport: "usb"},
				{DevPath: "/dev/vda", Size: 6442450944, Transport: "virtio"},
				{DevPath: "/dev/vdc", Size: 6442450943, Transport: "usb"},
			},
			expectedDisk:       "/dev/vda",
			expectedCandidates: []string{"/dev/vda", "/dev/sda", "/dev/vdc", "/dev/vdb"},
		},
		{
			name: "system disk wins",
			disks: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
				{DevPath: "/dev/sda", Size: 8e9},
				{DevPath: "/dev/sdb", Size: 10e9, SystemDisk: true},
				{DevPath: "/dev/sdc", Size: 14e9},
			},
			expectedDisk: "/dev/sdb",
		},
		{
			name: "equal sizes tie-break by dev path",
			disks: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
				{DevPath: "/dev/sdb", Size: 10e9},
				{DevPath: "/dev/sda", Size: 10e9},
			},
			expectedDisk:       "/dev/sda",
			expectedCandidates: []string{"/dev/sda", "/dev/sdb"},
		},
		{
			// Data polled by older Omni versions has no dev paths and no disk relationships, so
			// an md array member is indistinguishable from a plain disk there. Automatic
			// selection holds until the machine is polled again.
			name: "pre-upgrade data without dev paths is not eligible",
			disks: []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
				{LinuxName: "/dev/sda", Size: 8e9},
			},
			expectMessage: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resolution := installdisk.Resolve(machineStatusWithDisks(tt.disks...), nil)

			assert.Equal(t, tt.expectedDisk, resolution.Disk)
			assert.Empty(t, resolution.SelectionHash, "a resolution without a selection must have an empty selection hash")

			if tt.expectedCandidates != nil {
				assert.Equal(t, tt.expectedCandidates, candidatePathsOf(resolution))
			}

			if tt.expectMessage {
				assert.NotEmpty(t, resolution.Message)
			}
		})
	}
}

func TestResolveSelector(t *testing.T) {
	t.Parallel()

	disks := []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
		{DevPath: "/dev/sda", Size: 8e9, Transport: "sata", Serial: "serial-a"},
		{DevPath: "/dev/sdb", Size: 10e9, Transport: "sata", Serial: "serial-b"},
		{DevPath: "/dev/md0", Size: 20e9, SecondaryDisks: []string{"sda", "sdb"}},
		{DevPath: "/dev/dm-0", Size: 9e9, BusPath: "/virtual"},
	}

	for _, tt := range []struct {
		name            string
		selector        string
		expectedDisk    string
		expectedMessage string
	}{
		{
			name:            "single match by path",
			selector:        `disk.dev_path == "/dev/sdb"`,
			expectedDisk:    "/dev/sdb",
			expectedMessage: "selected by the install disk selector",
		},
		{
			name:            "single match by serial",
			selector:        `disk.serial == "serial-a"`,
			expectedDisk:    "/dev/sda",
			expectedMessage: "selected by the install disk selector",
		},
		{
			name:            "no match holds",
			selector:        `disk.serial == "no-such-serial"`,
			expectedDisk:    "",
			expectedMessage: "no disk matches the install disk selector",
		},
		{
			name:            "multiple matches prefer regular disks first by dev path",
			selector:        `disk.size > 7000000000u`,
			expectedDisk:    "/dev/sda",
			expectedMessage: "4 disks match the install disk selector, using /dev/sda",
		},
		{
			name:            "a disk built on other disks resolves as the sole match",
			selector:        `disk.dev_path == "/dev/md0"`,
			expectedDisk:    "/dev/md0",
			expectedMessage: "selected by the install disk selector",
		},
		{
			name:            "multiple matches with no regular disk resolve nothing",
			selector:        `disk.size == 20000000000u || disk.size == 9000000000u`, // matches md0 and dm-0 only
			expectedDisk:    "",
			expectedMessage: "2 disks match the install disk selector and all of them are built on top of other disks",
		},
		{
			name:            "evaluation failure is a message, not an error",
			selector:        `disk.dev_path`, // not a boolean expression
			expectedDisk:    "",
			expectedMessage: "install disk selector evaluation failed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := diskConfig(tt.selector)
			resolution := installdisk.Resolve(machineStatusWithDisks(disks...), config)

			assert.Equal(t, tt.expectedDisk, resolution.Disk)
			assert.Equal(t, installdisk.SelectionHash(config), resolution.SelectionHash)
			assert.NotEmpty(t, resolution.SelectionHash)

			if tt.expectedMessage != "" {
				assert.Contains(t, resolution.Message, tt.expectedMessage)
			} else {
				assert.Empty(t, resolution.Message)
			}
		})
	}
}

func TestResolveInstalledMachine(t *testing.T) {
	t.Parallel()

	disks := []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
		{DevPath: "/dev/sda", Size: 8e9, SystemDisk: true},
		{DevPath: "/dev/sdb", Size: 10e9},
	}

	// An installed machine without a selection carries the plain installed message, so the UI
	// can explain why the disk is not changeable.
	resolution := installdisk.Resolve(machineStatusWithDisks(disks...), nil)

	assert.Equal(t, "/dev/sda", resolution.Disk)
	assert.Equal(t, "Talos is already installed on /dev/sda", resolution.Message)
	assert.Empty(t, resolution.SelectionHash)

	// A selector disagreeing with the system disk has no effect, and the message says so. The
	// selection hash must be present on this path too. Otherwise, an installed machine with a
	// hash-less install disk resolution would fail the staleness check forever and never
	// re-generate its config.
	config := diskConfig(`disk.dev_path == "/dev/sdb"`)
	resolution = installdisk.Resolve(machineStatusWithDisks(disks...), config)

	assert.Equal(t, "/dev/sda", resolution.Disk)
	assert.Contains(t, resolution.Message, "takes effect only if the machine is wiped")
	assert.Equal(t, installdisk.SelectionHash(config), resolution.SelectionHash)

	// a selector matching the system disk gets the plain installed message
	resolution = installdisk.Resolve(machineStatusWithDisks(disks...), diskConfig(`disk.dev_path == "/dev/sda"`))

	assert.Equal(t, "/dev/sda", resolution.Disk)
	assert.Equal(t, "Talos is already installed on /dev/sda", resolution.Message)

	// A selector that compiles but fails at evaluation time must never erase the resolved disk
	// of an installed machine, as an empty disk would block its config updates and upgrades.
	resolution = installdisk.Resolve(machineStatusWithDisks(disks...), diskConfig(`disk.symlinks[0] == "no-such-symlink"`))

	assert.Equal(t, "/dev/sda", resolution.Disk)
	assert.Contains(t, resolution.Message, "failed to evaluate")

	// same for the static selection, disagreeing with the system disk has no effect and says so
	staticConfig := staticDiskConfig("/dev/sdb")
	resolution = installdisk.Resolve(machineStatusWithDisks(disks...), staticConfig)

	assert.Equal(t, "/dev/sda", resolution.Disk)
	assert.Contains(t, resolution.Message, "takes effect only if the machine is wiped")
	assert.Equal(t, installdisk.SelectionHash(staticConfig), resolution.SelectionHash)

	// a static selection matching the system disk gets the plain installed message
	resolution = installdisk.Resolve(machineStatusWithDisks(disks...), staticDiskConfig("/dev/sda"))

	assert.Equal(t, "/dev/sda", resolution.Disk)
	assert.Equal(t, "Talos is already installed on /dev/sda", resolution.Message)
}

func TestResolveStaticDisk(t *testing.T) {
	t.Parallel()

	disks := []*specs.MachineStatusSpec_HardwareStatus_BlockDevice{
		{DevPath: "/dev/sda", Size: 8e9},
		{DevPath: "/dev/sdb", Size: 10e9},
		{LinuxName: "/dev/sdc", Size: 10e9},
		{DevPath: "/dev/md0", Size: 20e9, SecondaryDisks: []string{"sda", "sdb"}},
	}

	// A present disk resolves as-is, even when it is not among the automatic candidates - here
	// it is an md array member, which the automatic selection never picks. The message makes the
	// selection visible, so that a selection surviving from a previous cluster does not look
	// like the automatic default in the UI.
	config := staticDiskConfig("/dev/sdb")
	resolution := installdisk.Resolve(machineStatusWithDisks(disks...), config)

	assert.Equal(t, "/dev/sdb", resolution.Disk)
	assert.Equal(t, "selected by the install disk selection", resolution.Message)
	assert.Equal(t, installdisk.SelectionHash(config), resolution.SelectionHash)

	// the linux name fallback applies to static matching too
	resolution = installdisk.Resolve(machineStatusWithDisks(disks...), staticDiskConfig("/dev/sdc"))

	assert.Equal(t, "/dev/sdc", resolution.Disk)

	// a disk that is not present on the machine blocks the install disk resolution instead of installing blind
	resolution = installdisk.Resolve(machineStatusWithDisks(disks...), staticDiskConfig("/dev/nvme0n1"))

	assert.Empty(t, resolution.Disk)
	assert.Contains(t, resolution.Message, "the selected install disk /dev/nvme0n1 is not present on the machine")
}

// TestResolveFieldMappingParity fills every field of the block device data with a
// distinguishable non-zero value, and asserts that a selector on each mapped field matches. This
// way, a missed or misrouted field in the mapping to the Talos disk spec cannot silently
// evaluate against a zero value.
func TestResolveFieldMappingParity(t *testing.T) {
	t.Parallel()

	fullDisk := &specs.MachineStatusSpec_HardwareStatus_BlockDevice{
		Size:            111e9,
		Model:           "model-x",
		Serial:          "serial-x",
		Wwid:            "wwid-x",
		BusPath:         "/pci/bus/path",
		Transport:       "nvme",
		DevPath:         "/dev/nvme17n1",
		IoSize:          131072,
		SectorSize:      4096,
		Modalias:        "modalias-x",
		SubSystem:       "/sys/class/block",
		Rotational:      true,
		Cdrom:           true,
		PrettySize:      "111 GB",
		SecondaryDisks:  []string{"nvme16n1"},
		Uuid:            "uuid-x",
		Symlinks:        []string{"/dev/disk/by-id/nvme-model-x"},
		FirmwareVersion: "fw-1.2.3",
		Readonly:        true,
		SystemDisk:      false, // must stay false: true short-circuits into the system-disk branch, unreachable through Resolve
	}

	for _, tt := range []struct {
		name     string
		selector string
	}{
		{"size", `disk.size == 111000000000u`},
		{"io_size", `disk.io_size == 131072u`},
		{"sector_size", `disk.sector_size == 4096u`},
		{"readonly", `disk.readonly`},
		{"model", `disk.model == "model-x"`},
		{"serial", `disk.serial == "serial-x"`},
		{"modalias", `disk.modalias == "modalias-x"`},
		{"wwid", `disk.wwid == "wwid-x"`},
		{"bus_path", `disk.bus_path == "/pci/bus/path"`},
		{"sub_system", `disk.sub_system == "/sys/class/block"`},
		{"transport", `disk.transport == "nvme"`},
		{"rotational", `disk.rotational`},
		{"cdrom", `disk.cdrom`},
		{"dev_path", `disk.dev_path == "/dev/nvme17n1"`},
		{"pretty_size", `disk.pretty_size == "111 GB"`},
		{"secondary_disks", `"nvme16n1" in disk.secondary_disks`},
		{"uuid", `disk.uuid == "uuid-x"`},
		{"symlinks", `disk.symlinks.exists(s, s == "/dev/disk/by-id/nvme-model-x")`},
		{"firmware_version", `disk.firmware_version == "fw-1.2.3"`},
		{"system_disk", `!system_disk`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resolution := installdisk.Resolve(machineStatusWithDisks(fullDisk), diskConfig(tt.selector))

			assert.Equal(t, "/dev/nvme17n1", resolution.Disk,
				fmt.Sprintf("selector %q must match the disk, a zero value leaked into the field mapping", tt.selector))
		})
	}
}

func TestSelectionHash(t *testing.T) {
	t.Parallel()

	assert.Empty(t, installdisk.SelectionHash(nil), "no selection must hash to the empty string so absence and presence can never collide")
	assert.Len(t, installdisk.SelectionHash(diskConfig(`disk.dev_path == "/dev/sda"`)), 64)
	assert.Equal(t, installdisk.SelectionHash(diskConfig("x")), installdisk.SelectionHash(diskConfig("x")))
	assert.NotEqual(t, installdisk.SelectionHash(diskConfig("x")), installdisk.SelectionHash(diskConfig("y")))
	assert.NotEqual(t, installdisk.SelectionHash(staticDiskConfig("x")), installdisk.SelectionHash(diskConfig("x")),
		"a static disk and a selector with the same content are different selections")
	assert.NotEqual(t, installdisk.SelectionHash(staticDiskConfig("\x00true//")), installdisk.SelectionHash(diskConfig("true//\x00")),
		"the kind tag must prevent cross-kind collisions engineered around the field separator")
}

// TestGetResolved pins the staleness contract of the accessor. The resolution is usable only
// when its selection hash matches the current selection, or both are absent.
func TestGetResolved(t *testing.T) {
	t.Parallel()

	status := func(disk, selectionHash string) *omni.MachineInstallDiskStatus {
		res := omni.NewMachineInstallDiskStatus("test-machine")
		res.TypedSpec().Value.Disk = disk
		res.TypedSpec().Value.SelectionHash = selectionHash

		return res
	}

	const selector = `disk.dev_path == "/dev/vdb"`

	selectorHash := installdisk.SelectionHash(diskConfig(selector))

	for _, tt := range []struct {
		name           string
		config         *omni.MachineInstallDiskConfig
		status         *omni.MachineInstallDiskStatus
		expectedDisk   string
		expectedInSync bool
	}{
		{
			name:           "no selection, resolution reflects it",
			status:         status("/dev/vda", ""),
			expectedDisk:   "/dev/vda",
			expectedInSync: true,
		},
		{
			name:           "selection matches the observation",
			config:         diskConfig(selector),
			status:         status("/dev/vdb", selectorHash),
			expectedDisk:   "/dev/vdb",
			expectedInSync: true,
		},
		{
			name:           "static disk selection matches the observation",
			config:         staticDiskConfig("/dev/vdb"),
			status:         status("/dev/vdb", installdisk.SelectionHash(staticDiskConfig("/dev/vdb"))),
			expectedDisk:   "/dev/vdb",
			expectedInSync: true,
		},
		{
			name:           "selection not absorbed yet",
			config:         diskConfig(selector),
			status:         status("/dev/vda", ""),
			expectedInSync: false,
		},
		{
			name:           "selection changed since the observation",
			config:         diskConfig(`disk.serial == "other"`),
			status:         status("/dev/vdb", selectorHash),
			expectedInSync: false,
		},
		{
			name:           "selection kind changed since the observation",
			config:         staticDiskConfig("/dev/vdb"),
			status:         status("/dev/vdb", selectorHash),
			expectedInSync: false,
		},
		{
			name:           "selection deleted since the observation",
			status:         status("/dev/vdb", selectorHash),
			expectedInSync: false,
		},
		{
			name:           "no resolution at all, no selection",
			expectedInSync: true,
		},
		{
			name:           "no resolution at all, selection exists",
			config:         diskConfig(selector),
			expectedInSync: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			disk, inSync := installdisk.GetResolved(tt.config, tt.status)

			assert.Equal(t, tt.expectedInSync, inSync)
			assert.Equal(t, tt.expectedDisk, disk)
		})
	}
}
