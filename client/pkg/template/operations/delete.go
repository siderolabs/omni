// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operations

import (
	"context"
	"fmt"
	"io"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/template"
)

// DeleteTemplate removes all template resources from Omni.
func DeleteTemplate(ctx context.Context, templateReader io.Reader, out io.Writer, st state.State, syncOptions SyncOptions) error {
	tmpl, err := template.Load(templateReader)
	if err != nil {
		return fmt.Errorf("error loading template: %w", err)
	}

	if err = tmpl.Validate(); err != nil {
		return err
	}

	return deleteTemplate(ctx, tmpl, out, st, syncOptions)
}

// DeleteCluster removes all cluster resources from Omni via template operations.
func DeleteCluster(ctx context.Context, clusterName string, out io.Writer, st state.State, syncOptions SyncOptions) error {
	tmpl := template.WithCluster(clusterName)

	return deleteTemplate(ctx, tmpl, out, st, syncOptions)
}

func deleteTemplate(ctx context.Context, tmpl *template.Template, out io.Writer, st state.State, syncOptions SyncOptions) error {
	syncResult, err := tmpl.Delete(ctx, st)
	if err != nil {
		return err
	}

	if syncOptions.DestroyMachines {
		clusterName, err := tmpl.ClusterName()
		if err != nil {
			return err
		}

		machines, err := st.List(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.MachineStatusType, "", resource.VersionUndefined),
			state.WithLabelQuery(resource.LabelExists(omni.MachineStatusLabelDisconnected), resource.LabelEqual(omni.LabelCluster, clusterName)),
		)
		if err != nil {
			return err
		}

		var allPatches []resource.Resource

		links := make([]resource.Resource, 0, len(machines.Items))

		for _, machine := range machines.Items {
			patches, err := st.List(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.ConfigPatchType, "", resource.VersionUndefined),
				state.WithLabelQuery(resource.LabelEqual(omni.LabelMachine, machine.Metadata().ID())),
			)
			if err != nil {
				return err
			}

			allPatches = append(allPatches, patches.Items...)
			links = append(links, siderolink.NewLink(resources.DefaultNamespace, machine.Metadata().ID(), nil))
		}

		syncResult.Destroy = append([][]resource.Resource{links},
			append([][]resource.Resource{allPatches}, syncResult.Destroy...)...)
	}

	return syncDelete(ctx, syncResult, out, st, syncOptions)
}
