<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">
        Cancel Destroy of Node {{ node ?? $route.query.machine }} ?
      </h3>
      <close-button @click="close(true)" />
    </div>
    <watch
      :opts="{ resource: { namespace: DefaultNamespace, id: $route.query.machine as string, type: ClusterMachineType }, runtime: Runtime.Omni }" spinner>
      <template #default="{ items }">
        <template v-if="canRestore(items)">
          <p class="text-xs mb-2">Please confirm the action.</p>

          <div class="flex items-end gap-4 mt-2">
            <div class="flex-1" />
            <t-button @click="() => restore(items[0])" class="h-9">
              Restore Machine
            </t-button>
          </div>
        </template>
        <template v-else>
          <p class="text-xs mb-2">Restoring the machine is not possible at this stage.</p>

          <div class="flex items-end gap-4 mt-2">
            <div class="flex-1" />
            <t-button @click="close" class="h-9">
              Close
            </t-button>
          </div>
        </template>
      </template>
    </watch>
  </div>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from "vue-router";
import { showError, showSuccess } from "@/notification";
import { setupNodenameWatch } from "@/methods/node";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import { Resource } from "@/api/grpc";
import {
  ClusterMachineType,
  DefaultNamespace,
} from "@/api/resources";
import { ClusterMachineSpec } from "@/api/omni/specs/omni.pb";
import { Runtime } from "@/api/common/omni.pb";
import Watch from "@/components/common/Watch/Watch.vue";
import { restoreNode } from "@/methods/cluster";

const router = useRouter();
const route = useRoute();

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

const canRestore = (items: Resource[]) => {
  return items.length === 0 || items[0].metadata.phase !== 'Running';
}

const node = setupNodenameWatch(route.query.machine as string);

const restore = async (clusterMachine: Resource<ClusterMachineSpec>) => {
  if (!route.query.machine) {
    showError(
      "Failed to Restore The Machine Set Node",
      "The machine id not resolved",
    )

    close(true);

    return;
  }

  try {
    await restoreNode(clusterMachine);
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

    showError("Failed to Restore The Node", e.message)

    return;
  }

  close();

  showSuccess(
    `The Machine ${node.value} was Restored`,
  );
}
</script>

<style scoped>
.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}
</style>
