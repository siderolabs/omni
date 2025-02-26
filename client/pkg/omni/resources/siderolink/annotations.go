// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

const (
	// SystemPrefix defines siderolink annotations system prefix.
	SystemPrefix = "siderolink.omni.sidero.dev/"
)

const (
	// PendingMachineUUIDConflict is the annotation set on the pending machine to mark it as having UUID conflict.
	// It is used internally by the SideroLink provision code.
	PendingMachineUUIDConflict = SystemPrefix + "uuid-conflict"

	// ForceValidNodeUniqueToken is the annotation that is set on the link resources when the Talos installation is detected.
	// It is used internally by the SideroLink provision code.
	ForceValidNodeUniqueToken = SystemPrefix + "force-valid-node-unique-token"
)
