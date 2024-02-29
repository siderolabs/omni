// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"context"
	"fmt"
	"regexp"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/omnictl/output"
)

var getCmdFlags struct {
	namespace string
	output    string
	selector  string
	idRegexp  string
	watch     bool
}

// getCmd represents the get (resources) command.
var getCmd = &cobra.Command{
	Use:     "get <type> [<id>]",
	Aliases: []string{"g"},
	Short:   "Get a specific resource or list of resources.",
	Long: `Similar to 'kubectl get', 'omnictl get' returns a set of resources from the OS.
To get a list of all available resource definitions, issue 'omnictl get rd'`,
	Example: "",
	Args:    cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return access.WithClient(getResources(cmd, args))
	},
}

//nolint:gocognit,gocyclo,cyclop,maintidx
func getResources(cmd *cobra.Command, args []string) func(ctx context.Context, client *client.Client) error {
	return func(ctx context.Context, client *client.Client) error {
		st := client.Omni().State()

		var (
			resourceType = resource.Type(args[0]) //nolint:unconvert
			resourceID   resource.ID
		)

		if len(args) > 1 {
			resourceID = args[1]
		}

		rd, err := resolveResourceType(ctx, st, resourceType)
		if err != nil {
			return err
		}

		if !cmd.Flags().Lookup("namespace").Changed {
			getCmdFlags.namespace = rd.TypedSpec().DefaultNamespace
		}

		var labelQuery []resource.LabelQueryOption

		if getCmdFlags.selector != "" {
			if resourceID != "" {
				return fmt.Errorf("cannot specify both resource ID and selector")
			}

			var query *resource.LabelQuery

			query, err = labels.ParseQuery(getCmdFlags.selector)
			if err != nil {
				return err
			}

			labelQuery = append(labelQuery, resource.RawLabelQuery(*query))
		}

		var idQuery []resource.IDQueryOption

		if getCmdFlags.idRegexp != "" {
			if resourceID != "" {
				return fmt.Errorf("cannot specify both resource ID and ID regexp")
			}

			var idRegexp *regexp.Regexp

			idRegexp, err = regexp.Compile(getCmdFlags.idRegexp)
			if err != nil {
				return fmt.Errorf("invalid ID regexp: %w", err)
			}

			idQuery = append(idQuery, resource.IDRegexpMatch(idRegexp))
		}

		out, err := output.NewWriter(getCmdFlags.output)
		if err != nil {
			return err
		}

		defer out.Flush() //nolint:errcheck

		if err = out.WriteHeader(rd, getCmdFlags.watch); err != nil {
			return err
		}

		md := resource.NewMetadata(getCmdFlags.namespace, rd.TypedSpec().Type, resourceID, resource.VersionUndefined)

		switch {
		case resourceID == "" && !getCmdFlags.watch:
			items, err := st.List(ctx, md,
				state.WithLabelQuery(labelQuery...),
				state.WithIDQuery(idQuery...),
			)
			if err != nil {
				return err
			}

			for _, item := range items.Items {
				if err = out.WriteResource(item, state.EventType(0)); err != nil {
					return err
				}
			}
		case resourceID == "" && getCmdFlags.watch:
			watchCh := make(chan state.Event)

			err := st.WatchKind(ctx, md, watchCh,
				state.WithBootstrapContents(true),
				state.WatchWithLabelQuery(labelQuery...),
				state.WatchWithIDQuery(idQuery...),
			)
			if err != nil {
				return err
			}

			bootstrapped := false

		watchLoopKind:
			for {
				select {
				case e := <-watchCh:
					if e.Type == state.Errored {
						return fmt.Errorf("watch error: %w", e.Error)
					}

					if e.Type == state.Bootstrapped {
						bootstrapped = true

						if err = out.Flush(); err != nil {
							return err
						}

						continue
					}

					if e.Resource == nil {
						continue
					}

					if err = out.WriteResource(e.Resource, e.Type); err != nil {
						return err
					}

					if bootstrapped {
						if err = out.Flush(); err != nil {
							return err
						}
					}
				case <-ctx.Done():
					break watchLoopKind
				}
			}
		case resourceID != "" && !getCmdFlags.watch:
			res, err := st.Get(ctx, md)
			if err != nil {
				return err
			}

			if err = out.WriteResource(res, state.EventType(0)); err != nil {
				return err
			}
		case resourceID != "" && getCmdFlags.watch:
			watchCh := make(chan state.Event)

			err := st.Watch(ctx, md, watchCh)
			if err != nil {
				return err
			}

		watchLoop:
			for {
				select {
				case e := <-watchCh:
					if e.Type == state.Errored {
						return fmt.Errorf("watch error: %w", e.Error)
					}

					if e.Resource == nil {
						continue
					}

					if err = out.WriteResource(e.Resource, e.Type); err != nil {
						return err
					}

					if err = out.Flush(); err != nil {
						return err
					}
				case <-ctx.Done():
					break watchLoop
				}
			}
		}

		return nil
	}
}

func init() {
	getCmd.PersistentFlags().StringVarP(&getCmdFlags.namespace, "namespace", "n", resources.DefaultNamespace, "The resource namespace.")
	getCmd.PersistentFlags().BoolVarP(&getCmdFlags.watch, "watch", "w", false, "Watch the resource state.")
	getCmd.PersistentFlags().StringVarP(&getCmdFlags.output, "output", "o", "table", "Output format (json, table, yaml, jsonpath).")
	getCmd.PersistentFlags().StringVarP(&getCmdFlags.selector, "selector", "l", "", "Selector (label query) to filter on, supports '=' and '==' (e.g. -l key1=value1,key2=value2)")
	getCmd.PersistentFlags().StringVar(&getCmdFlags.idRegexp, "id-match-regexp", "", "Match resource ID against a regular expression.")

	if err := getCmd.RegisterFlagCompletionFunc("output", output.CompleteOutputArg); err != nil {
		panic(err)
	}

	RootCmd.AddCommand(getCmd)
}
