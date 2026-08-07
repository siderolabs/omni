// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package installdisk

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"

	celgo "github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/siderolabs/gen/xslices"
	blockpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// installDiskMinSize is the minimum disk size eligible for automatic install disk selection.
const installDiskMinSize = 5e9 // 5GB

const transportUSB = "usb"

// Resolution is the result of resolving the install disk of a machine.
type Resolution struct {
	// Disk is the resolved install disk dev path, empty while unresolved.
	Disk string
	// Message explains why Disk is empty, or carries extra information about the resolved disk.
	Message string
	// SelectionHash is the hash of the selection this resolution was computed from, empty when it
	// was computed without one.
	SelectionHash string
	// Candidates lists the dev paths eligible for automatic selection.
	Candidates []string
}

// SelectionHash returns the comparison hash of an install disk selection, for detecting whether a
// resolution reflects the current selection. It covers both the static disk and the disk selector,
// with the selection kind tagged into the hash input so the two kinds can never collide on equal
// content. The absent selection (nil config) hashes to the empty string, and write-time validation
// requires one of the two fields to be set, so absence and presence can never collide either.
func SelectionHash(config *omni.MachineInstallDiskConfig) string {
	if config == nil {
		return ""
	}

	spec := config.TypedSpec().Value

	input := "selector\x00" + spec.DiskSelector
	if spec.Disk != "" {
		input = "disk\x00" + spec.Disk
	}

	digest := sha256.Sum256([]byte(input))

	return hex.EncodeToString(digest[:])
}

// GetResolved returns the resolved install disk of a machine. Install disk resolution results
// MUST be consumed through this function: the status is derived asynchronously from the selection,
// so its disk is trustworthy only once it is verified to reflect the current selection, which this
// function checks by comparing the selection's hash (or its absence) against the selection hash
// the resolver stamped onto the status.
//
// The config must be a FRESH read (uncached, a cheap keyed read): it is the user intent anchoring
// the check, and its commit order relative to the cluster resources is guaranteed. The status may
// come from the cache: a stale status can only fail the check, never pass it wrongly, and the
// resolver's catch-up write retriggers any controller that has the status as an input. Both may
// be nil (not found).
//
// When inSync is false the caller must not act on the disk: skip and let the retrigger converge.
func GetResolved(config *omni.MachineInstallDiskConfig, status *omni.MachineInstallDiskStatus) (disk string, inSync bool) {
	selectionHash := SelectionHash(config)

	if status == nil {
		// no resolution exists: only in sync when there is no selection to reflect either
		return "", selectionHash == ""
	}

	if status.TypedSpec().Value.SelectionHash != selectionHash {
		return "", false
	}

	return status.TypedSpec().Value.Disk, true
}

// Resolve computes the install disk of a machine from its block devices and the optional user selection.
//
// The config may be nil, in which case the automatic default (the first candidate) is resolved.
func Resolve(machineStatus *omni.MachineStatus, config *omni.MachineInstallDiskConfig) Resolution {
	resolution := resolve(machineStatus, config)
	resolution.SelectionHash = SelectionHash(config)

	return resolution
}

