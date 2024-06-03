// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/gosuri/uiprogress"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var supportCmdFlags struct {
	cluster string
	output  string
	verbose bool
}

// supportCmd represents the get (resources) command.
var supportCmd = &cobra.Command{
	Use:   "support [local-path]",
	Short: "Download the support bundle for a cluster",
	Long:  `The command collects all non-sensitive information for the cluster from the Omni state.`,
	Args:  cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		return access.WithClient(createSupportBundle())
	},
}

type supportBundleError struct {
	source string
	value  string
}

type supportBundleErrors struct {
	errors []supportBundleError
}

func (sbe *supportBundleErrors) handleProgress(p *management.GetSupportBundleResponse_Progress) {
	if p.Error != "" {
		sbe.errors = append(sbe.errors, supportBundleError{
			source: p.Source,
			value:  p.Error,
		})
	}
}

func (sbe *supportBundleErrors) print() error {
	if sbe.errors == nil {
		return nil
	}

	var wroteHeader bool

	w := tabwriter.NewWriter(os.Stderr, 0, 0, 3, ' ', 0)

	for _, err := range sbe.errors {
		if !wroteHeader {
			wroteHeader = true

			fmt.Fprintln(os.Stderr, "Processed with errors:")
			fmt.Fprintln(w, "\tSOURCE\tERROR") //nolint:errcheck
		}

		details := strings.Split(err.value, "\n")
		for i, d := range details {
			details[i] = strings.TrimSpace(d)
		}

		fmt.Fprintf(w, "\t%s\t%s\n", err.source, color.RedString(details[0])) //nolint:errcheck

		if len(details) > 1 {
			for _, line := range details[1:] {
				fmt.Fprintf(w, "\t\t%s\n", color.RedString(line)) //nolint:errcheck
			}
		}
	}

	return w.Flush()
}

func createSupportBundle() func(ctx context.Context, client *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		progress := make(chan *management.GetSupportBundleResponse_Progress)

		eg, ctx := errgroup.WithContext(ctx)

		var errors supportBundleErrors

		eg.Go(func() error {
			if supportCmdFlags.verbose {
				showProgress(progress, &errors)
			} else {
				for p := range progress {
					if p == nil {
						return nil
					}

					errors.handleProgress(p)
				}
			}

			return nil
		})

		data, err := client.Management().GetSupportBundle(ctx, supportCmdFlags.cluster, progress)
		if err != nil {
			return err
		}

		if err = eg.Wait(); err != nil {
			return err
		}

		if err = errors.print(); err != nil {
			return err
		}

		f, err := openArchive()
		if err != nil {
			return err
		}

		defer f.Close() //nolint:errcheck

		_, err = f.Write(data)

		return err
	}
}

func openArchive() (*os.File, error) {
	if _, err := os.Stat(supportCmdFlags.output); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else {
		buf := bufio.NewReader(os.Stdin)

		fmt.Printf("%s already exists, overwrite? [y/N]: ", supportCmdFlags.output)

		choice, err := buf.ReadString('\n')
		if err != nil {
			return nil, err
		}

		if strings.TrimSpace(strings.ToLower(choice)) != "y" {
			return nil, fmt.Errorf("operation was aborted")
		}
	}

	return os.OpenFile(supportCmdFlags.output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
}

func showProgress(progress <-chan *management.GetSupportBundleResponse_Progress, errors *supportBundleErrors) {
	uiprogress.Start()

	type nodeProgress struct {
		bar   *uiprogress.Bar
		state string
	}

	nodes := map[string]*nodeProgress{}

	for p := range progress {
		if p == nil {
			return
		}

		errors.handleProgress(p)

		if p.Total == 0 {
			continue
		}

		var (
			np *nodeProgress
			ok bool
		)

		if np, ok = nodes[p.Source]; !ok {
			bar := uiprogress.AddBar(int(p.Total))
			bar = bar.AppendCompleted().PrependElapsed()

			src := p.Source

			np = &nodeProgress{
				state: "initializing...",
				bar:   bar,
			}

			bar.AppendFunc(func(*uiprogress.Bar) string {
				return fmt.Sprintf("%s: %s", src, np.state)
			})

			bar.Width = 20

			nodes[src] = np
		} else {
			np = nodes[p.Source]
		}

		np.state = p.State
		np.bar.Incr()
	}

	uiprogress.Stop()
}

func init() {
	supportCmd.Flags().StringVarP(&supportCmdFlags.cluster, "cluster", "c", "", "cluster to use")
	supportCmd.Flags().StringVarP(&supportCmdFlags.output, "output", "O", "support.zip", "support bundle output")
	supportCmd.Flags().BoolVarP(&supportCmdFlags.verbose, "verbose", "v", false, "verbose output")

	supportCmd.MarkFlagRequired("cluster") //nolint:errcheck

	RootCmd.AddCommand(supportCmd)
}
