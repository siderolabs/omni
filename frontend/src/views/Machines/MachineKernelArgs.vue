<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { type KernelArgsSpec, type KernelArgsStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, KernelArgsStatusType, KernelArgsType } from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import EditKernelArgsModal from '@/views/Machines/EditKernelArgsModal.vue'

const { machine } = defineProps<{
  machine: string
}>()

const editKernelArgsModalOpen = ref(false)

const { data: status } = useResourceWatch<KernelArgsStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    id: machine,
    namespace: DefaultNamespace,
    type: KernelArgsStatusType,
  },
}))

const { data: kernelArgs } = useResourceWatch<KernelArgsSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    id: machine,
    namespace: DefaultNamespace,
    type: KernelArgsType,
  },
}))

const currentArgs = computed(() => status.value?.spec.current_args?.join(' ') ?? '')
const currentCmdline = computed(() => status.value?.spec.current_cmdline ?? '')
</script>

<template>
  <PageContainer class="flex flex-col gap-2">
    <div class="text-sm font-semibold text-naturals-n14">Current Kernel Cmdline</div>
    <code class="rounded bg-naturals-n6 px-2.5 py-2 font-mono text-xs text-naturals-n13">
      {{ currentCmdline || 'none' }}
    </code>

    <div class="text-sm font-semibold text-naturals-n14">Extra Kernel Args</div>
    <div class="my-0.5 flex items-center gap-2 rounded bg-naturals-n6 pr-2">
      <code class="flex-1 rounded bg-naturals-n6 px-2.5 py-2 font-mono text-xs text-naturals-n13">
        {{ currentArgs || 'none' }}
      </code>

      <Tooltip description="Edit Extra Kernel Args">
        <IconButton
          aria-label="Edit Extra Kernel Args"
          icon="edit"
          @click="editKernelArgsModalOpen = true"
        />
      </Tooltip>
    </div>

    <EditKernelArgsModal
      v-model:open="editKernelArgsModalOpen"
      :machine-id="machine"
      :kernel-args
      :kernel-args-status="status"
    />
  </PageContainer>
</template>
