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

var vexCmdFlags struct {
	outputFlag
	versionFlags
}

// vexCmd structurally mirrors sbomCmd (validate flags, resolve, fetch, write); the two fetch/output
// shapes differ enough (per-schematic vs per-version) that factoring out the shared shell isn't worth it.
//
//nolint:dupl
var vexCmd = &cobra.Command{
	Use:   "vex",
	Short: "Fetch the VEX document of a Talos version, or of every version a cluster runs.",
	Long: `Fetches the OpenVEX document of a Talos version, or of a cluster's current version and
(with --upgrade-paths) its upgrade targets. A VEX document covers a Talos version as a whole, not
a specific schematic, so no --schematic/--arch is needed. Always prints a JSON array of {version,
vex}, one entry per version fetched.

Examples:
    # Fetch the VEX document for a specific Talos version
    omnictl security vex --talos-version 1.9.0

    # Fetch the VEX documents for a cluster's current and upgrade-target versions
    omnictl security vex --cluster-id my-cluster --upgrade-paths
`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		if err := vexCmdFlags.versionFlags.validate(); err != nil {
			return err
		}

		if err := vexCmdFlags.outputFlag.validate("json", "yaml"); err != nil {
			return err
		}

		return access.WithClient(func(ctx context.Context, c *client.Client, info access.ServerInfo) error {
			if !info.ServerSupports(minServerMajor, minServerMinor) {
				return fmt.Errorf("security commands require Omni v%d.%d.0 or newer (server is %s)", minServerMajor, minServerMinor, info.Version)
			}

			versions, err := resolveVersions(ctx, c.ImageFactory(), vexCmdFlags.versionFlags)
			if err != nil {
				return err
			}

			results, err := fetchVEXResults(ctx, c, versions, vexCmdFlags.resolvedFromCluster())
			if err != nil {
				return err
			}

			return vexCmdFlags.write(results)
		})
	},
}

// vexResult is one Talos version's VEX document, embedded verbatim - see rawJSON.
type vexResult struct {
	Version string          `json:"version" yaml:"version"`
	VEX     json.RawMessage `json:"vex"     yaml:"vex"`
}

// fetchVEXResults fetches every version's document concurrently, in resolution order. With
// tolerateMissing set, a document the factory has not produced is skipped rather than failing the
// fetch - see skipMissing.
func fetchVEXResults(ctx context.Context, c *client.Client, versions []string, tolerateMissing bool) ([]vexResult, error) {
	fetch := func(ctx context.Context, version string) (vexResult, error) {
		resp, err := c.ImageFactory().VEXDocument(ctx, &imagefactorypb.VEXDocumentRequest{TalosVersion: version})
		if err != nil {
			return vexResult{}, fmt.Errorf("failed to fetch VEX document for %s: %w", version, err)
		}

		vex, err := rawJSON(resp.GetData())
		if err != nil {
			return vexResult{}, fmt.Errorf("failed to parse VEX document for %s: %w", version, err)
		}

		return vexResult{Version: version, VEX: vex}, nil
	}

	if tolerateMissing {
		fetch = skipMissing(fetch)
	}

	results, err := fetchConcurrently(ctx, versions, fetch)
	if err != nil {
		return nil, err
	}

	return requireArtifacts(results, func(result vexResult) bool { return result.VEX == nil })
}

func init() {
	registerVersionFlags(vexCmd, &vexCmdFlags.versionFlags)
	registerOutputFlag(vexCmd, &vexCmdFlags.outputFlag, "json", "Output format (json, yaml).")

	securityCmd.AddCommand(vexCmd)
}
