<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { AdvertisedEndpointsSpec } from '@/api/omni/specs/virtual.pb'
import { AdvertisedEndpointsID, AdvertisedEndpointsType, VirtualNamespace } from '@/api/resources'
import Code from '@/components/Labels/Code.vue'
import { useResourceGet } from '@/methods/useResourceGet'

defineProps<{ secretKey: string }>()

const { data: endpoints } = useResourceGet<AdvertisedEndpointsSpec>({
  resource: {
    type: AdvertisedEndpointsType,
    namespace: VirtualNamespace,
    id: AdvertisedEndpointsID,
  },
  runtime: Runtime.Omni,
})

const apiURL = computed(() => (endpoints.value ? endpoints.value.spec.grpc_api_url : 'loading...'))
</script>

<template>
  <div v-if="apiURL" class="flex flex-col gap-1 text-xs">
    <span class="text-naturals-n13">
      Set the following environment variables to use the service account:
    </span>
    <Code :text="`export OMNI_ENDPOINT=${apiURL}\nexport OMNI_SERVICE_ACCOUNT_KEY=${secretKey}`" />

    <span class="font-bold text-primary-p2">
      Store the key securely as it will not be displayed again.
    </span>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

code {
  @apply rounded bg-naturals-n4;
}
</style>
