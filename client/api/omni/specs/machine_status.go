// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package specs

func (spec *MachineStatusSpec) SchematicReady() bool {
	if spec.Schematic == nil {
		return false
	}

	if spec.Schematic.InAgentMode {
		return false
	}

	if spec.Schematic.Invalid {
		// If the schematic is invalid, we consider it "ready" to allow it to be allocated to a cluster.
		// This is the case for the machines which were built bypassing image factory and with extensions in them:
		// we mark those as invalid, as we do not know if those extensions were official (available also in image factory),
		// but those machines still need to be usable - users should be able to create clusters with those machines.
		return true
	}

	return spec.Schematic.Id != "" && spec.Schematic.FullId != ""
}
