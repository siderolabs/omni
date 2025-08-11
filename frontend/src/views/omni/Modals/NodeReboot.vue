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
const state = ref('Reboot')

const close = () => {
  router.go(-1)
}

const node = setupNodenameWatch(route.query.machine as string)
const context = getContext()

const { canRebootMachines } = setupClusterPermissions(computed(() => context.cluster ?? ''))

const reboot = async () => {
  state.value = 'Rebooting'
  const nodeName = node.value ?? (route.query.machine as string)

  try {
    const res = await MachineService.Reboot({}, withRuntime(Runtime.Talos), withContext(context))

    const errors: string[] = []
    for (const message of res.messages || []) {
      if (message?.metadata?.error)
        errors.push(`${message.metadata.hostname || nodeName} ${message.metadata.error}`)
    }

    if (errors.length > 0) throw new Error(errors.join(', '))
  } catch (e: any) {
    close()

    showError('Failed to Issue Reboot', e.toString())

    return
  }

  if (route.query.goback) {
    close()
  } else {
    await router.push({ name: 'ClusterOverview', params: { cluster: route.params.cluster } })
  }

  showSuccess('Machine Reboot', `Machine ${nodeName} is rebooting now.`)
}
</script>

<template>
  <div class="modal-wrapper" @click.self="close">
    <div class="modal">
      <div class="modal-heading">
        <h3 id="modal-title" class="modal-name">Reboot the machine {{ node }} ?</h3>
        <t-icon class="modal-exit" icon="close" />
      </div>
      <p class="text-xs">Please confirm the action.</p>

      <div class="modal-buttons-box">
        <TButton class="modal-button" type="secondary" @click="close">Cancel</TButton>
        <TButton
          :disabled="!canRebootMachines || state === 'Rebooting'"
          class="modal-button"
          @click="reboot"
          >{{ state }}</TButton
        >
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal {
  @apply z-30 rounded bg-naturals-N3 p-8;
  width: 390px;
}
.modal-wrapper {
  @apply fixed bottom-0 left-0 right-0 top-0 z-30 flex h-full w-full items-center justify-center;
  background-color: rgba(16, 17, 24, 0.5);
}
.modal-heading {
  @apply flex items-center justify-between;
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
  @apply h-6 w-6 cursor-pointer fill-current text-naturals-N7 transition-colors hover:text-naturals-N8;
}
.modal-buttons-box {
  @apply flex w-full justify-end;
}
.modal-button:nth-child(1) {
  @apply mr-4;
}
</style>
