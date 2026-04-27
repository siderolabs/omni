// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package template contains commands related to cluster template operations.
package template

import (
	"os"
	"path/filepath"

	"github.com/siderolabs/gen/ensure"
	"github.com/spf13/cobra"
)

// cmdFlags contains shared cluster template flags.
var cmdFlags struct {
	// Path to the cluster template file.
	TemplatePath string
	// AllowedDir is the directory that restricts file access in the template.
	AllowedDir string
}

// resolvedRoot is an *os.Root opened at the root dir, resolved relative to the template file's directory.
// It is populated by PersistentPreRunE on templateCmd before any subcommand runs.
var resolvedRoot *os.Root

// templateCmd represents the template sub-command.
var templateCmd = &cobra.Command{
	Use:     "template",
	Aliases: []string{"t"},
	Short:   "Cluster template management subcommands.",
	Long:    `Commands to render, validate, manage cluster templates.`,
	Example: "",
	PersistentPreRunE: func(*cobra.Command, []string) error {
		templateDir := filepath.Dir(cmdFlags.TemplatePath)

		p := cmdFlags.AllowedDir
		if !filepath.IsAbs(p) {
			p = filepath.Join(templateDir, p)
		}

		var err error

		resolvedRoot, err = os.OpenRoot(p)
		if err != nil {
			return err
		}

		return nil
	},
}

// RootCmd exports templateCmd.
func RootCmd() *cobra.Command {
	templateCmd.PersistentFlags().StringVar(&cmdFlags.AllowedDir, "allowed-dir", "./", "allowed directory for file access in the template;"+
		" relative paths are resolved against the template file's directory.")

	return templateCmd
}

func addRequiredFileFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&cmdFlags.TemplatePath, "file", "f", "", "path to the cluster template file or directory.")

	ensure.NoError(cmd.MarkPersistentFlagRequired("file"))
}
