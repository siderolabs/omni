// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
	"k8s.io/kubectl/pkg/cmd/util/editor"
	"k8s.io/kubectl/pkg/cmd/util/editor/crlf"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

var editCmdFlags struct {
	namespace string
	selector  string
	idRegexp  string
	options   options
}

//nolint:gocyclo,gocognit,cyclop
func editFn(c *client.Client) func(context.Context, []resource.Resource, error) error {
	var (
		path      string
		lastError string
	)

	edit := editor.NewDefaultEditor([]string{
		"OMNI_EDITOR",
		"TALOS_EDITOR",
		"EDITOR",
	})

	return func(ctx context.Context, resources []resource.Resource, callError error) error {
		if callError != nil {
			return callError
		}

		hash := sha256.New()

		ids := xslices.Map(resources, func(res resource.Resource) string {
			id := res.Metadata().Type() + "/" + res.Metadata().ID()

			hash.Write([]byte(id))

			return id
		})

		var current bytes.Buffer

		encoder := yaml.NewEncoder(&current)
		originalResources := map[string]resource.Resource{}

		for _, res := range resources {
			yamlRes, err := resource.MarshalYAML(res)
			if err != nil {
				return err
			}

			if err := encoder.Encode(yamlRes); err != nil {
				return err
			}

			originalResources[resourceKey(res)] = res
		}

		// Build map of original resource canonical YAML by key for per-document comparison.
		originalYAML := make(map[string][]byte)

		for _, res := range resources {
			data, err := marshalResourceCanonical(res)
			if err != nil {
				return err
			}

			originalYAML[resourceKey(res)] = data
		}

		edited := current.Bytes()

		editID := hash.Sum(nil)

		defer func() {
			if path != "" {
				os.Remove(path) //nolint:errcheck
			}
		}()

		for {
			var (
				buf bytes.Buffer
				w   io.Writer = &buf
			)

			if runtime.GOOS == "windows" {
				w = crlf.NewCRLFWriter(w)
			}

			_, err := fmt.Fprintf(w,
				"# Editing:\n",
			)
			if err != nil {
				return err
			}

			for _, id := range ids {
				_, err = fmt.Fprintf(w,
					"#   %s\n", id,
				)
				if err != nil {
					return err
				}
			}

			if lastError != "" {
				_, err = w.Write([]byte(addEditingComment(lastError)))
				if err != nil {
					return err
				}
			}

			_, err = w.Write(edited)
			if err != nil {
				return err
			}

			editedDiff := edited

			edited, path, err = edit.LaunchTempFile(fmt.Sprintf("%x-edit-", editID), ".yaml", &buf)
			if err != nil {
				return err
			}

			edited = stripComments(edited)

			// If we're retrying the loop because of an error, and no change was made in the file, short-circuit
			if lastError != "" && bytes.Equal(editedDiff, edited) {
				if _, err = os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
					message := lastError

					if lastError[len(lastError)-1] != '\n' {
						message += "\n"
					}

					message += fmt.Sprintf("A copy of your changes has been stored to %q\nEdit canceled, no valid changes were saved.\n", path)

					return errors.New(message)
				}
			}

			if len(bytes.TrimSpace(stripComments(edited))) == 0 {
				fmt.Fprintln(os.Stderr, "Apply was skipped: empty file.")

				break
			}

			// Decode edited documents and compare each against its original.
			// Only apply resources that actually changed.
			editedResources, decodeErr := decodeEditedResources(edited)
			if decodeErr != nil {
				lastError = decodeErr.Error()

				continue
			}

			var changed int

			for _, res := range editedResources {
				editedData, marshalErr := marshalResourceCanonical(res)
				if marshalErr != nil {
					return marshalErr
				}

				originalResource, ok := originalResources[resourceKey(res)]
				if !ok {
					continue
				}

				origData, ok := originalYAML[resourceKey(res)]
				if ok && bytes.Equal(editedData, origData) {
					continue
				}

				changed++

				if err := updateResource(ctx, c.Omni().State(), originalResource, res, editCmdFlags.options); err != nil {
					lastError = err.Error()

					continue
				}

				fmt.Fprintf(os.Stderr, "Updated %s '%s'\n", res.Metadata().Type(), res.Metadata().ID())
			}

			if changed == 0 {
				fmt.Fprintln(os.Stderr, "Apply was skipped: no changes detected.")

				break
			}

			break
		}

		return nil
	}
}

