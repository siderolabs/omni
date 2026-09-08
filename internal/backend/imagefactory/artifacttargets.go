// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package imagefactory

import (
	"context"
	"sort"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	imagefactorypb "github.com/siderolabs/omni/client/api/omni/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// ArchNames is the name the image factory knows each architecture by.
var ArchNames = map[imagefactorypb.Arch]string{
	imagefactorypb.Arch_AMD64: "amd64",
	imagefactorypb.Arch_ARM64: "arm64",
}

// archFromName is the reverse of ArchNames: the architecture the factory's name refers to.
var archFromName = func() map[string]imagefactorypb.Arch {
	m := make(map[string]imagefactorypb.Arch, len(ArchNames))
	for arch, name := range ArchNames {
		m[name] = arch
	}

	return m
}()

// ClusterArtifactTargets resolves the (schematic, arch) pairs installed across a cluster's
// machines, along with the Talos versions to fetch security artifacts for: the cluster's current
// version, plus the upgrade targets to scan against.
func ClusterArtifactTargets(ctx context.Context, st state.State, clusterID string) (*imagefactorypb.ClusterArtifactTargetsResponse, error) {
	clusterStatus, err := safe.StateGetByID[*omni.ClusterStatus](ctx, st, clusterID)
	if err != nil {
		return nil, err
	}

	labelQuery := state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID))

	machineConfigs, err := safe.StateListAll[*omni.ClusterMachineConfigStatus](ctx, st, labelQuery)
	if err != nil {
		return nil, err
	}

	machineStatuses, err := safe.StateListAll[*omni.MachineStatus](ctx, st, labelQuery)
	if err != nil {
		return nil, err
	}

	talosVersions, err := safe.StateListAll[*omni.TalosVersion](ctx, st)
	if err != nil {
		return nil, err
	}

	currentVersion := clusterStatus.TypedSpec().Value.GetTalosVersion()

	return &imagefactorypb.ClusterArtifactTargetsResponse{
		Targets:               resolveArtifactTargets(machineConfigs, machineStatuses),
		CurrentTalosVersion:   currentVersion,
		UpgradeTargetVersions: computeUpgradeVersions(currentVersion, availableTalosVersions(talosVersions)),
	}, nil
}

// resolveArtifactTargets groups a cluster's machines into the unique (schematic, arch) pairs
// installed across them, counting the machines on each and noting whether any of them is a
// control plane node.
//
// A machine on an architecture no scan is published for, or with no schematic recorded yet, is
// left out rather than reported as a broken target - there is nothing to fetch for it.
func resolveArtifactTargets(
	machineConfigs safe.List[*omni.ClusterMachineConfigStatus],
	machineStatuses safe.List[*omni.MachineStatus],
) []*imagefactorypb.ArtifactTarget {
	archByMachine := make(map[string]string, machineStatuses.Len())

	for machineStatus := range machineStatuses.All() {
		if hardware := machineStatus.TypedSpec().Value.GetHardware(); hardware != nil {
			archByMachine[machineStatus.Metadata().ID()] = hardware.GetArch()
		}
	}

	type targetKey struct {
		schematicID string
		arch        imagefactorypb.Arch
	}

	targets := make(map[targetKey]*imagefactorypb.ArtifactTarget, machineConfigs.Len())

	order := make([]targetKey, 0, machineConfigs.Len())

	for machineConfig := range machineConfigs.All() {
		schematicID := machineConfig.TypedSpec().Value.GetSchematicId()

		arch, ok := archFromName[archByMachine[machineConfig.Metadata().ID()]]
		if schematicID == "" || !ok {
			continue
		}

		key := targetKey{schematicID: schematicID, arch: arch}

		target, exists := targets[key]
		if !exists {
			target = &imagefactorypb.ArtifactTarget{SchematicId: schematicID, Arch: arch}
			targets[key] = target
			order = append(order, key)
		}

		target.MachineCount++

		if _, isControlPlane := machineConfig.Metadata().Labels().Get(omni.LabelControlPlaneRole); isControlPlane {
			target.IncludesControlPlane = true
		}
	}

	result := make([]*imagefactorypb.ArtifactTarget, 0, len(order))
	for _, key := range order {
		result = append(result, targets[key])
	}

	return result
}

// availableTalosVersions is the list of Talos versions safe to offer as an upgrade target: those
// neither deprecated nor unsupported.
func availableTalosVersions(talosVersions safe.List[*omni.TalosVersion]) []string {
	versions := make([]string, 0, talosVersions.Len())

	for talosVersion := range talosVersions.All() {
		spec := talosVersion.TypedSpec().Value

		if !omni.TalosVersionAvailable(spec) {
			continue
		}

		versions = append(versions, spec.GetVersion())
	}

	return versions
}

// parsedVersion pairs a raw version string with its parsed form, so the original (possibly
// "v"-prefixed) spelling can be returned even though comparisons run on the parsed value.
type parsedVersion struct {
	raw string
	sv  semver.Version
}

// computeUpgradeVersions returns the Talos versions to scan against in addition to the cluster's
// current version: the latest patch of the current minor, and the latest patch of the next
// available minor, when strictly newer than the current version.
func computeUpgradeVersions(currentRaw string, availableRaw []string) []string {
	current, err := semver.ParseTolerant(currentRaw)
	if err != nil {
		return nil
	}

	available := make([]parsedVersion, 0, len(availableRaw))

	for _, raw := range availableRaw {
		sv, err := semver.ParseTolerant(raw)
		if err != nil {
			continue
		}

		available = append(available, parsedVersion{raw: raw, sv: sv})
	}

	var targets []string

	// Latest patch of the current minor version.
	currMinorPatches := make([]parsedVersion, 0, len(available))

	for _, p := range available {
		if p.sv.Major == current.Major && p.sv.Minor == current.Minor {
			currMinorPatches = append(currMinorPatches, p)
		}
	}

	if currMinorLatestPatch, ok := latest(currMinorPatches); ok && currMinorLatestPatch.sv.GT(current) {
		targets = append(targets, currMinorLatestPatch.raw)
	}

	// Latest patch of the *next* minor version - the smallest major.minor strictly greater than
	// the current one, so a gap in the available minors is handled correctly.
	nextMinorPatches := make([]parsedVersion, 0, len(available))

	for _, p := range available {
		if p.sv.Major > current.Major || (p.sv.Major == current.Major && p.sv.Minor > current.Minor) {
			nextMinorPatches = append(nextMinorPatches, p)
		}
	}

	if len(nextMinorPatches) > 0 {
		sort.Slice(nextMinorPatches, func(i, j int) bool {
			return nextMinorPatches[i].sv.LT(nextMinorPatches[j].sv)
		})

		nextMinorPatch := nextMinorPatches[0].sv

		sameMinor := make([]parsedVersion, 0, len(nextMinorPatches))
		for _, p := range nextMinorPatches {
			if p.sv.Major == nextMinorPatch.Major && p.sv.Minor == nextMinorPatch.Minor {
				sameMinor = append(sameMinor, p)
			}
		}

		if nextMinorLatestPatch, ok := latest(sameMinor); ok {
			targets = append(targets, nextMinorLatestPatch.raw)
		}
	}

	return targets
}

// latest returns the highest version in versions, or false when it is empty.
func latest(versions []parsedVersion) (parsedVersion, bool) {
	if len(versions) == 0 {
		return parsedVersion{}, false
	}

	sorted := append([]parsedVersion(nil), versions...)

	sort.Slice(sorted, func(i, j int) bool { return sorted[i].sv.GT(sorted[j].sv) })

	return sorted[0], true
}
