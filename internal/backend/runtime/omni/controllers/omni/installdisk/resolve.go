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
	"path"
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
	// Disk is the resolved install disk dev path. It is empty while unresolved.
	Disk string
	// Message explains why Disk is empty, or carries extra information about the resolved disk.
	Message string
	// SelectionHash is the hash of the selection this install disk resolution was computed from.
	// It is empty when there was no selection.
	SelectionHash string
	// Disks lists every disk of the machine with its install disk eligibility.
	Disks []*specs.MachineInstallDiskStatusSpec_Disk
}

// SelectionHash returns the hash of an install disk selection. Comparing it against the hash
// stored on an install disk resolution tells whether the resolution is stale.
//
// The selection kind is part of the hash input, so a static disk and a selector with the same
// content hash differently. A nil config hashes to the empty string, and the validation requires
// one of the two fields to be set, so a missing selection and a present one always differ too.
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

// GetResolved returns the resolved install disk of a machine.
//
// The install disk resolution is computed asynchronously from the selection, so it can be stale.
// Because of this, the resolution must always be read through this function. It compares the hash
// of the current selection against the hash stored on the resolution, and reports the result as
// inSync.
// When inSync is false, the caller must not use the disk. It should skip the work instead, as it
// gets triggered again when the resolution catches up.
//
// The config must be read without the cache (a cheap read, as it is a read by ID). This is
// because it is the user intent the check is built on. The status can come from the cache, as a
// stale status can only fail the check, never wrongly pass it. Both can be nil (not found).
func GetResolved(config *omni.MachineInstallDiskConfig, status *omni.MachineInstallDiskStatus) (disk string, inSync bool) {
	selectionHash := SelectionHash(config)

	if status == nil {
		// A missing resolution carries no hash, so it is in sync only with a missing selection.
		// Reporting false here instead would win nothing: an existing in-sync resolution can
		// still carry an empty disk (e.g., no eligible disks), so the caller must hold on the
		// empty disk either way.
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
	evaluations := evaluateDisks(disks)

	// Install is a one-time operation, so an installed machine always resolves to its system
	// disk, no matter what the selection says (even when the selector fails to evaluate). This
	// way, a bad selection can never block the config updates and upgrades of an installed
	// machine.
	if systemDisk := findSystemDisk(disks); systemDisk != nil {
		resolution := Resolution{Disk: devPath(systemDisk), Disks: evaluations}
		resolution.Message = fmt.Sprintf("Talos is already installed on %s", resolution.Disk)

		if config != nil {
			matches, selectorErr := matchSelection(config, disks)

			switch {
			case selectorErr != nil:
				resolution.Message = fmt.Sprintf("Talos is already installed on %s, and the install disk selector failed to evaluate: %v", resolution.Disk, selectorErr)
			case !slices.Contains(xslices.Map(matches, devPath), resolution.Disk):
				// Only the bare-metal infra provider can bring a machine back to the uninstalled
				// state: it wipes the machine after a reset.
				resolution.Message = fmt.Sprintf("Talos is already installed on %s, the install disk selection takes effect only if the machine is wiped and installed again", resolution.Disk)
			}
		}

		return resolution
	}

	if config != nil {
		matches, selectorErr := matchSelection(config, disks)
		if selectorErr != nil {
			return Resolution{
				Message: fmt.Sprintf("install disk selector evaluation failed: %v", selectorErr),
				Disks:   evaluations,
			}
		}

		if disk := config.TypedSpec().Value.Disk; disk != "" {
			if len(matches) == 0 {
				return Resolution{
					Message: fmt.Sprintf("the selected install disk %s is not present on the machine", disk),
					Disks:   evaluations,
				}
			}

			return Resolution{Disk: disk, Message: "selected by the install disk selection", Disks: evaluations}
		}

		return resolveSelector(matches, evaluations)
	}

	// the evaluations list the candidates first, so the automatic default is the first entry
	// when it is a candidate
	if len(evaluations) == 0 || evaluations[0].SkipReason != "" {
		return Resolution{Message: "no disks are eligible for installation", Disks: evaluations}
	}

	return Resolution{Disk: evaluations[0].DevPath, Disks: evaluations}
}

// resolveSelector picks the install disk from the selector matches.
//
// Regular disks win over the ones built on top of other disks (md arrays, device-mapper
// volumes), and ties are broken by the dev path, as Talos does. A disk built on top of other
// disks is only picked when it is the only match. When multiple disks match and there is no
// regular disk among them, nothing is picked. This is because picking such a disk automatically
// could land an install on, e.g., the dm-crypt volume of another disk.
//
// Multiple matches are deterministically resolved, but the user should know about them, so the
// install disk resolution carries a message. A single match carries a message too (same for the
// static disk in the caller). This way, a selection surviving from a previous cluster is visible
// in the UI instead of looking like the automatic default.
func resolveSelector(matches []*specs.MachineStatusSpec_HardwareStatus_BlockDevice, evaluations []*specs.MachineInstallDiskStatusSpec_Disk) Resolution {
	if len(matches) == 0 {
		return Resolution{Message: "no disk matches the install disk selector", Disks: evaluations}
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
			Message: fmt.Sprintf("%d disks match the install disk selector and all of them are built on top of other disks, refusing to pick one automatically", len(matches)),
			Disks:   evaluations,
		}
	}

	resolution := Resolution{Disk: devPath(matches[0]), Disks: evaluations}

	if len(matches) > 1 {
		resolution.Message = fmt.Sprintf("%d disks match the install disk selector, using %s", len(matches), resolution.Disk)
	} else {
		resolution.Message = "selected by the install disk selector"
	}

	return resolution
}

