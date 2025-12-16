<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import { ResourceService } from '@/api/grpc'
import { type KernelArgsSpec, type KernelArgsStatusSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, KernelArgsStatusType, KernelArgsType } from '@/api/resources'
import IconButton from '@/components/common/Button/IconButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { useResourceGet } from '@/methods/useResourceGet.ts'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const args = ref('')
const route = useRoute()
const router = useRouter()

const { data: status } = useResourceWatch<KernelArgsStatusSpec>({
  runtime: Runtime.Omni,
  resource: {
    id: route.query.machine as string,
    namespace: DefaultNamespace,
    type: KernelArgsStatusType,
  },
})

const currentArgs = computed(() => {
  return status.value?.spec.current_args?.join(' ') || ''
})
const currentCmdline = computed(() => {
  return status.value?.spec.current_cmdline || ''
})
const unmetConditions = computed(() => {
  return status.value?.spec.unmet_conditions || []
})

const md = {
  id: route.query.machine as string,
  namespace: DefaultNamespace,
  type: KernelArgsType,
}

const { data: kernelArgs } = useResourceGet<KernelArgsSpec>({
  runtime: Runtime.Omni,
  resource: md,
})

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
      await ResourceService.Delete(md, withRuntime(Runtime.Omni))
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
        metadata: md,
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

let closed = false

const close = () => {
  if (closed) {
    return
  }

  closed = true

  router.go(-1)
}
</script>

<template>
  <div class="modal-window flex flex-col gap-2">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">
        Update Kernel Args of Machine {{ route.query.machine }}
      </h3>
      <CloseButton @click="close" />
    </div>

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
    <code>{{ currentCmdline || 'none' }}</code>
    <template v-if="!editArgs">
      <div class="text-sm font-semibold text-naturals-n14">Current Kernel Args</div>
      <div class="my-0.5 flex items-center gap-2 rounded bg-naturals-n6 pr-2">
        <code class="flex-1">
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
        <TButton type="highlighted" class="h-9 w-32" @click="handleUpdateKernelArgs">
          Update
        </TButton>
      </div>
    </template>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.modal-window {
  @apply w-3/4;
}

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-n14;
}

code {
  @apply rounded bg-naturals-n6 px-1 px-2.5 py-0.5 py-2 font-mono text-xs text-naturals-n13;
}
</style>
