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
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/ensure"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/execdiff"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var applyCmdFlags struct {
	resFile string
	options options
}

// applyCmd represents apply config command.
var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Create or update resource using YAML file or directory as an input",
	Long: `Create or update resources using YAML file(s) as input.

If a file is specified, only that file will be processed.
If a directory is specified, all YAML files (*.yaml, *.yml) in the directory
and its subdirectories will be processed recursively. Each file is processed
independently, similar to kubectl behavior.

When --dry-run is used, ` + execdiff.EnvExternalDiff + ` environment variable can be
used to select an external diff command. Users can use external commands with
params too, example: ` + execdiff.EnvExternalDiff + `="colordiff -N -u"

By default, the built-in colorized unified diff is used.

Exit status: 0 No differences were found. 1 Differences were found. >1 An error occurred.`,
	Args: cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		if applyCmdFlags.options.dryRun {
			applyCmdFlags.options.verbose = true
			applyCmdFlags.options.differ = execdiff.New(os.Stdout)
		}

		err := access.WithClient(applyConfigFiles)

		if applyCmdFlags.options.differ != nil {
			if _, flushErr := applyCmdFlags.options.differ.Flush(); flushErr != nil {
				return errors.Join(err, flushErr)
			}
		}

		return err
	},
}

func discoverFiles(path string) ([]string, error) {
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

func applyConfigFiles(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
	files, err := discoverFiles(applyCmdFlags.resFile)
	if err != nil {
		return fmt.Errorf("failed to discover yaml files from %q: %w", applyCmdFlags.resFile, err)
	}

	if applyCmdFlags.options.verbose {
		fmt.Printf("Processing %d resource file(s)\n", len(files))
	}

	// Process each file independently
	for _, file := range files {
		if applyCmdFlags.options.verbose {
			fmt.Printf("Applying resources from: %s\n", file)
		}

		yamlRaw, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read resource yaml file %q: %w", file, err)
		}

		err = applyConfigFromBytes(ctx, client, yamlRaw)
		if err != nil {
			return fmt.Errorf("failed to apply resources from file %q: %w", file, err)
		}
	}

	return nil
}

func applyConfigFromBytes(ctx context.Context, client *client.Client, yamlRaw []byte) error {
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
				return fmt.Errorf("failed to create resource '%s' '%s': %w", res.Metadata().ID(), res.Metadata().Type(), err)
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

type options struct {
	differ  *execdiff.Differ
	dryRun  bool
	verbose bool
}

func resourceLabel(res resource.Resource) string {
	return fmt.Sprintf("%s(%s)", res.Metadata().Type(), res.Metadata().ID())
}

func resourceFilename(res resource.Resource) string {
	return fmt.Sprintf("%s-%s.yaml", res.Metadata().Type(), res.Metadata().ID())
}

func createResource(ctx context.Context, st state.State, res resource.Resource, opts options) error {
	if opts.verbose {
		newYAML, err := marshalResource(res)
		if err != nil {
			return err
		}

		if opts.differ != nil {
			if err := opts.differ.AddDiff(resourceLabel(res), resourceFilename(res), nil, newYAML); err != nil {
				return err
			}
		} else {
			fmt.Printf("Creating resource '%s'\n\n%s\n\n", res.Metadata().ID(), newYAML)
		}
	}

	if opts.dryRun {
		return nil
	}

	if err := st.Create(ctx, res); err != nil {
		return err
	}

	return nil
}

func updateResource(ctx context.Context, st state.State, got resource.Resource, res resource.Resource, opts options) error {
	if opts.verbose {
		oldYAML, err := marshalResource(got)
		if err != nil {
			return err
		}

		newYAML, err := marshalResource(res)
		if err != nil {
			return err
		}

		if opts.differ != nil {
			if err := opts.differ.AddDiff(resourceLabel(res), resourceFilename(res), oldYAML, newYAML); err != nil {
				return err
			}
		} else {
			fmt.Printf("Updating resource '%s'\n\nold:\n%s\nnew:\n%s\n\n", res.Metadata().ID(), oldYAML, newYAML)
		}
	}

	if opts.dryRun {
		return nil
	}

	res.Metadata().SetVersion(got.Metadata().Version())

	if err := st.Update(ctx, res); err != nil {
		return err
	}

	return nil
}

func marshalResource(res resource.Resource) ([]byte, error) {
	yamlRes, err := resource.MarshalYAML(res)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource '%s' '%s': %w", res.Metadata().ID(), res.Metadata().Type(), err)
	}

	out, err := yaml.Marshal(yamlRes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource '%s' '%s': %w", res.Metadata().ID(), res.Metadata().Type(), err)
	}

	return out, nil
}

func init() {
	applyCmd.PersistentFlags().StringVarP(&applyCmdFlags.resFile, "file", "f", "", "Resource file or directory to load and apply")
	applyCmd.PersistentFlags().BoolVarP(&applyCmdFlags.options.verbose, "verbose", "v", false, "Verbose output")
	applyCmd.PersistentFlags().BoolVarP(&applyCmdFlags.options.dryRun, "dry-run", "d", false, "Dry run, implies verbose")
	ensure.NoError(applyCmd.MarkPersistentFlagRequired("file"))

	RootCmd.AddCommand(applyCmd)
}
