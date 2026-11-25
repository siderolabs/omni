// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package statustree

import (
	"fmt"

	"github.com/fatih/color"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/internal/safeout"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func readyString(ready bool) string {
	if ready {
		return color.GreenString("Ready")
	}

	return color.RedString("Not Ready")
}

func clusterPhaseString(phase specs.ClusterStatusSpec_Phase) string {
	phaseString := phase.String()

	var c func(string, ...any) string

	switch phase {
	case specs.ClusterStatusSpec_UNKNOWN:
		c = color.YellowString
	case specs.ClusterStatusSpec_SCALING_UP,
		specs.ClusterStatusSpec_SCALING_DOWN:
		c = color.HiYellowString
	case specs.ClusterStatusSpec_DESTROYING:
		c = color.HiRedString
	case specs.ClusterStatusSpec_RUNNING:
		c = color.GreenString
	default:
		c = fmt.Sprintf
	}

	return c(phaseString)
}

func kubernetesUpgradePhaseString(phase specs.KubernetesUpgradeStatusSpec_Phase) string {
	phaseString := phase.String()

	var c func(string, ...any) string

	switch phase {
	case specs.KubernetesUpgradeStatusSpec_Done:
		c = color.GreenString
	case specs.KubernetesUpgradeStatusSpec_Upgrading, specs.KubernetesUpgradeStatusSpec_Reverting:
		c = color.HiYellowString
	case specs.KubernetesUpgradeStatusSpec_Failed:
		c = color.HiRedString
	case specs.KubernetesUpgradeStatusSpec_Unknown:
		c = fmt.Sprintf
	default:
		c = fmt.Sprintf
	}

	return c(phaseString)
}

func talosUpgradePhaseString(phase specs.TalosUpgradeStatusSpec_Phase) string {
	phaseString := phase.String()

	var c func(string, ...any) string

	switch phase {
	case specs.TalosUpgradeStatusSpec_Done:
		c = color.GreenString
	case specs.TalosUpgradeStatusSpec_Upgrading, specs.TalosUpgradeStatusSpec_Reverting, specs.TalosUpgradeStatusSpec_UpdatingMachineSchematics:
		c = color.HiYellowString
	case specs.TalosUpgradeStatusSpec_Failed:
		c = color.HiRedString
	case specs.TalosUpgradeStatusSpec_Unknown:
		c = fmt.Sprintf
	default:
		c = fmt.Sprintf
	}

	return c(phaseString)
}

func machineSetPhaseString(phase specs.MachineSetPhase) string {
	phaseString := phase.String()

	var c func(string, ...any) string

	switch phase {
	case specs.MachineSetPhase_Unknown:
		c = color.YellowString
	case specs.MachineSetPhase_ScalingUp,
		specs.MachineSetPhase_ScalingDown,
		specs.MachineSetPhase_Upgrading,
		specs.MachineSetPhase_Reconfiguring:
		c = color.HiYellowString
	case specs.MachineSetPhase_Destroying, specs.MachineSetPhase_Failed:
		c = color.HiRedString
	case specs.MachineSetPhase_Running:
		c = color.GreenString
	default:
		c = fmt.Sprintf
	}

	return c(phaseString)
}

func clusterMachineStageString(phase specs.ClusterMachineStatusSpec_Stage) string {
	phaseString := phase.String()

	var c func(string, ...any) string

	switch phase {
	case specs.ClusterMachineStatusSpec_UNKNOWN:
		c = color.YellowString
	case specs.ClusterMachineStatusSpec_CONFIGURING,
		specs.ClusterMachineStatusSpec_INSTALLING,
		specs.ClusterMachineStatusSpec_UPGRADING,
		specs.ClusterMachineStatusSpec_REBOOTING,
		specs.ClusterMachineStatusSpec_BOOTING,
		specs.ClusterMachineStatusSpec_POWERING_ON:
		c = color.HiYellowString
	case specs.ClusterMachineStatusSpec_BEFORE_DESTROY,
		specs.ClusterMachineStatusSpec_DESTROYING,
		specs.ClusterMachineStatusSpec_SHUTTING_DOWN:
		c = color.HiRedString
	case specs.ClusterMachineStatusSpec_RUNNING:
		c = color.GreenString
	case specs.ClusterMachineStatusSpec_POWERED_OFF:
		c = color.WhiteString
	default:
		c = fmt.Sprintf
	}

	return c(phaseString)
}

func clusterMachineNodeNameString(clusterMachine *omni.ClusterMachineStatus) string {
	nodename, _ := clusterMachine.Metadata().Labels().Get(omni.ClusterMachineStatusLabelNodeName)
	if nodename == "" {
		return ""
	}

	return color.CyanString("(%s)", safeout.Cell(nodename))
}

