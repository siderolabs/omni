<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex gap-2">
    <div
      v-bind:style="'color: ' + phaseColor(item)"
      class="cluster-phase-box"
      >
        <t-icon :icon="phaseIcon(item)" class="h-4"/>
        <div id="machine-set-phase-name">{{ phaseName(item) || "" }}</div>
      </div>
      <div v-if="item.spec.locked_updates"
        class="flex gap-1 items-center text-light-blue-400">
        <t-icon icon="time" class="h-4"/>
        {{ pluralize('Pending Config Update', item.spec.locked_updates, true) }}
      </div>
    </div>
</template>

<script setup lang="ts">
import TIcon, { IconType } from "@/components/common/Icon/TIcon.vue";
import { MachineSetPhase, MachineSetStatusSpec } from "@/api/omni/specs/omni.pb";
import { Resource } from "@/api/grpc";
import pluralize from 'pluralize';

const phaseName = (machineset: Resource<MachineSetStatusSpec>): string => {
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

const phaseIcon = (machineset: Resource<MachineSetStatusSpec>): IconType => {
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
  item: Resource<MachineSetStatusSpec>;
};

defineProps<Props>();

</script>

<style>
.cluster-phase-box {
  @apply flex items-center gap-1;
}
</style>
