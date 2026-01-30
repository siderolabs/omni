// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package specs

func (spec *MachineStatusSpec) SchematicReady() bool {
	return spec.Schematic != nil && !spec.Schematic.InAgentMode && spec.Schematic.Id != "" && spec.Schematic.FullId != ""
}
