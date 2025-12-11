// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/mattn/go-isatty"
	"github.com/siderolabs/go-kubeconfig"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

const (
	serviceAccountFlag     = "service-account"
	serviceAccountUserFlag = "user"
)

var kubeconfigCmdFlags struct {
	cluster              string
	forceContextName     string
	serviceAccountUser   string
	grantType            string
	serviceAccountGroups []string
	serviceAccountTTL    time.Duration
	force                bool
	merge                bool
	serviceAccount       bool
	breakGlass           bool
}

var allGrantTypes = strings.Join([]string{
	"auto",
	"authcode",
	"authcode-keyboard",
}, "|")

// kubeconfigCmd represents the get (resources) command.
var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig [local-path]",
	Short: "Download the admin kubeconfig of a cluster",
	Long: `Download the admin kubeconfig of a cluster.
If merge flag is defined, config will be merged with ~/.kube/config or [local-path] if specified.
Otherwise kubeconfig will be written to PWD or [local-path] if specified.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return access.WithClient(getKubeconfig(args))
	},
}

//nolint:gocognit,gocyclo,cyclop
func getKubeconfig(args []string) func(ctx context.Context, client *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		var localPath string

		if len(args) == 0 {
			// no path given, use defaults
			var err error

			if kubeconfigCmdFlags.merge {
				localPath, err = kubeconfig.SinglePath()
				if err != nil {
					return err
				}
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
			// only dir name was given, append `kubeconfig` by default
			localPath = filepath.Join(localPath, "kubeconfig")
		}

		_, err = os.Stat(localPath)
		if err == nil && !kubeconfigCmdFlags.force && !kubeconfigCmdFlags.merge {
			return fmt.Errorf("kubeconfig file already exists, use --force to overwrite: %q", localPath)
		} else if err != nil {
			if os.IsNotExist(err) {
				// merge doesn't make sense if target path doesn't exist
				kubeconfigCmdFlags.merge = false
			} else {
				return fmt.Errorf("error checking path %q: %w", localPath, err)
			}
		}

		if validateErr := validateClusterExists(ctx, client, kubeconfigCmdFlags.cluster); validateErr != nil {
			return validateErr
		}

		var opts []management.KubeconfigOption

		if kubeconfigCmdFlags.serviceAccount {
			if kubeconfigCmdFlags.serviceAccountUser == "" {
				return fmt.Errorf("--%s flag is required when --%s is set to true", serviceAccountUserFlag, serviceAccountFlag)
			}

			opts = append(opts, management.WithServiceAccount(
				kubeconfigCmdFlags.serviceAccountTTL,
				kubeconfigCmdFlags.serviceAccountUser,
				kubeconfigCmdFlags.serviceAccountGroups...,
			))
		}

		opts = append(opts,
			management.WithGrantType(kubeconfigCmdFlags.grantType),
			management.WithBreakGlassKubeconfig(kubeconfigCmdFlags.breakGlass),
		)

		data, err := client.Management().WithCluster(kubeconfigCmdFlags.cluster).Kubeconfig(ctx, opts...)
		if err != nil {
			if code, ok := status.FromError(err); ok && code.Code() == codes.NotFound {
				return fmt.Errorf("cluster %s not found", kubeconfigCmdFlags.cluster)
			}

			return err
		}

		if kubeconfigCmdFlags.merge {
			return extractAndMerge(data, localPath)
		}

		return os.WriteFile(localPath, data, 0o640)
	}
}

// validateClusterExists checks if the specified cluster exists in Omni.
func validateClusterExists(ctx context.Context, client *client.Client, clusterName string) error {
	st := client.Omni().State()

	// Get the cluster by ID
	if _, err := safe.StateGetByID[*omni.Cluster](ctx, st, clusterName); err != nil {
		if state.IsNotFoundError(err) {
			return fmt.Errorf("cluster not found: %q", clusterName)
		}

		return fmt.Errorf("failed to check if cluster '%s' exists: %w", clusterName, err)
	}

	return nil
}

func extractAndMerge(data []byte, localPath string) error {
	config, err := clientcmd.Load(data)
	if err != nil {
		return err
	}

	merger, err := kubeconfig.Load(localPath)
	if err != nil {
		return err
	}

	interactive := isatty.IsTerminal(os.Stdout.Fd())

	err = merger.Merge(config, kubeconfig.MergeOptions{
		ActivateContext:  true,
		ForceContextName: kubeconfigCmdFlags.forceContextName,
		OutputWriter:     os.Stdout,
		ConflictHandler: func(component kubeconfig.ConfigComponent, name string) (kubeconfig.ConflictDecision, error) {
			if kubeconfigCmdFlags.force {
				return kubeconfig.OverwriteDecision, nil
			}

			if !interactive {
				return kubeconfig.RenameDecision, nil
			}

			return askOverwriteOrRename(fmt.Sprintf("%s %q already exists", component, name))
		},
	})
	if err != nil {
		return err
	}

	return merger.Write(localPath)
}

func askOverwriteOrRename(prompt string) (kubeconfig.ConflictDecision, error) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s [(r)ename/(o)verwrite]: ", prompt)

		response, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		switch strings.ToLower(strings.TrimSpace(response)) {
		case "overwrite", "o":
			return kubeconfig.OverwriteDecision, nil
		case "rename", "r":
			return kubeconfig.RenameDecision, nil
		}
	}
}

func init() {
	kubeconfigCmd.Flags().StringVarP(&kubeconfigCmdFlags.cluster, "cluster", "c", "", "cluster to use")
	kubeconfigCmd.Flags().BoolVarP(&kubeconfigCmdFlags.force, "force", "f", false, "force overwrite of kubeconfig if already present, force overwrite on kubeconfig merge")
	kubeconfigCmd.Flags().StringVar(&kubeconfigCmdFlags.forceContextName, "force-context-name", "", "force context name for kubeconfig merge")
	kubeconfigCmd.Flags().BoolVarP(&kubeconfigCmdFlags.merge, "merge", "m", true, "merge with existing kubeconfig")

	kubeconfigCmd.Flags().BoolVar(&kubeconfigCmdFlags.serviceAccount, serviceAccountFlag, false,
		"create a service account type kubeconfig instead of a OIDC-authenticated user type")
	kubeconfigCmd.Flags().DurationVar(&kubeconfigCmdFlags.serviceAccountTTL, "ttl", 365*24*time.Hour,
		fmt.Sprintf("ttl for the service account token. only used when --%s is set to true", serviceAccountFlag))
	kubeconfigCmd.Flags().StringVar(&kubeconfigCmdFlags.serviceAccountUser, serviceAccountUserFlag, "",
		fmt.Sprintf("user to be used in the service account token (sub). required when --%s is set to true", serviceAccountFlag))
	kubeconfigCmd.Flags().StringSliceVar(&kubeconfigCmdFlags.serviceAccountGroups, "groups", []string{constants.DefaultAccessGroup},
		fmt.Sprintf("group to be used in the service account token (groups). only used when --%s is set to true", serviceAccountFlag))
	kubeconfigCmd.Flags().StringVar(&kubeconfigCmdFlags.grantType, "grant-type", "", fmt.Sprintf("Authorization grant type to use. One of (%s)", allGrantTypes))
	kubeconfigCmd.Flags().BoolVar(&kubeconfigCmdFlags.breakGlass, "break-glass", false, "get kubeconfig that allows accessing nodes bypasing Omni (if enabled for the account)")

	kubeconfigCmd.MarkFlagRequired("cluster") //nolint:errcheck

	RootCmd.AddCommand(kubeconfigCmd)
}
