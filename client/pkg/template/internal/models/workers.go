// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package models

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// KindWorkers is Workers model kind.
const KindWorkers = "Workers"

// Workers describes Cluster worker nodes.
type Workers struct {
	MachineSet `yaml:",inline"`
}

// Validate the model.
func (workers *Workers) Validate() error {
	var multiErr error

	if workers.Name == omni.ControlPlanesIDSuffix {
		multiErr = multierror.Append(multiErr, fmt.Errorf("name %q cannot be used in workers", omni.ControlPlanesIDSuffix))
	}

	if workers.BootstrapSpec != nil {
		multiErr = multierror.Append(multiErr, fmt.Errorf("bootstrapSpec is not allowed in workers"))
	}

	multiErr = joinErrors(multiErr, workers.MachineSet.Validate(), workers.Machines.Validate(), workers.Patches.Validate())

	if multiErr != nil {
		return fmt.Errorf("workers is invalid: %w", multiErr)
	}

	return nil
}

// Translate the model.
func (workers *Workers) Translate(ctx TranslateContext) ([]resource.Resource, error) {
	var nameSuffix string

	if workers.Name == "" {
		nameSuffix = omni.DefaultWorkersIDSuffix
	} else {
		nameSuffix = workers.Name
	}

	return workers.translate(ctx, nameSuffix, omni.LabelWorkerRole)
}

func init() {
	register[Workers](KindWorkers)
}
