// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"context"
	"fmt"
	"regexp"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	omniresources "github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
	"github.com/siderolabs/omni/client/pkg/omnictl/output"
	"github.com/siderolabs/omni/client/pkg/omnictl/resources"
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
		return access.WithClient(getResources(args))
	},
}

//nolint:gocognit,gocyclo,cyclop,maintidx
func getResources(args []string) func(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
	return func(ctx context.Context, client *client.Client, _ access.ServerInfo) error {
		st := client.Omni().State()

		var id string

		if len(args) > 1 {
			id = args[1]
		}

		req, err := createResourceRequest(ctx, client, getCmdFlags.namespace, args[0], id, getCmdFlags.selector, getCmdFlags.idRegexp)
		if err != nil {
			return err
		}

		out, err := output.NewWriter(getCmdFlags.output)
		if err != nil {
			return err
		}

		defer out.Flush() //nolint:errcheck

		if err = out.WriteHeader(req.rd, getCmdFlags.watch); err != nil {
			return err
		}

		switch {
		case req.md.ID() == "" && !getCmdFlags.watch:
			opts := []state.ListOption{
				state.WithListUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
			}

			if len(req.labelQueryOptions) > 0 {
				opts = append(opts, state.WithLabelQuery(req.labelQueryOptions...))
			}

			if len(req.idQueryOptions) > 0 {
				opts = append(opts, state.WithIDQuery(req.idQueryOptions...))
			}

			items, err := st.List(ctx, req.md,
				opts...,
			)
			if err != nil {
				return err
			}

			for _, item := range items.Items {
				if err = out.WriteResource(item, state.EventType(0)); err != nil {
					return err
				}
			}
		case req.md.ID() == "" && getCmdFlags.watch:
			watchCh := make(chan state.Event)

			err := st.WatchKind(ctx, req.md, watchCh,
				state.WithBootstrapContents(true),
				state.WatchWithLabelQuery(req.labelQueryOptions...),
				state.WatchWithIDQuery(req.idQueryOptions...),
				state.WithWatchKindUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
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
		case req.md.ID() != "" && !getCmdFlags.watch:
			res, err := st.Get(ctx, req.md,
				state.WithGetUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
			)
			if err != nil {
				return err
			}

			if err = out.WriteResource(res, state.EventType(0)); err != nil {
				return err
			}
		case req.md.ID() != "" && getCmdFlags.watch:
			watchCh := make(chan state.Event)

			err := st.Watch(ctx, req.md, watchCh,
				state.WithWatchUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
			)
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

type resourceRequest struct {
	rd                *meta.ResourceDefinition
	idQueryOptions    []resource.IDQueryOption
	labelQueryOptions []resource.LabelQueryOption
	md                resource.Metadata
}

func createResourceRequest(
	ctx context.Context,
	client *client.Client,
	namespace string,
	resourceType string,
	resourceID resource.ID,
	selector string,
	idRegexp string,
) (resourceRequest, error) {
	st := client.Omni().State()

	rd, err := resources.ResolveType(ctx, st, resourceType)
	if err != nil {
		return resourceRequest{}, err
	}

	if namespace == "" {
		namespace = rd.TypedSpec().DefaultNamespace
	}

	var labelQuery []resource.LabelQueryOption

	if selector != "" {
		if resourceID != "" {
			return resourceRequest{}, fmt.Errorf("cannot specify both resource ID and selector")
		}

		var query *resource.LabelQuery

		query, err = labels.ParseQuery(selector)
		if err != nil {
			return resourceRequest{}, err
		}

		labelQuery = append(labelQuery, resource.RawLabelQuery(*query))
	}

	var idQuery []resource.IDQueryOption

	if idRegexp != "" {
		if resourceID != "" {
			return resourceRequest{}, fmt.Errorf("cannot specify both resource ID and ID regexp")
		}

		var compiledIDRegexp *regexp.Regexp

		compiledIDRegexp, err = regexp.Compile(idRegexp)
		if err != nil {
			return resourceRequest{}, fmt.Errorf("invalid ID regexp: %w", err)
		}

		idQuery = append(idQuery, resource.IDRegexpMatch(compiledIDRegexp))
	}

	return resourceRequest{
		md:                resource.NewMetadata(namespace, rd.TypedSpec().Type, resourceID, resource.VersionUndefined),
		idQueryOptions:    idQuery,
		labelQueryOptions: labelQuery,
		rd:                rd,
	}, nil
}

func init() {
	getCmd.PersistentFlags().StringVarP(&getCmdFlags.namespace, "namespace", "n", omniresources.DefaultNamespace, "The resource namespace.")
	getCmd.PersistentFlags().BoolVarP(&getCmdFlags.watch, "watch", "w", false, "Watch the resource state.")
	getCmd.PersistentFlags().StringVarP(&getCmdFlags.output, "output", "o", "table", "Output format (json, table, yaml, jsonpath).")
	getCmd.PersistentFlags().StringVarP(&getCmdFlags.selector, "selector", "l", "", "Selector (label query) to filter on, supports '=' and '==' (e.g. -l key1=value1,key2=value2)")
	getCmd.PersistentFlags().StringVar(&getCmdFlags.idRegexp, "id-match-regexp", "", "Match resource ID against a regular expression.")

	if err := getCmd.RegisterFlagCompletionFunc("output", output.CompleteOutputArg); err != nil {
		panic(err)
	}

	RootCmd.AddCommand(getCmd)
}
