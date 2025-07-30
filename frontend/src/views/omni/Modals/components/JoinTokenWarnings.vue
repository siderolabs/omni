<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col overflow-hidden">
    <t-spinner class="w-4 h-4" v-if="loading"/>
    <template v-else-if="joinTokenStatus?.spec.warnings">
      <div class="flex items-center gap-1 text-yellow-Y1"><t-icon icon="warning" class="w-5 h-5"/>Warning</div>
      <div class="flex items-center gap-1 text-yellow-Y1 text-xs">{{ joinTokenStatus.spec.warnings.length }} of {{ pluralize("machine", parseFloat(joinTokenStatus?.spec.use_count ?? '0'), true) }} won't be able to connect if the token is revoked/deleted</div>
      <div class="flex flex-col flex-1 text-xs w-full overflow-y-auto overflow-x-hidden">
        <div v-for="warning in joinTokenStatus?.spec.warnings" class="border-l-2 border-yellow-Y1 py-2 px-4 bg-naturals-N4 my-1 rounded" :key="warning.machine">
          <div class="truncate">
            ID: <span class="text-naturals-N13 font-bold font-mono">{{ warning.machine }}</span>
          </div>
          <div class="truncate col-span-2">
            Details: {{ warning.message }}
          </div>
        </div>
      </div>
    </template>
    <div class="text-xs flex items-center gap-1 text-green-G1" v-else>
      <t-icon icon="check-in-circle"/>
      The token can be safely revoked/deleted.
    </div>
  </div>
</template>

<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb';
import { Resource } from '@/api/grpc';
import { JoinTokenStatusSpec } from '@/api/omni/specs/siderolink.pb';
import { DefaultNamespace, JoinTokenStatusType } from '@/api/resources';
import Watch from '@/api/watch';
import TIcon from '@/components/common/Icon/TIcon.vue';
import TSpinner from '@/components/common/Spinner/TSpinner.vue';
import pluralize from 'pluralize';
import { ref, watch } from 'vue';

const joinTokenStatus = ref<Resource<JoinTokenStatusSpec>>()
const joinTokenStatusWatch = new Watch(joinTokenStatus);

const props = defineProps<{
  id: string
}>();


const emit = defineEmits(['ready'])

joinTokenStatusWatch.setup({
  resource: {
    id: props.id,
    namespace: DefaultNamespace,
    type: JoinTokenStatusType
  },
  runtime: Runtime.Omni,
});

const loading = joinTokenStatusWatch.loading;

watch(joinTokenStatusWatch.running, () => {
  if (joinTokenStatusWatch.running.value) {
    emit('ready');
  }
})
</script>
