<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->

<script setup lang="ts">
import pluralize from 'pluralize'
import { ref, watch } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { JoinTokenStatusSpec } from '@/api/omni/specs/siderolink.pb'
import { DefaultNamespace, JoinTokenStatusType } from '@/api/resources'
import Watch from '@/api/watch'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'

const joinTokenStatus = ref<Resource<JoinTokenStatusSpec>>()
const joinTokenStatusWatch = new Watch(joinTokenStatus)

const props = defineProps<{
  id: string
}>()

const emit = defineEmits(['ready'])

joinTokenStatusWatch.setup({
  resource: {
    id: props.id,
    namespace: DefaultNamespace,
    type: JoinTokenStatusType,
  },
  runtime: Runtime.Omni,
})

const loading = joinTokenStatusWatch.loading

watch(joinTokenStatusWatch.running, () => {
  if (joinTokenStatusWatch.running.value) {
    emit('ready')
  }
})
</script>

<template>
  <div class="flex flex-col overflow-hidden">
    <TSpinner v-if="loading" class="h-4 w-4" />
    <template v-else-if="joinTokenStatus?.spec.warnings">
      <div class="flex items-center gap-1 text-yellow-y1">
        <TIcon icon="warning" class="h-5 w-5" />Warning
      </div>
      <div class="flex items-center gap-1 text-xs text-yellow-y1">
        {{ joinTokenStatus.spec.warnings.length }} of
        {{ pluralize('machine', parseFloat(joinTokenStatus?.spec.use_count ?? '0'), true) }} won't
        be able to connect if the token is revoked/deleted
      </div>
      <div class="flex w-full flex-1 flex-col overflow-x-hidden overflow-y-auto text-xs">
        <div
          v-for="warning in joinTokenStatus?.spec.warnings"
          :key="warning.machine"
          class="my-1 rounded border-l-2 border-yellow-y1 bg-naturals-n4 px-4 py-2"
        >
          <div class="truncate">
            ID: <span class="font-mono font-bold text-naturals-n13">{{ warning.machine }}</span>
          </div>
          <div class="col-span-2 truncate">Details: {{ warning.message }}</div>
        </div>
      </div>
    </template>
    <div v-else class="flex items-center gap-1 text-xs text-green-g1">
      <TIcon icon="check-in-circle" />
      The token can be safely revoked/deleted.
    </div>
  </div>
</template>