func clusterMachineLockedString(clusterMachine *omni.ClusterMachineStatus) string {
	if _, locked := clusterMachine.Metadata().Annotations().Get(omni.MachineLocked); !locked {
		return ""
	}

	return " " + color.BlueString("Locked")
}

func clusterMachineConnected(clusterMachine *omni.ClusterMachineStatus) string {
	_, connected := clusterMachine.Metadata().Labels().Get(omni.MachineStatusLabelConnected)
	if connected {
		return ""
	}

	return " " + color.RedString("Unreachable")
}

func clusterMachineConfigOutdated(outdated bool) string {
	if !outdated {
		return ""
	}

	return " " + color.YellowString("Config Outdated")
}

func clusterMachineConfigStatus(node *omni.ClusterMachineStatus) string {
	if _, locked := node.Metadata().Labels().Get(omni.UpdateLocked); locked {
		return " " + color.BlueString("Pending Config Update")
	}

	switch node.TypedSpec().Value.ConfigApplyStatus {
	case specs.ConfigApplyStatus_UNKNOWN,
		specs.ConfigApplyStatus_PENDING,
		specs.ConfigApplyStatus_APPLIED:
		return ""
	case specs.ConfigApplyStatus_FAILED:
		return " " + color.RedString("Config Apply Failed")
	default:
		return ""
	}
}

func clusterMachineOperationString(operation *omni.MachineOperationStatus) string {
	if operation == nil || operation.TypedSpec().Value.Stage == specs.MachineOperationStatusSpec_UNKNOWN {
		return ""
	}

	result := " - " + color.HiYellowString(operation.TypedSpec().Value.Stage.String())

	if operation.TypedSpec().Value.Message != "" {
		result += ": " + safeout.Cell(operation.TypedSpec().Value.Message)
	}

	return result
}

// clusterMachineDryRunString uses its own color, as a dry run never changes the machine and must not read as a failed apply.
func clusterMachineDryRunString(dryRun *omni.DryRunStatus) string {
	spec := dryRun.TypedSpec().Value

	// an outcome added by a newer Omni is shown as unknown
	summary := "unknown"

	switch spec.Outcome {
	case specs.DryRunStatusSpec_PASSED:
		summary = "passed"

		if spec.RebootRequired {
			summary += ", reboots on unlock"
		}
	case specs.DryRunStatusSpec_FAILED:
		summary = "failed"
	case specs.DryRunStatusSpec_NO_PENDING_CHANGES:
		summary = "no pending changes"
	case specs.DryRunStatusSpec_NOT_CHECKED:
		summary = "not checked (" + dryRunNotCheckedReasonString(spec.NotCheckedReason) + ")"
	case specs.DryRunStatusSpec_UNKNOWN:
	}

	result := color.MagentaString("Dry run: %s", summary)

	// set only for a failed dry run and for a check error
	if spec.Error != "" {
		result += "\n" + safeout.String(spec.Error)
	}

	return result
}

func dryRunNotCheckedReasonString(reason specs.DryRunStatusSpec_NotCheckedReason) string {
	switch reason {
	case specs.DryRunStatusSpec_DISCONNECTED:
		return "disconnected"
	case specs.DryRunStatusSpec_TRY_IN_FLIGHT:
		return "config try in flight"
	case specs.DryRunStatusSpec_MAINTENANCE:
		return "maintenance mode"
	case specs.DryRunStatusSpec_NOT_RUNNING:
		return "not running"
	case specs.DryRunStatusSpec_CHECK_ERROR:
		return "check failed"
	case specs.DryRunStatusSpec_NONE:
		return "unknown"
	}

	return "unknown"
}

func clusterMachineReadyString(clusterMachine *omni.ClusterMachineStatus) string {
	if clusterMachine.TypedSpec().Value.Stage != specs.ClusterMachineStatusSpec_RUNNING {
		return ""
	}

	return " " + readyString(clusterMachine.TypedSpec().Value.Ready)
}

func controlPlaneStatusString(cpStatus *omni.ControlPlaneStatus) string {
	var failedConditions []specs.ConditionType

	for _, condition := range cpStatus.TypedSpec().Value.GetConditions() {
		if condition.GetStatus() == specs.ControlPlaneStatusSpec_Condition_NotReady {
			failedConditions = append(failedConditions, condition.GetType())
		}
	}

	if len(failedConditions) == 0 {
		return color.GreenString("OK")
	}

	return color.RedString("Failing: %s", failedConditions)
}

func machineSetName(machineSet *omni.MachineSetStatus) string {
	if _, ok := machineSet.Metadata().Labels().Get(omni.LabelControlPlaneRole); ok {
		return "Control Plane"
	}

	cluster, _ := machineSet.Metadata().Labels().Get(omni.LabelCluster)

	if machineSet.Metadata().ID() == omni.WorkersResourceID(cluster) {
		return "Workers"
	}

	return "Additional Workers"
}
