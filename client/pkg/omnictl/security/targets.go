// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package security

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	imagefactorypb "github.com/siderolabs/omni/client/api/omni/imagefactory"
	"github.com/siderolabs/omni/client/internal/safeout"
)

// fetchConcurrency caps how many artifact fetches run in parallel, so a fleet-wide pull across
// many (schematic, arch, version) combinations doesn't run one RPC at a time.
const fetchConcurrency = 8

// fetchConcurrently runs fetch for each item in items concurrently (bounded by fetchConcurrency),
// preserving item order in the returned results: results[i] corresponds to items[i]. The first
// error encountered is returned, canceling the rest - there is no point continuing to hammer a
// factory that is unreachable or rejecting Omni's credentials.
func fetchConcurrently[T, R any](ctx context.Context, items []T, fetch func(context.Context, T) (R, error)) ([]R, error) {
	results := make([]R, len(items))

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(fetchConcurrency)

	for i, item := range items {
		eg.Go(func() error {
			result, err := fetch(egCtx, item)
			if err != nil {
				return err
			}

			results[i] = result

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// skipMissing wraps fetch so an artifact the factory has not produced is skipped with a warning on
// stderr - yielding R's zero value - instead of failing the command. An artifact that does not
// exist yet is a first-class answer here, not a fault: a cluster-wide fetch routinely asks for
// combinations the factory has no artifact for, most of all with --upgrade-paths, and one of those
// must not discard every artifact that did come back.
//
// Only a fetch whose targets came from a cluster is wrapped. When the caller named one exact
// artifact, NotFound is the answer to the only thing they asked for, so it stays an error and keeps
// the message saying what was missing. That comes from the flags - see resolvedFromCluster - not
// from how many artifacts the cluster happened to resolve to, since a cluster running a single
// schematic is still a cluster-wide fetch.
func skipMissing[T, R any](fetch func(context.Context, T) (R, error)) func(context.Context, T) (R, error) {
	return func(ctx context.Context, item T) (R, error) {
		result, err := fetch(ctx, item)
		if err != nil && status.Code(err) == codes.NotFound {
			var zero R

			fmt.Fprintf(safeout.Stderr(), "warning: %s\n", err) //nolint:errcheck

			return zero, nil
		}

		return result, err
	}
}

// requireArtifacts drops the results skipMissing left as the zero value, and reports
// errNoArtifacts if that leaves nothing to render - the individual warnings are already on stderr
// by then.
func requireArtifacts[R any](results []R, missing func(R) bool) ([]R, error) {
	results = slices.DeleteFunc(results, missing)

	if len(results) == 0 {
		return nil, errNoArtifacts
	}

	return results, nil
}

// errNoArtifacts is returned when every artifact a command asked for turned out not to exist, so
// there is nothing to render - the individual warnings are already on stderr by then.
var errNoArtifacts = errors.New("no artifact was found for any of the requested targets")

// targetVersionJob is one (target, version) pair to fetch a per-schematic artifact for - shared
// by scan and sbom, which both fetch one artifact per target per version.
type targetVersionJob struct {
	version string
	target  target
}

// String is the schematic/version/arch triple every per-artifact message identifies a job by.
func (j targetVersionJob) String() string {
	return j.target.schematicID + "/" + j.version + "/" + archName(j.target.arch)
}

// targetVersionJobs flattens each target's versions into a job list, in (target, version) order.
func targetVersionJobs(targets []target) []targetVersionJob {
	var jobs []targetVersionJob

	for _, t := range targets {
		for _, version := range t.versions {
			jobs = append(jobs, targetVersionJob{target: t, version: version})
		}
	}

	return jobs
}

// artifactTargetsClient is the one method resolveTargets/resolveVersions need from
// *imagefactory.Client - narrowed so they can be exercised in tests against a fake, without a
// live gRPC connection.
type artifactTargetsClient interface {
	ClusterArtifactTargets(ctx context.Context, req *imagefactorypb.ClusterArtifactTargetsRequest) (*imagefactorypb.ClusterArtifactTargetsResponse, error)
}

// errUpgradePathsRequiresClusterID is returned when --upgrade-paths is set without --cluster-id.
// Upgrade-path computation only exists cluster-side - it picks the next patch/minor against the
// cluster's current version - so it has no meaning for an arbitrary schematic/version/arch.
var errUpgradePathsRequiresClusterID = errors.New("--upgrade-paths requires --cluster-id")

// target is one (schematic, arch) pair to fetch artifacts for, at one or more Talos versions.
type target struct {
	schematicID  string
	role         string
	versions     []string
	arch         imagefactorypb.Arch
	machineCount int32
}

// targetFlags is the flag set shared by commands that fetch a per-(schematic, arch) artifact:
// scan and sbom.
type targetFlags struct {
	clusterID    string
	schematicID  string
	talosVersion string
	arch         string
	upgradePaths bool
}

func registerTargetFlags(cmd *cobra.Command, flags *targetFlags) {
	cmd.Flags().StringVar(&flags.clusterID, "cluster-id", "", "Resolve targets from a cluster's installed schematics.")
	cmd.Flags().StringVar(&flags.schematicID, "schematic", "", "Schematic ID to fetch an artifact for.")
	cmd.Flags().StringVar(&flags.talosVersion, "talos-version", "", "Talos version to fetch an artifact for.")
	cmd.Flags().StringVar(&flags.arch, "arch", "", "Architecture to fetch an artifact for (amd64, arm64).")
	cmd.Flags().BoolVar(&flags.upgradePaths, "upgrade-paths", false,
		"Also fetch the cluster's upgrade-target versions, not just the current one (requires --cluster-id).")

	cmd.MarkFlagsMutuallyExclusive("cluster-id", "schematic")
	cmd.MarkFlagsMutuallyExclusive("cluster-id", "talos-version")
	cmd.MarkFlagsMutuallyExclusive("cluster-id", "arch")
	cmd.MarkFlagsRequiredTogether("schematic", "talos-version", "arch")
	cmd.MarkFlagsOneRequired("cluster-id", "schematic")
}

// validate checks the target flags for validity without making any network calls, so a bad flag
// value fails fast instead of only surfacing after a connection to the server is opened.
func (f targetFlags) validate() error {
	if f.upgradePaths && f.clusterID == "" {
		return errUpgradePathsRequiresClusterID
	}

	if f.clusterID == "" {
		if _, err := parseArch(f.arch); err != nil {
			return err
		}
	}

	return nil
}

// resolvedFromCluster reports whether the command resolves its own targets from a cluster, rather
// than being handed one exact (schematic, version, arch) to fetch. Those targets tolerate artifacts
// the factory has not produced; a named one does not - see skipMissing.
func (f targetFlags) resolvedFromCluster() bool {
	return f.clusterID != ""
}

// resolveTargets resolves the target(s) a command should fetch artifacts for: either the single
// explicit (schematic, arch) the caller named, or every target installed across a cluster's
// machines. Flag combinations are rejected by targetFlags.validate, which every command runs first.
func resolveTargets(ctx context.Context, c artifactTargetsClient, flags targetFlags) ([]target, error) {
	if flags.clusterID == "" {
		arch, err := parseArch(flags.arch)
		if err != nil {
			return nil, err
		}

		return []target{{
			schematicID: flags.schematicID,
			arch:        arch,
			versions:    []string{flags.talosVersion},
		}}, nil
	}

	resp, err := c.ClusterArtifactTargets(ctx, &imagefactorypb.ClusterArtifactTargetsRequest{ClusterId: flags.clusterID})
	if err != nil {
		return nil, err
	}

	versions := targetVersions(resp, flags.upgradePaths)

	targets := make([]target, 0, len(resp.GetTargets()))

	for _, t := range resp.GetTargets() {
		targets = append(targets, target{
			schematicID:  t.GetSchematicId(),
			arch:         t.GetArch(),
			role:         roleFor(t.GetIncludesControlPlane()),
			machineCount: t.GetMachineCount(),
			versions:     versions,
		})
	}

	return targets, nil
}

// versionFlags is the flag set shared by commands that fetch an artifact keyed by Talos version
// alone: vex.
type versionFlags struct {
	clusterID    string
	talosVersion string
	upgradePaths bool
}

func registerVersionFlags(cmd *cobra.Command, flags *versionFlags) {
	cmd.Flags().StringVar(&flags.clusterID, "cluster-id", "", "Resolve versions from a cluster's current and upgrade-target Talos versions.")
	cmd.Flags().StringVar(&flags.talosVersion, "talos-version", "", "Talos version to fetch an artifact for.")
	cmd.Flags().BoolVar(&flags.upgradePaths, "upgrade-paths", false,
		"Also fetch the cluster's upgrade-target versions, not just the current one (requires --cluster-id).")

	cmd.MarkFlagsMutuallyExclusive("cluster-id", "talos-version")
	cmd.MarkFlagsOneRequired("cluster-id", "talos-version")
}

// validate checks the version flags for validity without making any network calls.
func (f versionFlags) validate() error {
	if f.upgradePaths && f.clusterID == "" {
		return errUpgradePathsRequiresClusterID
	}

	return nil
}

// resolvedFromCluster reports whether the command resolves its own versions from a cluster, rather
// than being handed one exact version to fetch - see targetFlags.resolvedFromCluster.
func (f versionFlags) resolvedFromCluster() bool {
	return f.clusterID != ""
}

// resolveVersions resolves the Talos version(s) a command should fetch a version-keyed artifact
// for: either the single explicit version the caller named, or a cluster's current version plus,
// with --upgrade-paths, its upgrade targets. Flag combinations are rejected by
// versionFlags.validate, which every command runs first.
func resolveVersions(ctx context.Context, c artifactTargetsClient, flags versionFlags) ([]string, error) {
	if flags.clusterID == "" {
		return []string{flags.talosVersion}, nil
	}

	resp, err := c.ClusterArtifactTargets(ctx, &imagefactorypb.ClusterArtifactTargetsRequest{ClusterId: flags.clusterID})
	if err != nil {
		return nil, err
	}

	return targetVersions(resp, flags.upgradePaths), nil
}

// targetVersions is the version list a resolved cluster's targets should be fetched at: the
// current version, plus the upgrade-target versions when requested.
func targetVersions(resp *imagefactorypb.ClusterArtifactTargetsResponse, upgradePaths bool) []string {
	versions := []string{resp.GetCurrentTalosVersion()}

	if upgradePaths {
		versions = append(versions, resp.GetUpgradeTargetVersions()...)
	}

	return versions
}

// roleFor labels a target by whether any control plane machine runs it. A (schematic, arch) pair
// can be shared by control planes and workers, so the label says the target includes control
// planes rather than that every machine on it is one - "worker" is the only exhaustive case.
func roleFor(includesControlPlane bool) string {
	if includesControlPlane {
		return "includes control plane"
	}

	return "worker"
}

// parseArch parses the --arch flag value into the factory's Arch enum.
func parseArch(value string) (imagefactorypb.Arch, error) {
	switch value {
	case "amd64":
		return imagefactorypb.Arch_AMD64, nil
	case "arm64":
		return imagefactorypb.Arch_ARM64, nil
	default:
		return imagefactorypb.Arch_UNKNOWN_ARCH, fmt.Errorf("invalid --arch %q: must be amd64 or arm64", value)
	}
}

// archName is the factory's name for arch, as used in filenames and table output.
func archName(arch imagefactorypb.Arch) string {
	switch arch { //nolint:exhaustive // UNKNOWN_ARCH and any future value fall through to the default.
	case imagefactorypb.Arch_AMD64:
		return "amd64"
	case imagefactorypb.Arch_ARM64:
		return "arm64"
	default:
		return arch.String()
	}
}
