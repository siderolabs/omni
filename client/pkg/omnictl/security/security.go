// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package security contains commands for fetching a schematic's security artifacts: vulnerability
// scans, SBOMs and VEX documents.
package security

import "github.com/spf13/cobra"

// securityCmd represents the security sub-command.
var securityCmd = &cobra.Command{
	Use:     "security",
	Aliases: []string{"sec"},
	Short:   "Security artifact subcommands.",
	Long: `Commands to fetch vulnerability scans, SBOMs and VEX documents for a schematic, either
named explicitly or resolved from a cluster's installed schematics.`,
	Example: "",
}

// RootCmd exposes the root security command.
func RootCmd() *cobra.Command {
	return securityCmd
}

// minServerMajor and minServerMinor are the Omni server version the security commands require:
// the version ImageFactoryService.ClusterArtifactTargets first shipped in.
const (
	minServerMajor = 1
	minServerMinor = 12
)
