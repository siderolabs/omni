<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { onBeforeMount, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { AdvertisedEndpointsSpec } from '@/api/omni/specs/virtual.pb'
import { withRuntime } from '@/api/options'
import { AdvertisedEndpointsID, AdvertisedEndpointsType, VirtualNamespace } from '@/api/resources'
import Code from '@/components/common/Labels/Code.vue'

defineProps<{ secretKey: string }>()

const apiURL = ref('loading...')

onBeforeMount(async () => {
  const endpoints = await ResourceService.Get<Resource<AdvertisedEndpointsSpec>>(
    {
      type: AdvertisedEndpointsType,
      namespace: VirtualNamespace,
      id: AdvertisedEndpointsID,
    },
    withRuntime(Runtime.Omni),
  )

  apiURL.value = endpoints.spec.grpc_api_url!
})
</script>

<template>
  <div v-if="apiURL" class="flex flex-col gap-1 text-xs">
    <span class="text-naturals-N13"
      >Set the following environment variables to use the service account:</span
    >
    <Code :text="`export OMNI_ENDPOINT=${apiURL}\nexport OMNI_SERVICE_ACCOUNT_KEY=${secretKey}`" />

    <span class="font-bold text-primary-P2"
      >Store the key securely as it will not be displayed again.</span
    >
  </div>
</template>

<style scoped>
code {
  @apply rounded bg-naturals-N4;
}
</style>
