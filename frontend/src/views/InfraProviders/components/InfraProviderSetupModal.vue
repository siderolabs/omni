<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import type { InfraProviderSpec } from '@/api/omni/specs/infra.pb'
import { withRuntime } from '@/api/options'
import { InfraProviderNamespace, ProviderType, RoleInfraProvider } from '@/api/resources'
import Modal from '@/components/Modals/Modal.vue'
import TInput from '@/components/TInput/TInput.vue'
import { createServiceAccount } from '@/methods/user'
import { showError } from '@/notification'
import ServiceAccountKey from '@/views/Users/components/ServiceAccountKey.vue'

const open = defineModel<boolean>('open', { default: false })

const key = ref<string>()
const providerId = ref('')
const isCreating = ref(false)

watchEffect(() => {
  if (open.value) return

  key.value = undefined
  providerId.value = ''
  isCreating.value = false
})

const onConfirm = async () => {
  isCreating.value = true

  try {
    await ResourceService.Create<Resource<InfraProviderSpec>>(
      {
        metadata: {
          namespace: InfraProviderNamespace,
          type: ProviderType,
          id: providerId.value,
        },
        spec: {},
      },
      withRuntime(Runtime.Omni),
    )

    key.value = await createServiceAccount(providerId.value, RoleInfraProvider)
  } catch (e) {
    showError('Failed to Create Service Account', e instanceof Error ? e.message : String(e))
  } finally {
    isCreating.value = false
  }
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Setup a new Infra Provider"
    :loading="isCreating"
    :cancel-label="key ? 'Close' : 'Cancel'"
    :action-label="key ? undefined : 'Next'"
    :action-disabled="!key && !providerId"
    content-class="max-w-md"
    @confirm="onConfirm"
  >
    <TInput
      v-if="!key"
      v-model.trim="providerId"
      title="Provider ID"
      placeholder="examples: kubevirt, bare-metal"
    />

    <ServiceAccountKey v-else :secret-key="key" />
  </Modal>
</template>
