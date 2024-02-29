<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">
        Destroy the Node {{ node ?? $route.query.machine }} ?
      </h3>
      <close-button @click="close(true)"/>
    </div>
    <managed-by-templates-warning warning-style="popup"/>
    <p class="text-xs mb-2">Please confirm the action.</p>
    <div v-if="warning" class="text-yellow-Y1 text-xs mt-3">{{ warning }}</div>

    <div class="flex items-end gap-4 mt-2">
      <div class="flex-1"/>
      <t-button @click="destroyNode" class="w-32 h-9" :disabled="scalingDown">
        <t-spinner v-if="scalingDown" class="w-5 h-5"/>
        <span v-else>Destroy</span>
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, Ref, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { showError, showSuccess } from "@/notification";
import { destroyNodes } from "@/methods/cluster";
import { controlPlaneMachineSetId } from "@/methods/machineset";
import { setupNodenameWatch } from "@/methods/node";
import pluralize from "pluralize";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import { ResourceService, ResourceTyped } from "@/api/grpc";
import {
  ClusterMachineStatusType,
  DefaultNamespace, LabelCluster, LabelControlPlaneRole,
  LabelMachineSet,
  MachineSetNodeType
} from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import { ClusterMachineStatusSpec } from "@/api/omni/specs/omni.pb";
import { withRuntime, withSelectors } from "@/api/options";
import ManagedByTemplatesWarning from "@/views/cluster/ManagedByTemplatesWarning.vue";

const router = useRouter();
const route = useRoute();
const scalingDown = ref(false);
const warning: Ref<string | null> = ref(null);

let closed = false;

const close = (goBack?: boolean) => {
  if (closed) {
    return;
  }

  if (!goBack && !route.query.goback) {
    router.push({ name: 'ClusterOverview', params: { cluster: route.query.cluster as string } });

    return;
  }

  closed = true;

  router.go(-1);
};

onMounted(async () => {
  let controlPlanes = 0;
  let isControlPlane = true;

  try {
    const clusterMachineStatus: ResourceTyped<ClusterMachineStatusSpec> = await ResourceService.Get({
      type: ClusterMachineStatusType,
      namespace: DefaultNamespace,
      id: route.query.machine as string,
    }, withRuntime(Runtime.Omni));

    isControlPlane = clusterMachineStatus?.metadata?.labels?.[LabelControlPlaneRole] === "";

    // since this code called for both worker and control plane nodes, we need to check if the node is control plane node
    const controlPlaneMachineStatus: ResourceTyped<ClusterMachineStatusSpec> = await ResourceService.Get({
      type: ClusterMachineStatusType,
      namespace: DefaultNamespace,
      id: route.query.machine as string,
    },
      withRuntime(Runtime.Omni),
      withSelectors([
        `${LabelCluster}=${route.query.cluster as string}`,
        `${LabelControlPlaneRole}`,
      ]),
    );

    if (!controlPlaneMachineStatus) {
      warning.value = null;
      return;
    }

    const resp = await ResourceService.List({
      namespace: DefaultNamespace,
      type: MachineSetNodeType,
    },
      withRuntime(Runtime.Omni),
      withSelectors([`${LabelMachineSet}=${controlPlaneMachineSetId(route.params.cluster as string)}`]),
    );

    controlPlanes = resp.length
  } catch (e) {
    close(true);

    showError("Failed to Destroy The Node", e.message);

    return;
  }

  if (!isControlPlane || controlPlanes % 2 === 0 || controlPlanes === 1) {
    warning.value = null;

    return;
  }

  warning.value = `${pluralize("Control Plane", controlPlanes - 1, true)} will not provide fault-tolerance with etcd quorum requirements. Removing this machine will result in an even number of control plane nodes, which reduces the fault tolerance of the etcd cluster.`;
})

const node = setupNodenameWatch(route.query.machine as string);

const destroyNode = async () => {
  scalingDown.value = true;

  if (!route.query.machine) {
    showError(
      "Failed to Destroy The Machine",
      "The machine id not resolved",
    )

    close(true);

    return;
  }

  try {
    await destroyNodes(route.query.cluster as string, [route.query.machine as string]);
  } catch (e) {
    if (e.errorNotification) {
      showError(
        e.errorNotification.title,
        e.errorNotification.details,
      )

      close(true);

      return;
    }

    close(true);

    showError("Failed to Destroy The Node", e.message)

    return;
  } finally {
    scalingDown.value = false;
  }

  close();

  showSuccess(
    `The Machine ${node.value} was Removed From the Machine Set`,
    "The machine set is scaling down",
  );
}
</script>

<style scoped>
.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}
</style>
