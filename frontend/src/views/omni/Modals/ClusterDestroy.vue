<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import type { Ref } from 'vue'
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import { withRuntime } from '@/api/options'
import {
  DefaultNamespace,
  LabelCluster,
  MachineStatusLabelDisconnected,
  MachineStatusType,
  SiderolinkResourceType,
} from '@/api/resources'
import Watch from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import { clusterDestroy } from '@/methods/cluster'
import { showError, showSuccess } from '@/notification'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const router = useRouter()
const route = useRoute()
const phase = ref('')
let closed = false

const disconnectedMachines: Ref<Resource[]> = ref([])

const close = () => {
  if (closed) {
    return
  }

  closed = true

  router.go(-1)
}

const machinesWatch = new Watch(disconnectedMachines)
const loading = machinesWatch.loading

machinesWatch.setup({
  resource: {
    namespace: DefaultNamespace,
    type: MachineStatusType,
  },
  selectors: [MachineStatusLabelDisconnected, `${LabelCluster}=${route.query.cluster as string}`],
  runtime: Runtime.Omni,
})

const destroyCluster = async () => {
  destroying.value = true

  // copy machines to avoid updates coming from the watch
  const machines = disconnectedMachines.value.slice(0, disconnectedMachines.value.length)

  for (const machine of machines) {
    phase.value = `Tearing down disconnected machine ${machine.metadata.id}`

    try {
      await ResourceService.Teardown(
        {
          namespace: DefaultNamespace,
          type: SiderolinkResourceType,
          id: machine.metadata.id!,
        },
        withRuntime(Runtime.Omni),
      )
    } catch (e) {
      close()

      if (e.code !== Code.NOT_FOUND) {
        showError('Failed to Destroy the Cluster', e.message)
      }

      return
    }
  }

  try {
    await clusterDestroy(route.query.cluster as string)
  } catch (e) {
    close()

    if (e.errorNotification) {
      showError(e.errorNotification.title, e.errorNotification.details)

      return
    }

    showError('Failed to Destroy the Cluster', e.message)

    return
  }

  for (const machine of machines) {
    phase.value = `Remove disconnected machine ${machine.metadata.id}`

    try {
      await ResourceService.Delete(
        {
          namespace: DefaultNamespace,
          type: SiderolinkResourceType,
          id: machine.metadata.id!,
        },
        withRuntime(Runtime.Omni),
      )
    } catch (e) {
      close()

      if (e.code !== Code.NOT_FOUND) {
        showError('Failed to Destroy the Cluster', e.message)
      }

      return
    }
  }

  destroying.value = false

  close()

  showSuccess(`The Cluster ${route.query.cluster} is Tearing Down`)
}

const destroying = ref(false)
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Destroy the Cluster {{ $route.query.cluster }} ?</h3>
      <CloseButton @click="close" />
    </div>
    <ManagedByTemplatesWarning warning-style="popup" />
    <p v-if="destroying" class="text-xs">{{ phase }}...</p>
    <p v-else-if="loading" class="text-xs">Checking the cluster status...</p>
    <div v-else-if="disconnectedMachines.length > 0" class="text-xs">
      <p class="py-2 text-primary-p3">
        Cluster <code>{{ $route.query.cluster }}</code> has
        {{ disconnectedMachines.length }} disconnected
        {{ pluralize('machine', disconnectedMachines.length, false) }}. Destroying the cluster now
        will also destroy disconnected machines.
      </p>
      <p class="py-2 font-bold text-primary-p3">
        These machines will need to be wiped and reinstalled to be used with Omni again. If the
        machines can be recovered, you may wish to recover them before destroying the cluster, to
        allow a graceful reset of the machines.
      </p>
    </div>
    <p v-else class="text-xs">Please confirm the action.</p>

    <div class="mt-8 flex justify-end gap-4">
      <TButton :disabled="destroying || loading" class="h-9 w-32" @click="destroyCluster">
        <TSpinner v-if="destroying" class="h-5 w-5" />
        <span v-else> Destroy </span>
      </TButton>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.window {
  @apply z-30 flex w-1/3 flex-col rounded bg-naturals-n2 p-8;
}

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-n14;
}
</style>
