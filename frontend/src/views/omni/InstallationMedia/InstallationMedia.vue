<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { watch } from 'vue'
import { useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { ResourceService } from '@/api/grpc'
import type { InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, InstallationMediaConfigType } from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'

const router = useRouter()

const { data: medias, loading: mediasLoading } = useResourceWatch<InstallationMediaConfigSpec>({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: InstallationMediaConfigType,
  },
})

watch([medias, mediasLoading], ([medias, mediasLoading]) => {
  if (mediasLoading) return

  // Redirect to wizard immediately if no media have been created yet
  if (!medias.length) router.replace({ name: 'InstallationMediaCreate' })
})

async function deleteItem(id: string) {
  await ResourceService.Delete(
    {
      namespace: DefaultNamespace,
      type: InstallationMediaConfigType,
      id,
    },
    withRuntime(Runtime.Omni),
  )
}
</script>

<template>
  <div class="flex h-full flex-col gap-6">
    <h1 class="shrink-0 text-xl font-medium text-naturals-n14">Installation Media</h1>

    <TSpinner v-if="mediasLoading" class="size-8 self-center" />

    <ul>
      <li v-for="media in medias" :key="media.metadata.id" class="flex items-center gap-4">
        <span class="text-sm">{{ media.metadata.id }}</span>

        <code class="rounded-sm bg-naturals-n4 px-2 py-1 text-xs">
          {{ media.spec.cloud?.platform ?? media.spec.sbc?.overlay ?? 'metal' }}-{{
            media.spec.architecture
          }}
        </code>

        <TButton size="sm" @click="() => deleteItem(media.metadata.id!)">Delete</TButton>
      </li>
    </ul>
  </div>
</template>
