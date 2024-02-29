// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operations

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/fatih/color"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/template"
	"github.com/siderolabs/omni/client/pkg/template/operations/internal/utils"
)

// SyncOptions contains options for SyncTemplate.
type SyncOptions struct {
	// DryRun indicates that no changes should be made to the cluster.
	DryRun bool

	// Verbose indicates that diff for each resource should be printed.
	Verbose bool

	// DestroyMachines forcefully remove the disconnected nodes from Omni.
	DestroyMachines bool
}

// SyncTemplate performs resource sync to Omni.
func SyncTemplate(ctx context.Context, templateReader io.Reader, out io.Writer, st state.State, syncOptions SyncOptions) error {
	tmpl, err := template.Load(templateReader)
	if err != nil {
		return fmt.Errorf("error loading template: %w", err)
	}

	if err = tmpl.Validate(); err != nil {
		return err
	}

	syncResult, err := tmpl.Sync(ctx, st)
	if err != nil {
		return fmt.Errorf("error syncing template: %w", err)
	}

	// sync flow:
	//  1. create missing resources
	//  2. update resources
	//  3. delete resources last
	//
	// this follows the idea of a scaling up first

	yellow := color.New(color.FgYellow)
	boldFunc := color.New(color.Bold).SprintfFunc()

	dryRun := ""
	if syncOptions.DryRun {
		dryRun = " (dry run)"
	}

	for _, r := range syncResult.Create {
		yellow.Fprintf(out, "* creating%s %s\n", dryRun, boldFunc(utils.Describe(r))) //nolint:errcheck

		if syncOptions.Verbose {
			if err = utils.RenderDiff(out, nil, r); err != nil {
				return err
			}
		}

		if syncOptions.DryRun {
			continue
		}

		if err = st.Create(ctx, r); err != nil {
			return err
		}
	}

	for _, p := range syncResult.Update {
		yellow.Fprintf(out, "* updating%s %s\n", dryRun, boldFunc(utils.Describe(p.New))) //nolint:errcheck

		if syncOptions.Verbose {
			if err = utils.RenderDiff(os.Stdout, p.Old, p.New); err != nil {
				return err
			}
		}

		if syncOptions.DryRun {
			continue
		}

		if err = st.Update(ctx, p.New); err != nil {
			return err
		}
	}

	return syncDelete(ctx, syncResult, out, st, syncOptions)
}

func syncDelete(ctx context.Context, syncResult *template.SyncResult, out io.Writer, st state.State, syncOptions SyncOptions) error {
	for _, phase := range syncResult.Destroy {
		if err := syncDeleteResources(ctx, phase, out, st, syncOptions); err != nil {
			return err
		}
	}

	return nil
}

//nolint:gocognit,gocyclo,cyclop
func syncDeleteResources(ctx context.Context, toDelete []resource.Resource, out io.Writer, st state.State, syncOptions SyncOptions) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	yellow := color.New(color.FgYellow)
	boldFunc := color.New(color.Bold).SprintfFunc()

	dryRun := ""
	if syncOptions.DryRun {
		dryRun = " (dry run)"
	}

	teardownWatch := make(chan state.Event)
	tearingDownResourceTypes := map[resource.Type]struct{}{}

	for _, r := range toDelete {
		tearingDownResourceTypes[r.Metadata().Type()] = struct{}{}
	}

	for resourceType := range tearingDownResourceTypes {
		if err := st.WatchKind(ctx, resource.NewMetadata(resources.DefaultNamespace, resourceType, "", resource.VersionUndefined), teardownWatch, state.WithBootstrapContents(true)); err != nil {
			return err
		}
	}

	tearingDownResources := map[string]struct{}{}

	for _, r := range toDelete {
		yellow.Fprintf(out, "* tearing down%s %s\n", dryRun, boldFunc(utils.Describe(r))) //nolint:errcheck

		if syncOptions.Verbose {
			if err := utils.RenderDiff(os.Stdout, r, nil); err != nil {
				return err
			}
		}

		if syncOptions.DryRun {
			continue
		}

		if _, err := st.Teardown(ctx, r.Metadata()); err != nil && !state.IsNotFoundError(err) {
			return err
		}

		tearingDownResources[utils.Describe(r)] = struct{}{}
	}

	if syncOptions.DryRun {
		return nil
	}

	for len(tearingDownResources) > 0 {
		var event state.Event

		select {
		case <-ctx.Done():
			return ctx.Err()
		case event = <-teardownWatch:
		}

		switch event.Type {
		case state.Updated, state.Created:
			if _, ok := tearingDownResources[utils.Describe(event.Resource)]; ok {
				if event.Resource.Metadata().Phase() == resource.PhaseTearingDown && event.Resource.Metadata().Finalizers().Empty() {
					if err := st.Destroy(ctx, event.Resource.Metadata()); err != nil && !state.IsNotFoundError(err) {
						return err
					}
				}
			}
		case state.Destroyed:
			if _, ok := tearingDownResources[utils.Describe(event.Resource)]; ok {
				delete(tearingDownResources, utils.Describe(event.Resource))

				yellow.Fprintf(out, "* destroyed%s %s\n", dryRun, boldFunc(utils.Describe(event.Resource))) //nolint:errcheck
			}
		case state.Bootstrapped:
			// ignore
		case state.Errored:
			return event.Error
		}
	}

	return nil
}
