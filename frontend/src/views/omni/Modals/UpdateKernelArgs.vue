<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import { type KernelArgsSpec, type KernelArgsStatusSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, KernelArgsStatusType, KernelArgsType } from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { useWatch } from '@/components/common/Watch/useWatch'
import TAlert from '@/components/TAlert.vue'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const args = ref('')
const route = useRoute()
const router = useRouter()

const { data: status } = useWatch<KernelArgsStatusSpec>({
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

let kernelArgs: Resource<KernelArgsSpec> | undefined = undefined

onMounted(async () => {
  try {
    kernelArgs = await ResourceService.Get<Resource<KernelArgsSpec>>(md, withRuntime(Runtime.Omni))

    args.value = (kernelArgs.spec.args || []).map((arg) => arg.trim()).join(' ')
  } catch (e) {
    if (e.code !== Code.NOT_FOUND) {
      throw e
    }
  }
})

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

  if (!kernelArgs && emptyArgs) {
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

  if (!kernelArgs) {
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

  kernelArgs.spec.args = argsSplit

  await ResourceService.Update(kernelArgs, kernelArgs.metadata.version, withRuntime(Runtime.Omni))
}

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
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">
        Update Kernel Args of Machine {{ route.query.machine }}
      </h3>
      <CloseButton @click="close" />
    </div>

    <TAlert
      v-if="unmetConditions.length > 0"
      type="warn"
      title="Kernel args won't be updated, there are unmet conditions:"
    >
      <span v-for="(condition, index) in unmetConditions" :key="index" class="py-2 text-primary-p3">
        {{ condition }}
      </span>
    </TAlert>

    <div class="mb-5">
      <TAlert type="info" title="Current Kernel Args">
        <p class="font-mono break-words whitespace-pre-wrap">{{ currentArgs }}</p>
      </TAlert>
      <TAlert type="info" title="Current Kernel Cmdline">
        <p class="font-mono break-words whitespace-pre-wrap">{{ currentCmdline }}</p>
      </TAlert>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <TInput v-model="args" title="Extra Kernel Args" class="h-full flex-1" placeholder="..." />
      <TButton type="highlighted" class="h-9 w-32" @click="handleUpdateKernelArgs">Update</TButton>
    </div>
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
</style>
