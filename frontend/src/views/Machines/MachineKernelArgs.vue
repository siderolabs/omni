<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import { ResourceService } from '@/api/grpc'
import { type KernelArgsSpec, type KernelArgsStatusSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, KernelArgsStatusType, KernelArgsType } from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import TButton from '@/components/Button/TButton.vue'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TInput from '@/components/TInput/TInput.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'

const { machine } = defineProps<{
  machine: string
}>()

const args = ref('')

const { data: status } = useResourceWatch<KernelArgsStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    id: machine,
    namespace: DefaultNamespace,
    type: KernelArgsStatusType,
  },
}))

const currentArgs = computed(() => {
  return status.value?.spec.current_args?.join(' ') || ''
})
const currentCmdline = computed(() => {
  return status.value?.spec.current_cmdline || ''
})
const unmetConditions = computed(() => {
  return status.value?.spec.unmet_conditions || []
})

const { data: kernelArgs } = useResourceWatch<KernelArgsSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    id: machine,
    namespace: DefaultNamespace,
    type: KernelArgsType,
  },
}))

const initialArgs = computed(
  () => kernelArgs.value?.spec.args?.map((arg) => arg.trim()).join(' ') ?? '',
)

watch(initialArgs, () => (args.value = initialArgs.value))

const handleUpdateKernelArgs = async () => {
  try {
    await updateKernelArgs()
  } catch (e) {
    showError(`Failed to Update Kernel Args`, e.message)

    return
  }

  editArgs.value = false
  showSuccess(`Kernel args are updated`)

  close()
}

const updateKernelArgs = async () => {
  const argsSplit = args.value
    .trim()
    .split(' ')
    .filter((arg) => arg.trim() !== '')
  const emptyArgs = argsSplit.length === 0

  if (!kernelArgs.value && emptyArgs) {
    return
  }

  if (emptyArgs) {
    try {
      await ResourceService.Delete(
        {
          id: machine,
          namespace: DefaultNamespace,
          type: KernelArgsType,
        },
        withRuntime(Runtime.Omni),
      )
    } catch (e) {
      if (e.code === Code.NOT_FOUND) {
        return
      }

      throw e
    }

    return
  }

  if (!kernelArgs.value) {
    await ResourceService.Create(
      {
        metadata: {
          id: machine,
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

  kernelArgs.value.spec.args = argsSplit

  await ResourceService.Update(
    kernelArgs.value,
    kernelArgs.value.metadata.version,
    withRuntime(Runtime.Omni),
  )
}

const editArgs = ref(false)
</script>

<template>
  <PageContainer class="flex flex-col gap-2">
    <h3 class="text-base text-naturals-n14">Update Kernel Args of Machine {{ machine }}</h3>

    <ManagedByTemplatesWarning :resource="kernelArgs" />

    <template v-if="unmetConditions.length > 0">
      <div class="font-bold text-primary-p3">
        Kernel args won't be updated immediately, there are unmet conditions
      </div>
      <div class="flex flex-col">
        <span
          v-for="(condition, index) in unmetConditions"
          :key="index"
          class="my-0.5 rounded border-l-3 border-l-yellow-y1 bg-naturals-n5 py-1 pl-3 text-xs text-naturals-n13"
        >
          {{ condition }}
        </span>
      </div>
    </template>

    <div class="text-sm font-semibold text-naturals-n14">Current Kernel Cmdline</div>
    <code class="rounded bg-naturals-n6 px-2.5 py-2 font-mono text-xs text-naturals-n13">
      {{ currentCmdline || 'none' }}
    </code>
    <template v-if="!editArgs">
      <div class="text-sm font-semibold text-naturals-n14">Current Kernel Args</div>
      <div class="my-0.5 flex items-center gap-2 rounded bg-naturals-n6 pr-2">
        <code class="flex-1 rounded bg-naturals-n6 px-2.5 py-2 font-mono text-xs text-naturals-n13">
          {{ currentArgs || 'none' }}
        </code>
        <IconButton
          v-if="currentArgs === initialArgs"
          icon="edit"
          @click="() => (editArgs = true)"
        />
      </div>
    </template>

    <template v-if="currentArgs !== initialArgs || editArgs">
      <div class="text-sm font-semibold text-naturals-n14">New Kernel Args</div>
      <div class="flex flex-wrap items-center gap-2">
        <TInput v-model="args" class="h-full flex-1 font-mono" placeholder="none" />
        <TButton v-if="editArgs" class="h-9 w-32" @click="() => (editArgs = false)">Cancel</TButton>
        <TButton variant="highlighted" class="h-9 w-32" @click="handleUpdateKernelArgs">
          Update
        </TButton>
      </div>
    </template>
  </PageContainer>
</template>
