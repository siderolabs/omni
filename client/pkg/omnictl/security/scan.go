// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package security

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	imagefactorypb "github.com/siderolabs/omni/client/api/omni/imagefactory"
	"github.com/siderolabs/omni/client/internal/safeout"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var scanCmdFlags struct {
	outputFlag
	format string
	targetFlags
}

// parseReportFormat parses the --format flag value into the factory's report format. The Long help
// explains why there is no "table" value here.
func parseReportFormat(value string) (imagefactorypb.VulnerabilityReportFormat, error) {
	switch value {
	case "json":
		return imagefactorypb.VulnerabilityReportFormat_JSON, nil
	case "sarif":
		return imagefactorypb.VulnerabilityReportFormat_SARIF, nil
	case "cyclonedx":
		return imagefactorypb.VulnerabilityReportFormat_CYCLONEDX, nil
	default:
		return imagefactorypb.VulnerabilityReportFormat_UNKNOWN_FORMAT, fmt.Errorf("invalid --format %q: must be one of json, sarif, cyclonedx", value)
	}
}

// validateScanOutputFormat rejects -o table paired with a --format other than json, which would
// otherwise count severities against a schema that does not have them and print all zeros. The
// error it returns says why.
func validateScanOutputFormat(outputFormat string, reportFormat imagefactorypb.VulnerabilityReportFormat) error {
	if outputFormat == "table" && reportFormat != imagefactorypb.VulnerabilityReportFormat_JSON {
		return fmt.Errorf("-o table only works with --format json: severity counts are read from Grype's own JSON schema, which sarif/cyclonedx don't share")
	}

	return nil
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Fetch the vulnerability scan report of a schematic, or of every schematic in a cluster.",
	Long: `Fetches the vulnerability scan report of a schematic, or of every schematic installed
across a cluster's machines.

By default, prints a table of findings by severity. With -o json or -o yaml, prints a JSON array
of {schematicId, arch, version, report, upgradeCandidates}, one entry per target fetched: report
is the factory's report embedded verbatim, and upgradeCandidates lists {version, report} for each
upgrade-target version --upgrade-paths also fetched.

--format selects the report format fetched from the factory (json, sarif, cyclonedx - all of
which are themselves JSON, so any embeds cleanly under -o json/yaml). There is no "table" format
value here - that's Grype's own preformatted text table, a separate thing from -o table's severity
summary, and can't be embedded in -o json/yaml output. The default -o table only works with
--format json: its severity counts are read from Grype's own JSON schema, which sarif/cyclonedx
don't share - pass -o json/yaml to fetch a report in another format.

Examples:
    # Scan a specific schematic build
    omnictl security scan --schematic <schematic-id> --talos-version 1.9.0 --arch amd64

    # Scan every schematic a cluster's machines are running, including upgrade targets
    omnictl security scan --cluster-id my-cluster --upgrade-paths

    # Print the full reports as JSON instead of the severity table
    omnictl security scan --cluster-id my-cluster -o json

    # Print SARIF reports instead of Grype's native JSON
    omnictl security scan --cluster-id my-cluster -o json --format sarif
`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		if err := scanCmdFlags.targetFlags.validate(); err != nil {
			return err
		}

		if err := scanCmdFlags.outputFlag.validate("table", "json", "yaml"); err != nil {
			return err
		}

		format, err := parseReportFormat(scanCmdFlags.format)
		if err != nil {
			return err
		}

		if err = validateScanOutputFormat(scanCmdFlags.outputFlag.format, format); err != nil {
			return err
		}

		return access.WithClient(func(ctx context.Context, c *client.Client, info access.ServerInfo) error {
			if !info.ServerSupports(minServerMajor, minServerMinor) {
				return fmt.Errorf("security commands require Omni v%d.%d.0 or newer (server is %s)", minServerMajor, minServerMinor, info.Version)
			}

			targets, err := resolveTargets(ctx, c.ImageFactory(), scanCmdFlags.targetFlags)
			if err != nil {
				return err
			}

			if scanCmdFlags.outputFlag.format == "table" {
				return writeScanTable(ctx, c, targets, format, scanCmdFlags.resolvedFromCluster())
			}

			results, err := fetchScanResults(ctx, c, targets, format, scanCmdFlags.resolvedFromCluster())
			if err != nil {
				return err
			}

			return scanCmdFlags.write(results)
		})
	},
}

