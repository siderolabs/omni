// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package template

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/template/operations"
)

var syncCmdFlags struct {
	options operations.SyncOptions
}

// syncCmd represents the template sync command.
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Apply template to the Omni.",
	Long: `Query existing resources for the cluster and compare them with the resources generated from the template, create/update/delete resources as needed. This command requires API access.
	
	If a file is specified, only that file will be processed.
	If a directory is specified, all YAML files (*.yaml, *.yml) in the directory
	and its subdirectories will be processed recursively. Each template file is
	processed independently, allowing management of multiple clusters.`,
	Example: "",
	Args:    cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		return access.WithClient(syncTemplateFiles)
	},
}

func discoverTemplateFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return []string{path}, nil
	}

	var files []string

	err = filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(filePath))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, filePath)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no YAML files found in directory %q", path)
	}

	return files, nil
}

func syncTemplateFiles(ctx context.Context, client *client.Client) error {
	files, err := discoverTemplateFiles(cmdFlags.TemplatePath)
	if err != nil {
		return fmt.Errorf("failed to discover template files from %q: %w", cmdFlags.TemplatePath, err)
	}

	if syncCmdFlags.options.Verbose {
		fmt.Printf("Processing %d template file(s)\n", len(files))
	}

	// Process each template file independently
	for _, file := range files {
		if syncCmdFlags.options.Verbose {
			fmt.Printf("Syncing template: %s\n", file)
		}

		f, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("failed to open template file %q: %w", file, err)
		}

		err = operations.SyncTemplate(ctx, f, os.Stdout, client.Omni().State(), syncCmdFlags.options)
		f.Close() //nolint:errcheck

		if err != nil {
			return fmt.Errorf("failed to sync template %q: %w", file, err)
		}
	}

	return nil
}

func init() {
	addRequiredFileFlag(syncCmd)
	syncCmd.PersistentFlags().BoolVarP(&syncCmdFlags.options.Verbose, "verbose", "v", false, "verbose output (show diff for each resource)")
	syncCmd.PersistentFlags().BoolVarP(&syncCmdFlags.options.DryRun, "dry-run", "d", false, "dry run")
	templateCmd.AddCommand(syncCmd)
}