func resolve(machineStatus *omni.MachineStatus, config *omni.MachineInstallDiskConfig) Resolution {
	hardware := machineStatus.TypedSpec().Value.GetHardware()
	if hardware == nil {
		return Resolution{Message: "waiting for the machine hardware information"}
	}

	disks := hardware.Blockdevices
	candidates := candidateDevPaths(disks)

	// Installs are one-shot: an installed machine keeps resolving to its system disk, no matter
	// what the selector does (including failing to evaluate), so nothing here can ever hold the
	// config updates and upgrades of an installed machine.
	if systemDisk := findSystemDisk(disks); systemDisk != nil {
		resolution := Resolution{Disk: devPath(systemDisk), Candidates: candidates}

		if config != nil {
			matches, selectorErr := matchSelection(config, disks)

			switch {
			case selectorErr != nil:
				resolution.Message = fmt.Sprintf("Talos is already installed on %s, and the install disk selector failed to evaluate: %v", resolution.Disk, selectorErr)
			case !slices.Contains(xslices.Map(matches, devPath), resolution.Disk):
				resolution.Message = fmt.Sprintf("Talos is already installed on %s, the install disk selection applies to the next install only", resolution.Disk)
			}
		}

		return resolution
	}

	if config != nil {
		matches, selectorErr := matchSelection(config, disks)
		if selectorErr != nil {
			return Resolution{
				Message:    fmt.Sprintf("install disk selector evaluation failed: %v", selectorErr),
				Candidates: candidates,
			}
		}

		if disk := config.TypedSpec().Value.Disk; disk != "" {
			if len(matches) == 0 {
				return Resolution{
					Message:    fmt.Sprintf("the selected install disk %s is not present on the machine", disk),
					Candidates: candidates,
				}
			}

			return Resolution{Disk: disk, Message: "selected by the install disk selection", Candidates: candidates}
		}

		return resolveSelector(matches, candidates)
	}

	if len(candidates) == 0 {
		return Resolution{Message: "no disks are eligible for installation", Candidates: candidates}
	}

	return Resolution{Disk: candidates[0], Candidates: candidates}
}

// resolveSelector picks the install disk from the selector matches.
//
// Non-composite matches win over composite ones and tie-break first by dev path, as Talos does.
// A composite device (e.g. an md array) is resolved only as the sole match: when several disks
// match and the best of them is composite, the resolution is held instead, because picking a
// composite automatically is how an install lands on e.g. the dm-crypt volume of another disk.
// Matching multiple disks resolves deterministically but is worth the user's attention, and a
// resolution driven by a selection (a selector here, the static disk in the caller) always says
// so, so a selection surviving from a previous cluster is visible rather than passing as the
// automatic default.
func resolveSelector(matches []*specs.MachineStatusSpec_HardwareStatus_BlockDevice, candidates []string) Resolution {
	if len(matches) == 0 {
		return Resolution{Message: "no disk matches the install disk selector", Candidates: candidates}
	}

	slices.SortFunc(matches, func(a, b *specs.MachineStatusSpec_HardwareStatus_BlockDevice) int {
		if compositeA, compositeB := omni.IsCompositeBlockDevice(a), omni.IsCompositeBlockDevice(b); compositeA != compositeB {
			if compositeA {
				return 1
			}

			return -1
		}

		return cmp.Compare(devPath(a), devPath(b))
	})

	if len(matches) > 1 && omni.IsCompositeBlockDevice(matches[0]) {
		return Resolution{
			Message:    fmt.Sprintf("%d disks match the install disk selector and all of them are composite devices, refusing to pick one automatically", len(matches)),
			Candidates: candidates,
		}
	}

	resolution := Resolution{Disk: devPath(matches[0]), Candidates: candidates}

	if len(matches) > 1 {
		resolution.Message = fmt.Sprintf("%d disks match the install disk selector, using %s", len(matches), resolution.Disk)
	} else {
		resolution.Message = "selected by the install disk selector"
	}

	return resolution
}

// matchSelection returns the disks matching the selection: a static disk matches by dev path,
// a disk selector by CEL evaluation against every disk.
func matchSelection(config *omni.MachineInstallDiskConfig, disks []*specs.MachineStatusSpec_HardwareStatus_BlockDevice) ([]*specs.MachineStatusSpec_HardwareStatus_BlockDevice, error) {
	if disk := config.TypedSpec().Value.Disk; disk != "" {
		return xslices.Filter(disks, func(blockDevice *specs.MachineStatusSpec_HardwareStatus_BlockDevice) bool {
			return devPath(blockDevice) == disk
		}), nil
	}

	return matchSelector(config.TypedSpec().Value.DiskSelector, disks)
}

