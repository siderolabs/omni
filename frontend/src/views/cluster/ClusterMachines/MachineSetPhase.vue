<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div
    v-bind:style="'color: ' + phaseColor(item)"
    class="cluster-phase-box"
  >
    <t-icon :icon="phaseIcon(item)" class="h-4" />
    <div class="cluster-phase-name">{{ phaseName(item) || "" }}</div>
  </div>
</template>

<script setup lang="ts">
import TIcon, { IconType } from "@/components/common/Icon/TIcon.vue";
import { MachineSetPhase, MachineSetStatusSpec } from "@/api/omni/specs/omni.pb";
import { ResourceTyped } from "@/api/grpc";

const phaseName = (machineset: ResourceTyped<MachineSetStatusSpec>): string => {
  switch (machineset?.spec.phase) {
    case MachineSetPhase.ScalingUp:
      return "Scaling Up";
    case MachineSetPhase.ScalingDown:
      return "Scaling Down";
    case MachineSetPhase.Running:
      if (machineset?.spec.ready) {
        return "Running";
      } else {
        return "Not Ready";
      }
    case MachineSetPhase.Destroying:
      return "Destroying";
    case MachineSetPhase.Failed:
      return "Failed";
    case MachineSetPhase.Reconfiguring:
      return "Reconfiguring";
    default:
      return "Unknown";
  }
};

const phaseIcon = (machineset: ResourceTyped<MachineSetStatusSpec>): IconType => {
  switch (machineset?.spec.phase) {
    case MachineSetPhase.ScalingUp:
    case MachineSetPhase.ScalingDown:
    case MachineSetPhase.Reconfiguring:
      return "loading";
    case MachineSetPhase.Running:
      if (machineset?.spec.ready) {
        return "check-in-circle";
      } else {
        return "error";
      }
    case MachineSetPhase.Destroying:
      return "delete";
    case MachineSetPhase.Failed:
      return "error";
    default:
      return "unknown";
  }
};

const phaseColor = (machineset): string => {
  const Y1 = "#FFB200";

  switch (machineset?.spec.phase) {
    case MachineSetPhase.ScalingUp:
    case MachineSetPhase.ScalingDown:
    case MachineSetPhase.Reconfiguring:
      return Y1;
    case MachineSetPhase.Running:
      if (machineset?.spec.ready) {
        return "#69C297";
      } else {
        return "#FF5F2A";
      }
    case MachineSetPhase.Failed:
    case MachineSetPhase.Destroying:
      return "#FF5F2A";
    default:
      return Y1;
  }
};

type Props = {
  item: ResourceTyped<MachineSetStatusSpec>;
};

defineProps<Props>();

</script>

<style>
.cluster-phase-box {
  display: flex;
  align-items: center;
}

.cluster-phase-name {
  margin-left: 5px;
}
</style>
