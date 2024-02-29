// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package specs contains all resource specs of the service.
package specs

// GetCondition returns condition from the list of conditions of the status if it's set.
func (x *ControlPlaneStatusSpec) GetCondition(condition ConditionType) *ControlPlaneStatusSpec_Condition {
	for _, c := range x.Conditions {
		if c.Type == condition {
			return c
		}
	}

	return &ControlPlaneStatusSpec_Condition{
		Status:   ControlPlaneStatusSpec_Condition_Unknown,
		Severity: ControlPlaneStatusSpec_Condition_Info,
	}
}

// SetCondition updates the conditions: adds new condition if it's not set, or updates the existing one.
func (x *ControlPlaneStatusSpec) SetCondition(condition ConditionType, status ControlPlaneStatusSpec_Condition_Status, severity ControlPlaneStatusSpec_Condition_Severity, reason string) {
	c := &ControlPlaneStatusSpec_Condition{
		Type:     condition,
		Status:   status,
		Severity: severity,
		Reason:   reason,
	}

	for i, cond := range x.Conditions {
		if cond.Type == condition {
			x.Conditions[i] = c

			return
		}
	}

	x.Conditions = append(x.Conditions, c)
}