func fetchScanReport(ctx context.Context, c *client.Client, job targetVersionJob, format imagefactorypb.VulnerabilityReportFormat) (*imagefactorypb.VulnerabilityReportResponse, error) {
	resp, err := c.ImageFactory().VulnerabilityReport(ctx, &imagefactorypb.VulnerabilityReportRequest{
		SchematicId:  job.target.schematicID,
		TalosVersion: job.version,
		Arch:         job.target.arch,
		Format:       format,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch scan report for %s: %w", job, err)
	}

	return resp, nil
}

// writeScanTable prints one row per (target, version) fetched: severity counts, not the report
// itself - the default, human-facing view. Reports are fetched concurrently; rows are still
// printed in resolution order once every fetch has completed.
func writeScanTable(
	ctx context.Context, c *client.Client, targets []target, format imagefactorypb.VulnerabilityReportFormat, tolerateMissing bool,
) error {
	// Counted through a pointer, so a row the factory has no report for stays distinguishable from
	// one that was scanned and came back clean: zero counts are a real, common answer here, and
	// skipMissing would otherwise leave both looking like all-zero counts.
	fetch := func(ctx context.Context, job targetVersionJob) (*severityCounts, error) {
		resp, err := fetchScanReport(ctx, c, job, format)
		if err != nil {
			return nil, err
		}

		counts, err := countSeverities(resp.GetData())
		if err != nil {
			return nil, fmt.Errorf("failed to parse scan report for %s: %w", job, err)
		}

		return &counts, nil
	}

	if tolerateMissing {
		fetch = skipMissing(fetch)
	}

	jobs := targetVersionJobs(targets)

	counts, err := fetchConcurrently(ctx, jobs, fetch)
	if err != nil {
		return err
	}

	tw := tabwriter.NewWriter(safeout.Stdout(), 0, 0, 3, ' ', 0)

	fmt.Fprintln(tw, strings.Join( //nolint:errcheck
		[]string{"SCHEMATIC", "ARCH", "VERSION", "ROLE", "MACHINES", "CRITICAL", "HIGH", "MEDIUM", "LOW", "OTHER"}, "\t"))

	for i, job := range jobs {
		writeScanRow(tw, job.target, job.version, counts[i])
	}

	return tw.Flush()
}

// writeScanRow writes one (target, version) row. A nil counts is a row the factory has no report
// for: its severity columns read "-" rather than 0, which would claim the target was scanned and
// found clean.
func writeScanRow(tw *tabwriter.Writer, t target, version string, counts *severityCounts) {
	role := t.role
	if role == "" {
		role = "-"
	}

	machines := "-"
	if t.machineCount > 0 {
		machines = strconv.Itoa(int(t.machineCount))
	}

	severities := []string{"-", "-", "-", "-", "-"}

	if counts != nil {
		severities = []string{
			strconv.Itoa(counts.Critical),
			strconv.Itoa(counts.High),
			strconv.Itoa(counts.Medium),
			strconv.Itoa(counts.Low),
			strconv.Itoa(counts.Other),
		}
	}

	// Each severity count is its own %s argument, not joined into one string first: joining with
	// "\t" would hand safeout.Fprintf a single value containing tabs, and Cell (correctly, for a
	// value that might be untrusted API data) escapes those rather than treating them as column
	// separators.
	safeout.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", //nolint:errcheck
		shortSchematic(t.schematicID), archName(t.arch), version, role, machines,
		severities[0], severities[1], severities[2], severities[3], severities[4])
}

func shortSchematic(id string) string {
	if len(id) <= 12 {
		return id
	}

	return id[:12] + "…"
}

// scanVersionReport is one upgrade candidate's report, nested under scanResult. Report is the
// factory's report embedded verbatim - see rawJSON.
type scanVersionReport struct {
	Version string          `json:"version" yaml:"version"`
	Report  json.RawMessage `json:"report"  yaml:"report"`
}

// scanResult is one target's scan result: {schematicId, arch, version, report,
// upgradeCandidates}. Report is the factory's report embedded verbatim under its own field, never
// merged with or replacing our own root-level fields.
type scanResult struct {
	SchematicID       string              `json:"schematicId"                 yaml:"schematicId"`
	Arch              string              `json:"arch"                        yaml:"arch"`
	Version           string              `json:"version"                     yaml:"version"`
	Report            json.RawMessage     `json:"report"                      yaml:"report"`
	UpgradeCandidates []scanVersionReport `json:"upgradeCandidates,omitempty" yaml:"upgradeCandidates,omitempty"`
}

// fetchScanResults fetches every target's reports concurrently, then reassembles them into one
// scanResult per target (current version plus upgrade candidates), in resolution order. With
// tolerateMissing set, a report the factory has not produced is skipped rather than failing the
// fetch - see skipMissing.
func fetchScanResults(
	ctx context.Context, c *client.Client, targets []target, format imagefactorypb.VulnerabilityReportFormat, tolerateMissing bool,
) ([]scanResult, error) {
	fetch := func(ctx context.Context, job targetVersionJob) (json.RawMessage, error) {
		resp, err := fetchScanReport(ctx, c, job, format)
		if err != nil {
			return nil, err
		}

		report, err := rawJSON(resp.GetData())
		if err != nil {
			return nil, fmt.Errorf("failed to parse scan report for %s: %w", job, err)
		}

		return report, nil
	}

	if tolerateMissing {
		fetch = skipMissing(fetch)
	}

	reports, err := fetchConcurrently(ctx, targetVersionJobs(targets), fetch)
	if err != nil {
		return nil, err
	}

	results := make([]scanResult, 0, len(targets))

	for _, t := range targets {
		// reports is the flattening targetVersionJobs produced, so each target's versions occupy
		// the next len(t.versions) entries - consumed as a window rather than tracked by an index,
		// so the two orders cannot silently drift apart.
		versionReports := reports[:len(t.versions)]
		reports = reports[len(t.versions):]

		// Without a report for the target's current version there is no result to render, whatever
		// its upgrade candidates turned up: Report is the one field every consumer of this shape
		// reads.
		if versionReports[0] == nil {
			continue
		}

		result := scanResult{
			SchematicID: t.schematicID,
			Arch:        archName(t.arch),
			Version:     t.versions[0],
			Report:      versionReports[0],
		}

		for i, version := range t.versions[1:] {
			if versionReports[i+1] == nil {
				continue
			}

			result.UpgradeCandidates = append(result.UpgradeCandidates, scanVersionReport{Version: version, Report: versionReports[i+1]})
		}

		results = append(results, result)
	}

	if len(results) == 0 {
		return nil, errNoArtifacts
	}

	return results, nil
}

func init() {
	registerTargetFlags(scanCmd, &scanCmdFlags.targetFlags)
	registerOutputFlag(scanCmd, &scanCmdFlags.outputFlag, "table", "Output format (table, json, yaml).")
	scanCmd.Flags().StringVar(&scanCmdFlags.format, "format", "json", "Report format to fetch (json, sarif, cyclonedx).")

	securityCmd.AddCommand(scanCmd)
}
