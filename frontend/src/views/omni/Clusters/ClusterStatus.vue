<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
    <div
      v-bind:style="'color: ' + phaseColor(cluster)"
      class="cluster-status-box"
    >
      <t-icon v-bind:icon="phaseIcon(cluster)" class="h-4" />
      <div class="cluster-status-name">{{ phaseName(cluster) || "" }}</div>
    </div>
</template>

<script setup lang="ts">
import TIcon from "@/components/common/Icon/TIcon.vue";
import { ClusterStatusSpec, ClusterStatusSpecPhase } from "@/api/omni/specs/omni.pb";
import { Resource } from "@/api/grpc";

type Props = {
  cluster?: Resource<ClusterStatusSpec>;
};

defineProps<Props>();

const phaseName = (cluster?: Resource<ClusterStatusSpec>): string => {
  switch (cluster?.spec.phase) {
    case ClusterStatusSpecPhase.SCALING_UP:
      return "Scaling Up";
    case ClusterStatusSpecPhase.SCALING_DOWN:
      return "Scaling Down";
    case ClusterStatusSpecPhase.RUNNING:
      if (cluster?.spec.ready) {
        return "Running";
      } else {
        return "Not Ready";
      }
    case ClusterStatusSpecPhase.DESTROYING:
      return "Destroying";
    default:
      return "Unknown";
  }
};

const phaseIcon = (cluster?: Resource<ClusterStatusSpec>): string => {
  switch (cluster?.spec.phase) {
    case ClusterStatusSpecPhase.SCALING_UP:
    case ClusterStatusSpecPhase.SCALING_DOWN:
      return "loading";
    case ClusterStatusSpecPhase.RUNNING:
      if (cluster?.spec.ready) {
        return "check-in-circle";
      } else {
        return "error";
      }
    case ClusterStatusSpecPhase.DESTROYING:
      return "delete";
    default:
      return "unknown";
  }
};

const phaseColor = (cluster?: Resource<ClusterStatusSpec>): string => {
  const Y1 = "#FFB200";

  switch (cluster?.spec.phase) {
    case ClusterStatusSpecPhase.SCALING_UP:
    case ClusterStatusSpecPhase.SCALING_DOWN:
      return Y1;
    case ClusterStatusSpecPhase.RUNNING:
      if (cluster?.spec.ready) {
        return "#69C297";
      } else {
        return "#FF5F2A";
      }
    case ClusterStatusSpecPhase.DESTROYING:
      return "#FF5F2A";
    default:
      return Y1;
  }
};

</script>

<style>
  .cluster-status-box {
    display: flex;
    align-items: center;
  }

  .cluster-status-name {
    margin-left: 5px;
  }
</style>