func addEditingComment(in string) string {
	lines := strings.Split(strings.TrimSpace(in), "\n")

	var sb strings.Builder

	fmt.Fprintln(&sb, "#")
	fmt.Fprintln(&sb, "# Edit Failed:")

	for _, line := range lines {
		fmt.Fprintf(&sb, "# %s\n", line)
	}

	return sb.String()
}

// editCmd represents the edit command.
var editCmd = &cobra.Command{
	Use:   "edit <type> [<id>]",
	Short: "Edit Omni resources with the default editor.",
	Args:  cobra.RangeArgs(1, 2),
	Long: `The edit command allows you to directly edit the resources
of an Omni instance using your preferred text editor.

It will open the editor defined by your OMNI_EDITOR, TALOS_EDITOR,
or EDITOR environment variables, or fall back to 'vi' for Linux
or 'notepad' for Windows.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return access.WithClient(func(ctx context.Context, c *client.Client, info access.ServerInfo) error {
			var id string

			if len(args) > 1 {
				id = args[1]
			}

			req, err := createResourceRequest(ctx, c, editCmdFlags.namespace, args[0], id, editCmdFlags.selector, editCmdFlags.idRegexp)
			if err != nil {
				return err
			}

			var resources []resource.Resource

			if req.md.ID() == "" {
				resList, err := c.Omni().State().List(ctx, req.md,
					state.WithListUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
					state.WithLabelQuery(req.labelQueryOptions...),
					state.WithIDQuery(req.idQueryOptions...),
				)
				if err != nil {
					return err
				}

				resources = resList.Items
			} else {
				res, err := c.Omni().State().Get(ctx, req.md,
					state.WithGetUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
				)
				if err != nil {
					return err
				}

				resources = []resource.Resource{res}
			}

			if len(resources) == 0 {
				return fmt.Errorf("no resources found for editing")
			}

			return editFn(c)(ctx, resources, nil)
		})
	},
}

// stripComments strips comments from a YAML file.
//
// If the YAML file is parseable, it will be accurately stripped. Otherwise, it
// will be stripped in a best-effort manner.
func stripComments(b []byte) []byte {
	stripped, err := stripViaDecoding(b)
	if err != nil {
		stripped = stripManual(b)
	}

	return stripped
}

func stripViaDecoding(b []byte) ([]byte, error) {
	var out bytes.Buffer

	decoder := yaml.NewDecoder(bytes.NewReader(b))
	encoder := yaml.NewEncoder(&out)

	for {
		var node yaml.Node

		err := decoder.Decode(&node)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, err
		}

		removeComments(&node)

		if err = encoder.Encode(&node); err != nil {
			return nil, err
		}
	}

	return out.Bytes(), nil
}

func removeComments(node *yaml.Node) {
	node.FootComment = ""
	node.HeadComment = ""
	node.LineComment = ""

	for _, child := range node.Content {
		removeComments(child)
	}
}

func stripManual(b []byte) []byte {
	var stripped []byte

	lines := bytes.Split(b, []byte("\n"))

	for i, line := range lines {
		trimline := bytes.TrimSpace(line)

		// this is not accurate, but best effort
		if bytes.HasPrefix(trimline, []byte("#")) && !bytes.HasPrefix(trimline, []byte("#!")) {
			continue
		}

		stripped = append(stripped, line...)

		if i < len(lines)-1 {
			stripped = append(stripped, '\n')
		}
	}

	return stripped
}

func resourceKey(res resource.Resource) string {
	return res.Metadata().Type() + "/" + res.Metadata().Namespace() + "/" + res.Metadata().ID()
}

func marshalResourceCanonical(res resource.Resource) ([]byte, error) {
	yamlRes, err := resource.MarshalYAML(res)
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(yamlRes)
}

func decodeEditedResources(data []byte) ([]resource.Resource, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))

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

func init() {
	editCmd.Flags().StringVar(&editCmdFlags.namespace, "namespace", "", "resource namespace (default is to use default namespace per resource)")
	editCmd.Flags().BoolVar(&editCmdFlags.options.dryRun, "dry-run", false, "do not apply the change after editing and print the change summary instead")
	editCmd.Flags().BoolVarP(&editCmdFlags.options.verbose, "verbose", "v", false, "Verbose output")
	editCmd.Flags().StringVar(&editCmdFlags.selector, "selector", "", "Selector (label query) to filter on, supports '=' and '==' (e.g. -l key1=value1,key2=value2)")
	editCmd.Flags().StringVar(&editCmdFlags.idRegexp, "id-match-regexp", "", "Match resource ID against a regular expression.")

	RootCmd.AddCommand(editCmd)
}
