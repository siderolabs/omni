<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { withContext, withRuntime } from '@/api/options'
import { MachineService } from '@/api/talos/machine/machine.pb'
import TButton from '@/components/common/Button/TButton.vue'
import { getContext } from '@/context'
import { setupClusterPermissions } from '@/methods/auth'
import { setupNodenameWatch } from '@/methods/node'
import { showError, showSuccess } from '@/notification'

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

<template>
  <div class="modal-wrapper" @click.self="close">
    <div class="modal">
      <div class="modal-heading">
        <h3 id="modal-title" class="modal-name">Shutdown the machine {{ node }} ?</h3>
        <TIcon class="modal-exit" icon="close" />
      </div>
      <p class="text-xs">Please confirm the action.</p>

      <div class="modal-buttons-box">
        <TButton class="modal-button" type="secondary" @click="close">Cancel</TButton>
        <TButton
          :disabled="!canRebootMachines || state === 'Shutdown in progress'"
          class="modal-button"
          @click="shutdown"
        >
          {{ state }}
        </TButton>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.modal {
  @apply z-30 rounded bg-naturals-n3 p-8;
  width: 500px;
}
.modal-wrapper {
  @apply fixed top-0 right-0 bottom-0 left-0 z-30 flex h-full w-full items-center justify-center;
  background-color: rgba(16, 17, 24, 0.5);
}
.modal-heading {
  @apply flex items-center justify-between;
  margin-bottom: 13px;
}
.modal-name {
  @apply text-base text-naturals-n14;
}
.modal-subtitle {
  @apply text-xs;
  margin-bottom: 19px;
}
.modal-subtitle-light {
  @apply text-xs text-naturals-n13;
}
.modal-exit {
  @apply h-6 w-6 cursor-pointer fill-current text-naturals-n7 transition-colors hover:text-naturals-n8;
}
.modal-buttons-box {
  @apply flex w-full justify-end;
}
.modal-button:nth-child(1) {
  @apply mr-4;
}
</style>
