// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/siderolabs/gen/ensure"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var applyCmdFlags struct {
	resFile string
	options options
}

// applyCmd represents apply config command.
var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Create or update resource using YAML file as an input",
	Args:  cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		yamlRaw, err := os.ReadFile(applyCmdFlags.resFile)
		if err != nil {
			return fmt.Errorf("failed to read resource yaml file %q: %w", applyCmdFlags.resFile, err)
		}

		if applyCmdFlags.options.dryRun {
			applyCmdFlags.options.verbose = true
		}

		return access.WithClient(applyConfig(yamlRaw))
	},
}

func applyConfig(yamlRaw []byte) func(ctx context.Context, client *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		st := client.Omni().State()
		dec := yaml.NewDecoder(bytes.NewReader(yamlRaw))

		var resources []resource.Resource

		for {
			var res protobuf.YAMLResource

			err := dec.Decode(&res)
			if errors.Is(err, io.EOF) {
				break
			}

			if err != nil {
				return err
			}

			resources = append(resources, res.Resource())
		}

		for _, res := range resources {
			got, err := st.Get(ctx, res.Metadata())
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("failed to get resource '%s' '%s': %w", res.Metadata().ID(), res.Metadata().Type(), err)
			}

			if state.IsNotFoundError(err) {
				err = createResource(ctx, st, res, applyCmdFlags.options)
				if err != nil {
					return err
				}

				continue
			}

			err = updateResource(ctx, st, got, res, applyCmdFlags.options)
			if err != nil {
				return fmt.Errorf("failed to update resource '%s' '%s': %w", res.Metadata().ID(), res.Metadata().Type(), err)
			}
		}

		return nil
	}
}

type options struct {
	dryRun  bool
	verbose bool
}

func createResource(ctx context.Context, st state.State, res resource.Resource, opts options) error {
	if opts.verbose {
		out, err := marshalResource(res)
		if err != nil {
			return err
		}

		fmt.Printf("Creating resource '%s'\n\n%s\n\n", res.Metadata().ID(), out)
	}

	if opts.dryRun {
		return nil
	}

	if err := st.Create(ctx, res); err != nil {
		return fmt.Errorf("failed to create resource '%s' '%s': %w", res.Metadata().ID(), res.Metadata().Type(), err)
	}

	return nil
}

func updateResource(ctx context.Context, st state.State, got resource.Resource, res resource.Resource, opts options) error {
	if opts.verbose {
		outGot, err := marshalResource(got)
		if err != nil {
			return err
		}

		outRes, err := marshalResource(res)
		if err != nil {
			return err
		}

		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(outGot, outRes, false)

		fmt.Printf("Updating resource '%s'\n\n%s\n\n", res.Metadata().ID(), dmp.DiffPrettyText(diffs))
	}

	if opts.dryRun {
		return nil
	}

	res.Metadata().SetVersion(got.Metadata().Version())

	if err := st.Update(ctx, res); err != nil {
		return fmt.Errorf("failed to update resource '%s' '%s': %w", res.Metadata().ID(), res.Metadata().Type(), err)
	}

	return nil
}

func marshalResource(res resource.Resource) (string, error) {
	yamlRes, err := resource.MarshalYAML(res)
	if err != nil {
		return "", fmt.Errorf("failed to marshal resource '%s' '%s': %w", res.Metadata().ID(), res.Metadata().Type(), err)
	}

	out, err := yaml.Marshal(yamlRes)
	if err != nil {
		return "", fmt.Errorf("failed to marshal resource '%s' '%s': %w", res.Metadata().ID(), res.Metadata().Type(), err)
	}

	return string(out), nil
}

func init() {
	applyCmd.PersistentFlags().StringVarP(&applyCmdFlags.resFile, "file", "f", "", "Resource file to load and apply")
	applyCmd.PersistentFlags().BoolVarP(&applyCmdFlags.options.verbose, "verbose", "v", false, "Verbose output")
	applyCmd.PersistentFlags().BoolVarP(&applyCmdFlags.options.dryRun, "dry-run", "d", false, "Dry run, implies verbose")
	ensure.NoError(applyCmd.MarkPersistentFlagRequired("file"))

	RootCmd.AddCommand(applyCmd)
}
