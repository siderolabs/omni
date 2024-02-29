<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div
    v-bind:style="'color: ' + stageColor(machine)"
    v-bind:class="'cluster-stage-box' + (connected(machine) ? '' : ' brightness-50')"
  >
    <popper v-if="!connected(machine)" hover placement="right" offsetDistance="4">
      <template #content>
        <div class="px-2 py-1 rounded bg-naturals-N4 drop-shadow text-naturals-N12">
          The machine is unreachable. The last known state is shown.
        </div>
      </template>
      <t-icon class="w-4 h-4" icon="unknown"/>
    </popper>
    <t-icon v-else :icon="stageIcon(machine)" class="h-4" />
    <div class="cluster-stage-name">
      {{ stageName(machine) || "" }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ClusterMachineStatusSpecStage, ClusterMachineStatusSpec } from "@/api/omni/specs/omni.pb";
import { MachineStatusLabelConnected } from "@/api/resources";
import { ResourceTyped } from "@/api/grpc";

import TIcon, { IconType } from "@/components/common/Icon/TIcon.vue";
import Popper from "vue3-popper";

const connected = (machine: ResourceTyped<ClusterMachineStatusSpec>): boolean => {
  return machine?.metadata.labels?.[MachineStatusLabelConnected] === "";
};

const stageName = (machine: ResourceTyped<ClusterMachineStatusSpec>): string => {
  switch (machine?.spec.stage) {
    case ClusterMachineStatusSpecStage.BOOTING:
      return "Booting";
    case ClusterMachineStatusSpecStage.INSTALLING:
      return "Installing";
    case ClusterMachineStatusSpecStage.UPGRADING:
      return "Upgrading";
    case ClusterMachineStatusSpecStage.CONFIGURING:
      return "Configuring";
    case ClusterMachineStatusSpecStage.RUNNING:
      if (machine?.spec.ready || !connected(machine)) {
        return "Running";
      } else {
        return "Not Ready";
      }
    case ClusterMachineStatusSpecStage.REBOOTING:
      return "Rebooting";
    case ClusterMachineStatusSpecStage.SHUTTING_DOWN:
      return "Shutting Down";
    case ClusterMachineStatusSpecStage.DESTROYING:
      return "Destroying";
    case ClusterMachineStatusSpecStage.BEFORE_DESTROY:
      return "Preparing to Destroy"
    default:
      return "Unknown";
  }
};

const stageIcon = (machine: ResourceTyped<ClusterMachineStatusSpec>): IconType => {
  switch (machine?.spec.stage) {
    case ClusterMachineStatusSpecStage.BOOTING:
    case ClusterMachineStatusSpecStage.INSTALLING:
    case ClusterMachineStatusSpecStage.UPGRADING:
    case ClusterMachineStatusSpecStage.CONFIGURING:
    case ClusterMachineStatusSpecStage.REBOOTING:
    case ClusterMachineStatusSpecStage.SHUTTING_DOWN:
      return "loading";
    case ClusterMachineStatusSpecStage.RUNNING:
    if (machine?.spec.ready) {
        return "check-in-circle";
      } else {
        return "error";
      }
    case ClusterMachineStatusSpecStage.BEFORE_DESTROY:
      return "loading";
    case ClusterMachineStatusSpecStage.DESTROYING:
      return "delete";
    default:
      return "unknown";
  }
};

const stageColor = (machine: ResourceTyped<ClusterMachineStatusSpec>): string => {
  const Y1 = "#FFB200";

  switch (machine?.spec.stage) {
    case ClusterMachineStatusSpecStage.BOOTING:
    case ClusterMachineStatusSpecStage.INSTALLING:
    case ClusterMachineStatusSpecStage.UPGRADING:
    case ClusterMachineStatusSpecStage.CONFIGURING:
    case ClusterMachineStatusSpecStage.REBOOTING:
      return Y1;
    case ClusterMachineStatusSpecStage.RUNNING:
      if (machine?.spec.ready || !connected(machine)) {
        return "#69C297";
      } else {
        return "#FF5F2A";
      }
    case ClusterMachineStatusSpecStage.SHUTTING_DOWN:
    case ClusterMachineStatusSpecStage.BEFORE_DESTROY:
    case ClusterMachineStatusSpecStage.DESTROYING:
      return "#FF5F2A";
    default:
      return Y1;
  }
};

type Props = {
  machine: ResourceTyped<ClusterMachineStatusSpec>;
};

defineProps<Props>();

</script>

<style>
.cluster-stage-box {
  display: flex;
  align-items: center;
}

.cluster-stage-name {
  margin-left: 5px;
}
</style>
