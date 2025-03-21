<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">
        Destroy the Cluster {{ $route.query.cluster }} ?
      </h3>
      <close-button @click="close"/>
    </div>
    <managed-by-templates-warning warning-style="popup"/>
    <p class="text-xs" v-if="destroying">{{ phase }}...</p>
    <p class="text-xs" v-else-if="loading">Checking the cluster status...</p>
    <div v-else-if="disconnectedMachines.length > 0" class="text-xs">
      <p class="text-primary-P3 py-2">
        Cluster <code>{{ $route.query.cluster }}</code> has {{ disconnectedMachines.length }} disconnected {{ pluralize('machine', disconnectedMachines.length, false) }}.
        Destroying the cluster now will also destroy disconnected machines.
      </p>
      <p class="text-primary-P3 py-2 font-bold">
        These machines will need to be wiped and reinstalled to be used with Omni again.
        If the machines can be recovered, you may wish to recover them before destroying the cluster, to allow a graceful reset of the machines.
      </p>
    </div>
    <p v-else class="text-xs">Please confirm the action.</p>

    <div class="flex justify-end gap-4 mt-8">
      <t-button @click="destroyCluster" :disabled="destroying || loading" class="w-32 h-9">
        <t-spinner v-if="destroying" class="w-5 h-5" />
        <span v-else>
          Destroy
        </span>
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, Ref } from 'vue';
import { useRoute, useRouter } from "vue-router";
import { showError, showSuccess } from "@/notification";
import pluralize from "pluralize";
import { Resource, ResourceService } from "@/api/grpc";
import Watch from "@/api/watch";
import { Runtime } from "@/api/common/omni.pb";
import { MachineStatusLabelDisconnected, MachineStatusType, DefaultNamespace, LabelCluster, SiderolinkResourceType } from "@/api/resources";

import CloseButton from "@/views/omni/Modals/CloseButton.vue";
import TButton from "@/components/common/Button/TButton.vue";
import TSpinner from "@/components/common/Spinner/TSpinner.vue";
import { clusterDestroy } from "@/methods/cluster";
import ManagedByTemplatesWarning from "@/views/cluster/ManagedByTemplatesWarning.vue";
import { withRuntime } from '@/api/options';
import { Code } from '@/api/google/rpc/code.pb';

const router = useRouter();
const route = useRoute();
const phase = ref("");
let closed = false;

const disconnectedMachines: Ref<Resource[]> = ref([]);

const close = () => {
  if (closed) {
    return;
  }

  closed = true;

  router.go(-1);
};

const machinesWatch = new Watch(disconnectedMachines);
const loading = machinesWatch.loading;

machinesWatch.setup({
  resource: {
    namespace: DefaultNamespace,
    type: MachineStatusType,
  },
  selectors: [
    MachineStatusLabelDisconnected,
    `${LabelCluster}=${route.query.cluster as string}`,
  ],
  runtime: Runtime.Omni
});

const destroyCluster = async () => {
  destroying.value = true;

  // copy machines to avoid updates coming from the watch
  const machines = disconnectedMachines.value.slice(0, disconnectedMachines.value.length);

  for (const machine of machines) {
    phase.value = `Tearing down disconnected machine ${machine.metadata.id}`;

    try {
      await ResourceService.Teardown({
        namespace: DefaultNamespace,
        type: SiderolinkResourceType,
        id: machine.metadata.id!,
      }, withRuntime(Runtime.Omni));
    } catch (e) {
      close();

      if (e.code !== Code.NOT_FOUND) {
        showError("Failed to Destroy the Cluster", e.message)
      }

      return;
    }
  }

  try {
    await clusterDestroy(route.query.cluster as string)
  } catch (e) {
    close();

    if (e.errorNotification) {
      showError(
        e.errorNotification.title,
        e.errorNotification.details,
      )

      return;
    }

    showError("Failed to Destroy the Cluster", e.message);

    return;
  }

  for (const machine of machines) {
    phase.value = `Remove disconnected machine ${machine.metadata.id}`;

    try {
      await ResourceService.Delete({
        namespace: DefaultNamespace,
        type: SiderolinkResourceType,
        id: machine.metadata.id!,
      }, withRuntime(Runtime.Omni));
    } catch (e) {
      close();

      if (e.code !== Code.NOT_FOUND) {
        showError("Failed to Destroy the Cluster", e.message)
      }

      return;
    }
  }

  destroying.value = false;

  close();

  showSuccess(
    `The Cluster ${route.query.cluster} is Tearing Down`,
  );
}

const destroying = ref(false);
</script>

<style scoped>
.window {
  @apply rounded bg-naturals-N2 z-30 w-1/3 flex flex-col p-8;
}

.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}
</style>
