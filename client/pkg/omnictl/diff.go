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
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/fatih/color"
	"github.com/siderolabs/gen/ensure"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/diff"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

const (
	diffExitDifferences = 1
	diffExitError       = 2
)

var diffCmdFlags struct {
	resFile string
	verbose bool
}

// diffCmd represents the diff command.
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show differences between local resource YAML and the live server state",
	Long: `Compare resources defined in YAML file(s) against their current state on the server.

	If a file is specified, only that file will be processed.
	If a directory is specified, all YAML files (*.yaml, *.yml) in the directory
	and its subdirectories will be processed recursively. Each file is processed
	independently, similar to kubectl behavior.`,
	Args: cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		var differ bool

		err := access.WithClient(func(ctx context.Context, c *client.Client, _ access.ServerInfo) error {
			var e error

			differ, e = diffAll(ctx, c.Omni().State(), os.Stdout)

			return e
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err) //nolint:errcheck

			os.Exit(diffExitError)
		}

		if differ {
			os.Exit(diffExitDifferences)
		}

		return nil
	},
}

func diffAll(ctx context.Context, st state.State, w io.Writer) (bool, error) {
	files, err := discoverFiles(diffCmdFlags.resFile)
	if err != nil {
		return false, fmt.Errorf("failed to discover yaml files from %q: %w", diffCmdFlags.resFile, err)
	}

	if diffCmdFlags.verbose {
		fmt.Fprintf(w, "Processing %d resource file(s)\n", len(files))
	}

	var differ bool

	for _, file := range files {
		if diffCmdFlags.verbose {
			fmt.Fprintf(w, "Diffing resources from: %s\n", file)
		}

		raw, err := os.ReadFile(file)
		if err != nil {
			return false, fmt.Errorf("failed to read resource yaml file %q: %w", file, err)
		}

		resources, err := decodeResources(raw)
		if err != nil {
			return false, fmt.Errorf("failed to decode resources from file %q: %w", file, err)
		}

		fileDiffer, err := diffResources(ctx, st, resources, w, diffCmdFlags.verbose)
		if err != nil {
			return false, fmt.Errorf("failed to diff resources from file %q: %w", file, err)
		}

		differ = differ || fileDiffer
	}

	return differ, nil
}

func decodeResources(raw []byte) ([]resource.Resource, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))

	var resources []resource.Resource

	for {
		var res protobuf.YAMLResource

		err := dec.Decode(&res)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, err
		}

		resources = append(resources, res.Resource())
	}

	return resources, nil
}

func diffResources(ctx context.Context, st state.State, resources []resource.Resource, w io.Writer, verbose bool) (bool, error) {
	var differ bool

	for _, res := range resources {
		got, err := st.Get(ctx, res.Metadata())
		code := status.Code(err)

		switch {
		case err == nil:
			diffStr, derr := computeResourceDiff(got, res)
			if derr != nil {
				return false, derr
			}

			if diffStr != "" {
				renderResourceDiff(w, resource.String(got), resource.String(res), diffStr)

				differ = true
			} else if verbose {
				fmt.Fprintf(w, "%s: no changes\n", resource.String(res))
			}
		case state.IsNotFoundError(err):
			diffStr, derr := computeResourceDiff(nil, res)
			if derr != nil {
				return false, derr
			}

			if diffStr != "" {
				renderResourceDiff(w, "/dev/null", resource.String(res), diffStr)

				differ = true
			}
		case code == codes.PermissionDenied || code == codes.Unauthenticated:
			return false, fmt.Errorf("cannot read resource '%s' '%s' to diff it: %w", res.Metadata().ID(), res.Metadata().Type(), err)
		default:
			return false, fmt.Errorf("failed to get resource '%s' '%s': %w", res.Metadata().ID(), res.Metadata().Type(), err)
		}
	}

	return differ, nil
}

func computeResourceDiff(live, desired resource.Resource) (string, error) {
	var (
		liveYAML, desiredYAML []byte
		err                   error
	)

	if live != nil {
		if liveYAML, err = normalize(live); err != nil {
			return "", err
		}
	}

	if desired != nil {
		if desiredYAML, err = normalize(desired); err != nil {
			return "", err
		}
	}

	return diff.Compute(liveYAML, desiredYAML)
}

func normalize(res resource.Resource) ([]byte, error) {
	out, err := marshalResource(res)
	if err != nil {
		return nil, err
	}

	return stripServerMetadata([]byte(out))
}

var serverManagedMetadataKeys = map[string]struct{}{
	"version":    {},
	"created":    {},
	"updated":    {},
	"phase":      {},
	"finalizers": {},
}

func stripServerMetadata(in []byte) ([]byte, error) {
	var doc yaml.Node

	if err := yaml.Unmarshal(in, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse resource yaml: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return in, nil
	}

	root := doc.Content[0]

	if root.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value != "metadata" {
				continue
			}

			meta := root.Content[i+1]
			if meta.Kind == yaml.MappingNode {
				meta.Content = stripKeys(meta.Content)
			}
		}
	}

	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(root); err != nil {
		return nil, fmt.Errorf("failed to re-encode resource yaml: %w", err)
	}

	if err := enc.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func stripKeys(content []*yaml.Node) []*yaml.Node {
	var filtered []*yaml.Node

	for i := 0; i+1 < len(content); i += 2 {
		if _, drop := serverManagedMetadataKeys[content[i].Value]; drop {
			continue
		}

		filtered = append(filtered, content[i], content[i+1])
	}

	return filtered
}

func renderResourceDiff(w io.Writer, fromPath, toPath, diffStr string) {
	if diffStr == "" {
		return
	}

	bold := color.New(color.Bold)
	bold.Fprintf(w, "--- %s\n", fromPath) //nolint:errcheck
	bold.Fprintf(w, "+++ %s\n", toPath)   //nolint:errcheck

	cyan := color.New(color.FgCyan)
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)

	for line := range strings.SplitSeq(diffStr, "\n") {
		switch {
		case strings.HasPrefix(line, "@@"):
			cyan.Fprintln(w, line) //nolint:errcheck
		case strings.HasPrefix(line, "-"):
			red.Fprintln(w, line) //nolint:errcheck
		case strings.HasPrefix(line, "+"):
			green.Fprintln(w, line) //nolint:errcheck
		case line == "":
			// skip trailing empty line
		default:
			fmt.Fprintln(w, line) //nolint:errcheck
		}
	}
}

func init() {
	diffCmd.PersistentFlags().StringVarP(&diffCmdFlags.resFile, "file", "f", "", "Resource file or directory to diff against the server")
	diffCmd.PersistentFlags().BoolVarP(&diffCmdFlags.verbose, "verbose", "v", false, "Verbose output")
	ensure.NoError(diffCmd.MarkPersistentFlagRequired("file"))

	RootCmd.AddCommand(diffCmd)
}
