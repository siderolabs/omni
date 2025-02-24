// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var talosconfigCmdFlags struct {
	cluster          string
	forceContextName string
	force            bool
	merge            bool
	breakGlass       bool
	getRawConfig     bool
}

// talosconfigCmd represents the get (resources) command.
var talosconfigCmd = &cobra.Command{
	Use:   "talosconfig [local-path]",
	Short: "Download an admin talosconfig.",
	Long: `Download the generic admin talosconfig of the Omni instance or the admin talosconfig of a cluster.
Generic talosconfig can be used with any machine, including those in maintenance mode.
If merge flag is defined, config will be merged with ~/.talos/config or [local-path] if specified.
Otherwise talosconfig will be written to PWD or [local-path] if specified.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(getTalosconfig(args))
	},
}

//nolint:gocognit
func getTalosconfig(args []string) func(ctx context.Context, client *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		localPath, err := getLocalTalosconfigPath(args)
		if err != nil {
			return err
		}

		st, err := os.Stat(localPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("error checking path %q: %w", localPath, err)
			}

			err = os.MkdirAll(filepath.Dir(localPath), 0o755)
			if err != nil {
				return err
			}
		} else if st.IsDir() {
			// only dir name was given, append `talosconfig` by default
			localPath = filepath.Join(localPath, "talosconfig")
		}

		_, err = os.Stat(localPath)
		if err == nil && !(talosconfigCmdFlags.force || talosconfigCmdFlags.merge) {
			return fmt.Errorf("talosconfig file already exists, use --force to overwrite: %q", localPath)
		} else if err != nil {
			if os.IsNotExist(err) {
				// merge doesn't make sense if target path doesn't exist
				talosconfigCmdFlags.merge = false
			} else {
				return fmt.Errorf("error checking path %q: %w", localPath, err)
			}
		}

		opts := []management.TalosconfigOption{
			management.WithBreakGlassTalosconfig(talosconfigCmdFlags.breakGlass),
			management.WithRawTalosconfig(talosconfigCmdFlags.getRawConfig),
		}

		var data []byte

		if talosconfigCmdFlags.cluster == "" {
			data, err = client.Management().Talosconfig(ctx, opts...)
		} else {
			_, err = safe.ReaderGetByID[*omni.Cluster](ctx, client.Omni().State(), talosconfigCmdFlags.cluster)
			if err != nil {
				if !state.IsNotFoundError(err) {
					return err
				}

				fmt.Fprintf(os.Stderr, "warning: cluster %q does not exist\n", talosconfigCmdFlags.cluster)
			}

			data, err = client.Management().WithCluster(talosconfigCmdFlags.cluster).Talosconfig(ctx, opts...)
		}

		if err != nil {
			return err
		}

		if talosconfigCmdFlags.merge {
			cfg, err := config.FromBytes(data)
			if err != nil {
				return err
			}

			existing, err := config.Open(localPath)
			if err != nil {
				return err
			}

			renames := existing.Merge(cfg)

			for _, rename := range renames {
				fmt.Printf("renamed talosconfig context %s\n", rename.String())
			}

			return existing.Save(localPath)
		}

		return os.WriteFile(localPath, data, 0o640)
	}
}

func getLocalTalosconfigPath(args []string) (string, error) {
	var localPath string

	if len(args) == 0 {
		// no path given, use defaults
		var err error

		if talosconfigCmdFlags.merge {
			localPath, err = config.GetTalosDirectory()
			if err != nil {
				return "", err
			}

			// TODO: figure out a proper way to get the path to .talos/config
			localPath = filepath.Join(localPath, "config")
		} else {
			localPath, err = os.Getwd()
			if err != nil {
				return "", fmt.Errorf("error getting current working directory: %w", err)
			}
		}
	} else {
		localPath = args[0]
	}

	localPath = filepath.Clean(localPath)

	return localPath, nil
}

func init() {
	talosconfigCmd.Flags().StringVarP(&talosconfigCmdFlags.cluster, "cluster", "c", "", "cluster to use. If omitted, download the generic talosconfig for the Omni instance.")
	talosconfigCmd.Flags().BoolVarP(&talosconfigCmdFlags.force, "force", "f", false, "force overwrite of talosconfig if already present")
	talosconfigCmd.Flags().BoolVarP(&talosconfigCmdFlags.merge, "merge", "m", true, "merge with existing talosconfig")
	talosconfigCmd.Flags().BoolVar(&talosconfigCmdFlags.breakGlass, "break-glass", false, "get operator talosconfig that allows bypassing Omni (if enabled for the account)")

	if constants.IsDebugBuild {
		talosconfigCmd.Flags().BoolVar(&talosconfigCmdFlags.getRawConfig, "raw", false, "get raw talosconfig as it's stored in Omni (DEBUG-ONLY)")
	}

	RootCmd.AddCommand(talosconfigCmd)
}
