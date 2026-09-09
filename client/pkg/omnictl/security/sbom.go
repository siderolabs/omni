// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package security

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	imagefactorypb "github.com/siderolabs/omni/client/api/omni/imagefactory"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var sbomCmdFlags struct {
	outputFlag
	targetFlags
}

// sbomCmd structurally mirrors vexCmd (validate flags, resolve, fetch, write); the two fetch/output
// shapes differ enough (per-schematic vs per-version) that factoring out the shared shell isn't worth it.
//
//nolint:dupl
var sbomCmd = &cobra.Command{
	Use:   "sbom",
	Short: "Fetch the SPDX bundle of a schematic, or of every schematic in a cluster.",
	Long: `Fetches the SPDX bundle of a schematic, or of every schematic installed across a
cluster's machines, including upgrade targets with --upgrade-paths. Always prints a JSON array of
{schematicId, arch, version, sbom}, one entry per bundle fetched.

Examples:
    # Fetch the SBOM of a specific schematic build
    omnictl security sbom --schematic <schematic-id> --talos-version 1.9.0 --arch amd64

    # Fetch the SBOM of every schematic a cluster's machines are running, including upgrade targets
    omnictl security sbom --cluster-id my-cluster --upgrade-paths
`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		if err := sbomCmdFlags.targetFlags.validate(); err != nil {
			return err
		}

		if err := sbomCmdFlags.outputFlag.validate("json", "yaml"); err != nil {
			return err
		}

		return access.WithClient(func(ctx context.Context, c *client.Client, info access.ServerInfo) error {
			if !info.ServerSupports(minServerMajor, minServerMinor) {
				return fmt.Errorf("security commands require Omni v%d.%d.0 or newer (server is %s)", minServerMajor, minServerMinor, info.Version)
			}

			targets, err := resolveTargets(ctx, c.ImageFactory(), sbomCmdFlags.targetFlags)
			if err != nil {
				return err
			}

			results, err := fetchSBOMResults(ctx, c, targets, sbomCmdFlags.resolvedFromCluster())
			if err != nil {
				return err
			}

			return sbomCmdFlags.write(results)
		})
	},
}

// sbomResult is one (schematic, arch, version) bundle fetched: {schematicId, arch, version, sbom}.
// SBOM is the factory's bundle embedded verbatim under its own field - see rawJSON.
type sbomResult struct {
	SchematicID string          `json:"schematicId" yaml:"schematicId"`
	Arch        string          `json:"arch"        yaml:"arch"`
	Version     string          `json:"version"     yaml:"version"`
	SBOM        json.RawMessage `json:"sbom"        yaml:"sbom"`
}

// fetchSBOMResults fetches every target's bundles concurrently, in resolution order. With
// tolerateMissing set, a bundle the factory has not produced is skipped rather than failing the
// fetch - see skipMissing.
func fetchSBOMResults(ctx context.Context, c *client.Client, targets []target, tolerateMissing bool) ([]sbomResult, error) {
	fetch := func(ctx context.Context, job targetVersionJob) (sbomResult, error) {
		resp, err := c.ImageFactory().SBOM(ctx, &imagefactorypb.SBOMRequest{
			SchematicId:  job.target.schematicID,
			TalosVersion: job.version,
			Arch:         job.target.arch,
		})
		if err != nil {
			return sbomResult{}, fmt.Errorf("failed to fetch SBOM for %s: %w", job, err)
		}

		sbom, err := rawJSON(resp.GetData())
		if err != nil {
			return sbomResult{}, fmt.Errorf("failed to parse SBOM for %s: %w", job, err)
		}

		return sbomResult{
			SchematicID: job.target.schematicID,
			Arch:        archName(job.target.arch),
			Version:     job.version,
			SBOM:        sbom,
		}, nil
	}

	if tolerateMissing {
		fetch = skipMissing(fetch)
	}

	results, err := fetchConcurrently(ctx, targetVersionJobs(targets), fetch)
	if err != nil {
		return nil, err
	}

	return requireArtifacts(results, func(result sbomResult) bool { return result.SBOM == nil })
}

func init() {
	registerTargetFlags(sbomCmd, &sbomCmdFlags.targetFlags)
	registerOutputFlag(sbomCmd, &sbomCmdFlags.outputFlag, "json", "Output format (json, yaml).")

	securityCmd.AddCommand(sbomCmd)
}
