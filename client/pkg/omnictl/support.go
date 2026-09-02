// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/gosuri/uiprogress"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/internal/safeout"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/supportbundle"
)

var supportCmdFlags struct {
	cluster                       string
	output                        string
	encryptionRecipients          []string
	verbose                       bool
	noEncryption                  bool
	encryptionNoDefaultRecipients bool
}

// supportCmd represents the get (resources) command.
var supportCmd = &cobra.Command{
	Use:   "support [local-path]",
	Short: "Download the support bundle for a cluster",
	Long: `The command collects all non-sensitive information for the cluster from the Omni state.

By default, the generated bundle is encrypted using age encryption to the list of recipients
set by the members of the 'siderolabs' GitHub organization. The encrypted bundle by default will
only be decryptable by the Sidero Labs team, but you can also specify additional recipients using the
--encryption-recipients flag, or disable encryption completely using the --no-encryption flag.
Default encryption recipients can be removed by setting --encryption-no-default-recipients flag.`,
	Args: cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		if supportCmdFlags.noEncryption && (len(supportCmdFlags.encryptionRecipients) > 0 || supportCmdFlags.encryptionNoDefaultRecipients) {
			return errors.New("--encryption-recipients and --encryption-no-default-recipients cannot be used with --no-encryption")
		}

		if supportCmdFlags.output == "" {
			supportCmdFlags.output = "support.zip"

			if !supportCmdFlags.noEncryption {
				supportCmdFlags.output += ".age"
			}
		}

		// parse the recipient keys here rather than inside the client callback, so a bad one fails
		// before we authenticate and pull down the whole bundle.
		if !supportCmdFlags.noEncryption {
			if _, _, err := supportbundle.Encrypt(io.Discard, encryptionOptions()); err != nil {
				return err
			}
		}

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

	w := tabwriter.NewWriter(os.Stderr, 0, 0, 3, ' ', 0) //nolint:forbidigo // the rows are colored, so the fields are escaped one by one above

	for _, err := range sbe.errors {
		if !wroteHeader {
			wroteHeader = true

			fmt.Fprintln(safeout.Stderr(), "Processed with errors:")
			fmt.Fprintln(w, "\tSOURCE\tERROR") //nolint:errcheck
		}

		details := strings.Split(err.value, "\n")
		for i, d := range details {
			details[i] = strings.TrimSpace(d)
		}

		fmt.Fprintf(w, "\t%s\t%s\n", safeout.Cell(err.source), color.RedString(safeout.Cell(details[0]))) //nolint:errcheck

		if len(details) > 1 {
			for _, line := range details[1:] {
				fmt.Fprintf(w, "\t\t%s\n", color.RedString(safeout.Cell(line))) //nolint:errcheck
			}
		}
	}

	return w.Flush()
}

func createSupportBundle() func(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
	return func(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
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

		// the bundle is encrypted here rather than on the server, so this works against any Omni version.
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

		recipients, err := writeBundle(f, data, !supportCmdFlags.noEncryption, encryptionOptions())
		if err != nil {
			return err
		}

		printRecipients(recipients)

		return nil
	}
}

func encryptionOptions() supportbundle.EncryptionOptions {
	return supportbundle.EncryptionOptions{
		Recipients:          supportCmdFlags.encryptionRecipients,
		NoDefaultRecipients: supportCmdFlags.encryptionNoDefaultRecipients,
	}
}

// writeBundle writes the downloaded bundle to dst, wrapping it in an age layer when encrypting, and
// reports the recipients able to open it.
func writeBundle(dst io.Writer, data []byte, encrypt bool, o supportbundle.EncryptionOptions) ([]string, error) {
	if !encrypt {
		_, err := dst.Write(data)

		return nil, err
	}

	encWriter, recipients, err := supportbundle.Encrypt(dst, o)
	if err != nil {
		return nil, err
	}

	if _, err = encWriter.Write(data); err != nil {
		return nil, err
	}

	// flush the age layer before reporting success.
	if err = encWriter.Close(); err != nil {
		return nil, err
	}

	return recipients, nil
}

func printRecipients(recipients []string) {
	if len(recipients) == 0 {
		return
	}

	fmt.Fprintln(safeout.Stderr(), "Support bundle encrypted to the following recipients:")

	for _, r := range recipients {
		fmt.Fprintf(safeout.Stderr(), "  - %s\n", r)
	}
}

func openArchive() (*os.File, error) {
	if _, err := os.Stat(supportCmdFlags.output); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else {
		buf := bufio.NewReader(os.Stdin)

		safeout.Printf("%s already exists, overwrite? [y/N]: ", supportCmdFlags.output)

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

			// the progress bars redraw themselves, so the fields received from the API are escaped here
			src := safeout.Cell(p.Source)

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

		np.state = safeout.Cell(p.State)
		np.bar.Incr()
	}

	uiprogress.Stop()
}

func init() {
	supportCmd.Flags().StringVarP(&supportCmdFlags.cluster, "cluster", "c", "", "cluster to use")
	supportCmd.Flags().StringVarP(&supportCmdFlags.output, "output", "O", "", "support bundle output (default \"support.zip.age\", or \"support.zip\" with --no-encryption)")
	supportCmd.Flags().BoolVarP(&supportCmdFlags.verbose, "verbose", "v", false, "verbose output")
	supportCmd.Flags().BoolVar(
		&supportCmdFlags.noEncryption, "no-encryption", false,
		"do not encrypt the support bundle (output is written as-is)",
	)
	supportCmd.Flags().StringArrayVar(
		&supportCmdFlags.encryptionRecipients, "encryption-recipients", nil,
		"additional age recipients (SSH or age public keys) to encrypt the support bundle to (can be specified multiple times)",
	)
	supportCmd.Flags().BoolVar(
		&supportCmdFlags.encryptionNoDefaultRecipients, "encryption-no-default-recipients", false,
		"do not encrypt to the default recipients, only to the ones provided via --encryption-recipients",
	)

	supportCmd.MarkFlagRequired("cluster") //nolint:errcheck

	RootCmd.AddCommand(supportCmd)
}
