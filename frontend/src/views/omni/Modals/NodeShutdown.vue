<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-wrapper" @click.self="close">
    <div class="modal">
      <div class="modal-heading">
        <h3 id="modal-title" class="modal-name">Shutdown the machine {{ node }} ?</h3>
        <t-icon class="modal-exit" icon="close" />
      </div>
      <p class="text-xs">Please confirm the action.</p>

      <div class="modal-buttons-box">
        <t-button @click="close" class="modal-button" type="secondary">Cancel</t-button>
        <t-button
          @click="shutdown"
          :disabled="!canRebootMachines || state === 'Shutdown in progress'"
          class="modal-button"
          >{{ state }}</t-button
        >
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import TButton from '@/components/common/Button/TButton.vue'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MachineService } from '@/api/talos/machine/machine.pb'
import { Runtime } from '@/api/common/omni.pb'
import { showError, showSuccess } from '@/notification'
import { getContext } from '@/context'
import { setupNodenameWatch } from '@/methods/node'
import { withContext, withRuntime } from '@/api/options'
import { setupClusterPermissions } from '@/methods/auth'

const route = useRoute()
const router = useRouter()
const state = ref('Shutdown')

const node = setupNodenameWatch(route.query.machine as string)

const close = () => {
  router.go(-1)
}

const context = getContext()

const { canRebootMachines } = setupClusterPermissions(computed(() => context.cluster ?? ''))

const shutdown = async () => {
  state.value = 'Shutdown in progress'

  const nodeName = node.value ?? (route.query.machine as string)

  try {
    const res = await MachineService.Shutdown({}, withRuntime(Runtime.Talos), withContext(context))

    const errors: string[] = []
    for (const message of res.messages || []) {
      if (message?.metadata?.error)
        errors.push(`${message.metadata.hostname || nodeName} ${message.metadata.error}`)
    }
    if (errors.length > 0) throw new Error(errors.join(', '))
  } catch (e: any) {
    close()

    showError('Failed to Issue Shutdown', e.toString())

    return
  }

  if (route.query.goback) {
    close()
  } else {
    await router.push({ name: 'ClusterOverview', params: { cluster: context.cluster } })
  }

  showSuccess('Machine Shutdown', `Machine ${nodeName} is shutting down now.`)
}
</script>

<style scoped>
.modal {
  @apply rounded bg-naturals-N3 p-8 z-30;
  width: 500px;
}
.modal-wrapper {
  @apply fixed top-0 bottom-0 left-0 right-0 w-full h-full flex justify-center items-center  z-30;
  background-color: rgba(16, 17, 24, 0.5);
}
.modal-heading {
  @apply flex justify-between items-center;
  margin-bottom: 13px;
}
.modal-name {
  @apply text-base text-naturals-N14;
}
.modal-subtitle {
  @apply text-xs;
  margin-bottom: 19px;
}
.modal-subtitle-light {
  @apply text-xs text-naturals-N13;
}
.modal-exit {
  @apply fill-current text-naturals-N7 cursor-pointer transition-colors hover:text-naturals-N8 w-6 h-6;
}
.modal-buttons-box {
  @apply flex justify-end w-full;
}
.modal-button:nth-child(1) {
  @apply mr-4;
}
</style>
