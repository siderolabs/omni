<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import type { KernelArgsSpec, KernelArgsStatusSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, KernelArgsType } from '@/api/resources'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import Modal from '@/components/Modals/Modal.vue'
import TInput from '@/components/TInput/TInput.vue'
import { showError, showSuccess } from '@/notification'

const { machineId, kernelArgs, kernelArgsStatus } = defineProps<{
  machineId: string
  kernelArgs?: Resource<KernelArgsSpec>
  kernelArgsStatus?: Resource<KernelArgsStatusSpec>
}>()

const unmetConditions = computed(() => kernelArgsStatus?.spec.unmet_conditions)
const initialArgs = computed(() => kernelArgs?.spec.args?.map((arg) => arg.trim()).join(' ') ?? '')

const open = defineModel<boolean>('open', { default: false })
const args = ref('')

watch([open, initialArgs], () => (args.value = initialArgs.value))

async function confirm() {
  try {
    await updateKernelArgs()
  } catch (e) {
    showError('Failed to Update Kernel Args', e.message)
    return
  }

  open.value = false
  showSuccess('Kernel args are updated')
}

async function updateKernelArgs() {
  const argsSplit = args.value
    .trim()
    .split(' ')
    .filter((arg) => arg.trim() !== '')

  const emptyArgs = argsSplit.length === 0

  if (!kernelArgs && emptyArgs) return

  if (emptyArgs) {
    try {
      await ResourceService.Delete(
        {
          id: machineId,
          namespace: DefaultNamespace,
          type: KernelArgsType,
        },
        withRuntime(Runtime.Omni),
      )
    } catch (e) {
      if (e.code === Code.NOT_FOUND) return

      throw e
    }

    return
  }

  if (!kernelArgs) {
    await ResourceService.Create(
      {
        metadata: {
          id: machineId,
          namespace: DefaultNamespace,
          type: KernelArgsType,
        },
        spec: {
          args: argsSplit,
        },
      },
      withRuntime(Runtime.Omni),
    )

    return
  }

  await ResourceService.Update(
    {
      ...kernelArgs,
      spec: {
        ...kernelArgs.spec,
        args: argsSplit,
      },
    },
    kernelArgs.metadata.version,
    withRuntime(Runtime.Omni),
  )
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Edit Extra Kernel Args"
    action-label="Save"
    :action-disabled="args === initialArgs"
    @confirm="confirm"
  >
    <ManagedByTemplatesWarning :resource="kernelArgs" />

    <div class="flex flex-col gap-6">
      <div v-if="unmetConditions?.length" class="flex flex-col gap-2">
        <div class="font-bold text-primary-p3">
          Kernel args won't be updated immediately, there are unmet conditions
        </div>

        <div class="flex flex-col gap-1">
          <span
            v-for="(condition, index) in unmetConditions"
            :key="index"
            class="my-0.5 rounded border-l-3 border-l-yellow-y1 bg-naturals-n5 py-1 pl-3 text-xs text-naturals-n13"
          >
            {{ condition }}
          </span>
        </div>
      </div>

      <TInput
        v-model="args"
        class="max-w-full min-w-70 font-mono sm:min-w-80 md:min-w-100 [&_input]:field-sizing-content"
        placeholder="none"
      />
    </div>
  </Modal>
</template>