// matchSelector evaluates the selector against every disk, compiling the expression once.
//
// Machinery's cel.Expression rebuilds the evaluation program on every call, so this uses the
// cel-go API directly with the same environment and the same parse and check semantics.
func matchSelector(selector string, disks []*specs.MachineStatusSpec_HardwareStatus_BlockDevice) ([]*specs.MachineStatusSpec_HardwareStatus_BlockDevice, error) {
	env := celenv.DiskLocator()

	ast, issues := env.Compile(selector)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}

	if outputType := ast.OutputType(); !outputType.IsExactType(types.BoolType) {
		return nil, fmt.Errorf("expression output type is %s, expected bool", outputType)
	}

	program, err := env.Program(ast)
	if err != nil {
		return nil, err
	}

	var matches []*specs.MachineStatusSpec_HardwareStatus_BlockDevice

	for _, disk := range disks {
		matched, err := evalDisk(program, disk)
		if err != nil {
			return nil, err
		}

		if matched {
			matches = append(matches, disk)
		}
	}

	return matches, nil
}

func evalDisk(program celgo.Program, disk *specs.MachineStatusSpec_HardwareStatus_BlockDevice) (bool, error) {
	out, _, err := program.Eval(map[string]any{
		"disk":        toDiskSpec(disk),
		"system_disk": disk.SystemDisk,
	})
	if err != nil {
		return false, fmt.Errorf("error evaluating the selector against %q: %w", devPath(disk), err)
	}

	matched, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("expression output type is %s, expected bool", out.Type())
	}

	return matched, nil
}

// toDiskSpec converts the machine status block device into the Talos disk spec the CEL disk
// locator environment evaluates against, so selectors behave exactly as they do in Talos.
func toDiskSpec(disk *specs.MachineStatusSpec_HardwareStatus_BlockDevice) *blockpb.DiskSpec {
	return &blockpb.DiskSpec{
		Size:            disk.Size,
		IoSize:          disk.IoSize,
		SectorSize:      disk.SectorSize,
		Readonly:        disk.Readonly,
		Model:           disk.Model,
		Serial:          disk.Serial,
		Modalias:        disk.Modalias,
		Wwid:            disk.Wwid,
		BusPath:         disk.BusPath,
		SubSystem:       disk.SubSystem,
		Transport:       disk.Transport,
		Rotational:      disk.Rotational,
		Cdrom:           disk.Cdrom,
		DevPath:         devPath(disk),
		PrettySize:      disk.PrettySize,
		SecondaryDisks:  disk.SecondaryDisks,
		Uuid:            disk.Uuid,
		Symlinks:        disk.Symlinks,
		FirmwareVersion: disk.FirmwareVersion,
	}
}

// candidateDevPaths returns the disks eligible for automatic install disk selection: writable,
// not a CD, above the minimum size and not a composite device, smallest first with USB disks last.
func candidateDevPaths(disks []*specs.MachineStatusSpec_HardwareStatus_BlockDevice) []string {
	candidates := xslices.Filter(disks, func(disk *specs.MachineStatusSpec_HardwareStatus_BlockDevice) bool {
		return !disk.Readonly && disk.Type != storage.Disk_CD.String() && disk.Size > installDiskMinSize && !omni.IsCompositeBlockDevice(disk)
	})

	slices.SortFunc(candidates, func(a, b *specs.MachineStatusSpec_HardwareStatus_BlockDevice) int {
		if a.Transport == transportUSB && b.Transport != transportUSB {
			return 1
		} else if b.Transport == transportUSB && a.Transport != transportUSB {
			return -1
		}

		// tie-break equal sizes by dev path: the block device order in the machine status is not
		// stable across polls, and the automatic default must not flip between equal disks
		return cmp.Or(cmp.Compare(a.Size, b.Size), cmp.Compare(devPath(a), devPath(b)))
	})

	return xslices.Map(candidates, devPath)
}

func findSystemDisk(disks []*specs.MachineStatusSpec_HardwareStatus_BlockDevice) *specs.MachineStatusSpec_HardwareStatus_BlockDevice {
	for _, disk := range disks {
		if disk.SystemDisk {
			return disk
		}
	}

	return nil
}

// devPath returns the disk's dev path, falling back to the linux name for machine status data
// polled before the dev path field existed (an offline machine never re-polls).
func devPath(disk *specs.MachineStatusSpec_HardwareStatus_BlockDevice) string {
	return cmp.Or(disk.DevPath, disk.LinuxName)
}