// matchSelection returns the disks matching the selection. A static disk matches by its dev
// path, a disk selector is evaluated against every disk.
func matchSelection(config *omni.MachineInstallDiskConfig, disks []*specs.MachineStatusSpec_HardwareStatus_BlockDevice) ([]*specs.MachineStatusSpec_HardwareStatus_BlockDevice, error) {
	if disk := config.TypedSpec().Value.Disk; disk != "" {
		return xslices.Filter(disks, func(blockDevice *specs.MachineStatusSpec_HardwareStatus_BlockDevice) bool {
			return devPath(blockDevice) == disk
		}), nil
	}

	return matchSelector(config.TypedSpec().Value.DiskSelector, disks)
}

// matchSelector evaluates the selector against every disk, compiling the expression only once.
//
// It uses the cel-go API directly with the same environment and the same parse and check
// semantics as the machinery. This is because the machinery API rebuilds the evaluation program
// on every call.
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

// toDiskSpec converts the machine status block device into the Talos disk spec. The CEL disk
// locator environment evaluates against this spec, so selectors behave exactly as they do in
// Talos.
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

// evaluation pairs a block device with its install disk classification, so that the sort can
// reach both. An empty skip reason means a candidate, and candidates are always selectable.
type evaluation struct {
	disk       *specs.MachineStatusSpec_HardwareStatus_BlockDevice
	skipReason string
	selectable bool
}

// evaluateDisks classifies every disk of the machine for the install disk selection:
// candidates first (smallest first, USB disks last), then the disks only an explicit
// selection can target, then the ineligible rest.
func evaluateDisks(disks []*specs.MachineStatusSpec_HardwareStatus_BlockDevice) []*specs.MachineInstallDiskStatusSpec_Disk {
	// Members of another disk, mapped to that disk. When a disk is a member of several disks,
	// the smallest dev path wins, so the skip reason does not flip with the block device order.
	memberOf := map[string]string{}

	for _, disk := range disks {
		for _, member := range disk.SecondaryDisks {
			if parent, ok := memberOf[member]; !ok || devPath(disk) < parent {
				memberOf[member] = devPath(disk)
			}
		}
	}

	classify := func(disk *specs.MachineStatusSpec_HardwareStatus_BlockDevice) (skipReason string, selectable bool) {
		// Old disk data has no dev path and no disk relationships, so an md array member looks
		// like a plain disk there. Do not trust it, wait for the next poll. Installed machines
		// and explicit selections keep the linux name fallback.
		if disk.DevPath == "" {
			return "reported without the disk details, waiting for the machine to be polled again", false
		}

		// installing onto a member would break the disk built on it, so members are reachable
		// only through a disk selector
		if parent, isMember := memberOf[path.Base(disk.DevPath)]; isMember {
			return "a member of " + parent, false
		}

		switch {
		case disk.Readonly:
			return "read-only", false
		case disk.Type == storage.Disk_CD.String():
			return "a CD-ROM drive", false
		case disk.Size <= installDiskMinSize:
			return "smaller than the minimum size for an install disk", false
		}

		// A disk built on top of other disks is a valid explicit pick, but never the automatic
		// one: booting from an md array needs Talos 1.14, and the resolution knows no Talos
		// version (it is computed before any cluster exists). Once the minimum supported Talos
		// version reaches 1.14, these can become plain candidates and this tier can go away.
		if omni.IsCompositeBlockDevice(disk) {
			return "built on top of other disks", true
		}

		return "", true
	}

	evaluations := xslices.Map(disks, func(disk *specs.MachineStatusSpec_HardwareStatus_BlockDevice) evaluation {
		skipReason, selectable := classify(disk)

		return evaluation{disk: disk, skipReason: skipReason, selectable: selectable}
	})

	slices.SortFunc(evaluations, func(a, b evaluation) int {
		if byGroup := cmp.Compare(evaluationGroup(a), evaluationGroup(b)); byGroup != 0 {
			return byGroup
		}

		if a.skipReason == "" {
			if a.disk.Transport == transportUSB && b.disk.Transport != transportUSB {
				return 1
			} else if b.disk.Transport == transportUSB && a.disk.Transport != transportUSB {
				return -1
			}

			// Equal sizes tie-break by the dev path, so the automatic default does not flip
			// with the block device order.
			if bySize := cmp.Compare(a.disk.Size, b.disk.Size); bySize != 0 {
				return bySize
			}
		}

		return cmp.Compare(devPath(a.disk), devPath(b.disk))
	})

	return xslices.Map(evaluations, func(e evaluation) *specs.MachineInstallDiskStatusSpec_Disk {
		return &specs.MachineInstallDiskStatusSpec_Disk{
			DevPath:    devPath(e.disk),
			Selectable: e.selectable,
			SkipReason: e.skipReason,
			Members:    e.disk.SecondaryDisks,
		}
	})
}

// evaluationGroup orders the evaluated disks: candidates, then explicitly selectable disks,
// then the ineligible rest.
func evaluationGroup(e evaluation) int {
	switch {
	case e.skipReason == "":
		return 0
	case e.selectable:
		return 1
	default:
		return 2
	}
}

func findSystemDisk(disks []*specs.MachineStatusSpec_HardwareStatus_BlockDevice) *specs.MachineStatusSpec_HardwareStatus_BlockDevice {
	for _, disk := range disks {
		if disk.SystemDisk {
			return disk
		}
	}

	return nil
}

// devPath returns the dev path of the disk. It falls back to the linux name for the machine
// status data polled before the dev path field existed, as an offline machine never re-polls.
func devPath(disk *specs.MachineStatusSpec_HardwareStatus_BlockDevice) string {
	return cmp.Or(disk.DevPath, disk.LinuxName)
}
