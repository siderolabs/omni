// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var talosconfigCmdFlags struct {
	cluster          string
	forceContextName string
	force            bool
	merge            bool
	getAdminConfig   bool
}

// talosconfigCmd represents the get (resources) command.
var talosconfigCmd = &cobra.Command{
	Use:   "talosconfig [local-path]",
	Short: "Download the admin talosconfig of a cluster",
	Long: `Download the admin talosconfig of a cluster.
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
		var localPath string

		if len(args) == 0 {
			// no path given, use defaults
			var err error

			if talosconfigCmdFlags.merge {
				localPath, err = config.GetTalosDirectory()
				if err != nil {
					return err
				}

				// TODO: figure out a proper way to get the path to .talos/config
				localPath = filepath.Join(localPath, "config")
			} else {
				localPath, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("error getting current working directory: %w", err)
				}
			}
		} else {
			localPath = args[0]
		}

		localPath = filepath.Clean(localPath)

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
			management.WithAdminTalosconfig(talosconfigCmdFlags.getAdminConfig),
		}

		var data []byte

		if talosconfigCmdFlags.cluster == "" {
			data, err = client.Management().Talosconfig(ctx, opts...)
		} else {
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

func init() {
	talosconfigCmd.Flags().StringVarP(&talosconfigCmdFlags.cluster, "cluster", "c", "", "cluster to use")
	talosconfigCmd.Flags().BoolVarP(&talosconfigCmdFlags.force, "force", "f", false, "force overwrite of talosconfig if already present")
	talosconfigCmd.Flags().BoolVarP(&talosconfigCmdFlags.merge, "merge", "m", true, "merge with existing talosconfig")

	if constants.IsDebugBuild {
		talosconfigCmd.Flags().BoolVar(&talosconfigCmdFlags.getAdminConfig, "admin", false, "get admin talosconfig (DEBUG-ONLY)")
	}

	RootCmd.AddCommand(talosconfigCmd)
}
