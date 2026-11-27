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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
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
func SyncTemplate(ctx context.Context, templateReader io.Reader, out io.Writer, st state.State, syncOptions SyncOptions, root *os.Root) error {
	tmpl, err := template.Load(templateReader, template.WithRoot(root))
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

	if err = tmpl.CheckClusterLock(syncResult); err != nil {
		return err
	}

	return sync(ctx, syncResult, out, st, syncOptions)
}

// sync flow:
//  1. create missing resources
//  2. update resources
//  3. delete resources last
//
// this follows the idea of a scaling up first.
func sync(ctx context.Context, syncResult *template.SyncResult, out io.Writer, st state.State, syncOptions SyncOptions) error {
	yellow := color.New(color.FgYellow)
	boldFunc := color.New(color.Bold).SprintfFunc()

	dryRun := ""
	if syncOptions.DryRun {
		dryRun = " (dry run)"
	}

	lockChanges, updates := splitClusterLockChanges(syncResult.Update)

	// locking first holds back the config changes below, unlocking first allows the machine set changes below
	for _, p := range lockChanges {
		if err := syncUpdate(ctx, p, out, st, syncOptions); err != nil {
			return err
		}
	}

	for _, r := range syncResult.Create {
		yellow.Fprintf(out, "* creating%s %s\n", dryRun, boldFunc(utils.Describe(r))) //nolint:errcheck

		if syncOptions.Verbose {
			if err := utils.RenderDiff(out, nil, r); err != nil {
				return err
			}
		}

		if syncOptions.DryRun {
			continue
		}

		if err := st.Create(ctx, r); err != nil {
			return err
		}
	}

	for _, p := range updates {
		if err := syncUpdate(ctx, p, out, st, syncOptions); err != nil {
			return err
		}
	}

	return syncDelete(ctx, syncResult, out, st, syncOptions)
}

func syncUpdate(ctx context.Context, p template.UpdateChange, out io.Writer, st state.State, syncOptions SyncOptions) error {
	dryRun := ""
	if syncOptions.DryRun {
		dryRun = " (dry run)"
	}

	color.New(color.FgYellow).Fprintf(out, "* updating%s %s\n", dryRun, color.New(color.Bold).Sprint(utils.Describe(p.New))) //nolint:errcheck

	if syncOptions.Verbose {
		if err := utils.RenderDiff(out, p.Old, p.New); err != nil {
			return err
		}
	}

	if syncOptions.DryRun {
		return nil
	}

	err := st.Update(ctx, p.New)
	if err != nil && isRefused(err) && releasesFromTemplate(p) {
		// The sync does not change the resource here, it only stops marking it as managed by the template.
		// A user who manages the cluster, but not the settings of its machines, cannot do that, and it must not fail the sync.
		color.New(color.FgYellow).Fprintf(out, "* skipped %s: no access, it stays marked as managed by cluster templates\n", color.New(color.Bold).Sprint(utils.Describe(p.New))) //nolint:errcheck

		return nil
	}

	return err
}

// releasesFromTemplate reports whether the update removes the mark of the resources managed by cluster templates.
func releasesFromTemplate(p template.UpdateChange) bool {
	_, wasManaged := p.Old.Metadata().Annotations().Get(omni.ResourceManagedByClusterTemplates)
	_, isManaged := p.New.Metadata().Annotations().Get(omni.ResourceManagedByClusterTemplates)

	return wasManaged && !isManaged
}

// isRefused reports whether Omni refused the write because the caller has no access.
// The state client of omnictl returns such a refusal as an owner conflict.
func isRefused(err error) bool {
	return status.Code(err) == codes.PermissionDenied || state.IsOwnerConflictError(err)
}

func splitClusterLockChanges(updates []template.UpdateChange) (lockChanges, rest []template.UpdateChange) {
	for _, p := range updates {
		if p.New.Metadata().Type() != omni.ClusterType {
			rest = append(rest, p)

			continue
		}

		_, wasLocked := p.Old.Metadata().Annotations().Get(omni.ClusterLocked)
		_, locked := p.New.Metadata().Annotations().Get(omni.ClusterLocked)

		if wasLocked == locked {
			rest = append(rest, p)

			continue
		}

		lockChanges = append(lockChanges, p)
	}

	return lockChanges, rest
}

func syncDelete(ctx context.Context, syncResult *template.SyncResult, out io.Writer, st state.State, syncOptions SyncOptions) error {
	for _, phase := range syncResult.Destroy {
		if err := syncDeleteResources(ctx, phase, out, st, syncOptions); err != nil {
			return err
		}
	}

	return nil
}

// watchTeardown starts the watches which follow the teardown of the resources.
//
// A user who has access to a single cluster cannot watch all resources of a type. So the resources of a cluster are watched
// using the cluster label, and a resource without the label (the cluster itself, a setting of a machine) is watched by its ID.
func watchTeardown(ctx context.Context, st state.State, toDelete []resource.Resource, events chan<- state.Event) error {
	watched := map[resource.Type]struct{}{}

	for _, r := range toDelete {
		cluster, ok := r.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			if err := st.Watch(ctx, r.Metadata(), events); err != nil {
				return err
			}

			continue
		}

		if _, ok = watched[r.Metadata().Type()]; ok {
			continue
		}

		watched[r.Metadata().Type()] = struct{}{}

		if err := st.WatchKind(
			ctx, resource.NewMetadata(resources.DefaultNamespace, r.Metadata().Type(), "", resource.VersionUndefined), events,
			state.WithBootstrapContents(true), state.WatchWithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster)),
		); err != nil {
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

	if err := watchTeardown(ctx, st, toDelete, teardownWatch); err != nil {
		return err
	}

	tearingDownResources := map[string]struct{}{}

	for _, r := range toDelete {
		yellow.Fprintf(out, "* tearing down%s %s\n", dryRun, boldFunc(utils.Describe(r))) //nolint:errcheck

		if syncOptions.Verbose {
			if err := utils.RenderDiff(out, r, nil); err != nil {
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
		case state.Bootstrapped, state.Noop:
			// ignore
		case state.Errored:
			return event.Error
		}
	}

	return nil
}
