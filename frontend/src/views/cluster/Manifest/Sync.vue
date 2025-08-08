<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-2">
    <div class="flex items-start">
      <page-header class="flex-1" :title="title" />
      <t-button type="highlighted" @click="applyChanges" :disabled="applyChangesDisabled"
        >Apply Changes ({{ numChanges }})</t-button
      >
    </div>
    <t-alert v-if="error" title="Manifest Sync Error" type="error"> {{ error }}. </t-alert>
    <div class="flex-1 font-sm overflow-y-auto" ref="resultsComponent">
      <div v-if="loading" class="w-full h-full flex items-center justify-center">
        <t-spinner class="w-6 h-6" />
      </div>
      <t-list-item
        v-for="item in syncResults"
        :key="itemID(item)"
        :isDefaultOpened="itemImportant(item)"
      >
        <template #default>
          <span class="label">
            {{ itemLabel(item) }}
          </span>
          <span
            :class="{
              'text-primary-P3':
                item.diff &&
                syncParams.dry_run &&
                item.response_type !== KubernetesSyncManifestResponseResponseType.UNKNOWN,
              'text-green-G1':
                item.diff &&
                !syncParams.dry_run &&
                item.response_type !== KubernetesSyncManifestResponseResponseType.UNKNOWN,
              'text-red-R1':
                item.response_type === KubernetesSyncManifestResponseResponseType.UNKNOWN,
            }"
            >{{ item.path }}</span
          >
          <span v-if="!item.diff" class="text-naturals-N9"> (No changes) </span>
          <span v-if="item.diff && !syncParams.dry_run" class="text-green-G1"> (Updated) </span>
        </template>
        <template
          #details
          v-if="
            [
              KubernetesSyncManifestResponseResponseType.MANIFEST,
              KubernetesSyncManifestResponseResponseType.UNKNOWN,
            ].includes(item.response_type!)
          "
        >
          <div class="diff bottom-line" v-if="item.diff">
            <div v-for="diff in item.diff.split('\n')" :key="diff" :class="highlightDiff(diff)">
              {{ diff }}
            </div>
          </div>
          <div class="diff" v-if="item.object">
            {{
              // TODO: Fix the type? Marked as Uint8Array but we are receiving a b64 encoded string
              textDecoder.decode(b64Decode(item.object as unknown as string))
            }}
          </div>
        </template>
      </t-list-item>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, ref, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { useRoute } from 'vue-router'

import PageHeader from '@/components/common/PageHeader.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TListItem from '@/components/common/List/TListItem.vue'
import TButton from '@/components/common/Button/TButton.vue'
import { showSuccess } from '@/notification'
import TAlert from '@/components/TAlert.vue'

import type {
  KubernetesSyncManifestRequest,
  KubernetesSyncManifestResponse,
} from '@/api/omni/management/management.pb'
import {
  ManagementService,
  KubernetesSyncManifestResponseResponseType,
} from '@/api/omni/management/management.pb'
import type { Stream } from '@/api/grpc'
import { subscribe } from '@/api/grpc'
import { withContext, withRuntime } from '@/api/options'
import { Runtime } from '@/api/common/omni.pb'
import { getContext } from '@/context'
import { b64Decode } from '@/api/fetch.pb'

const textDecoder = new TextDecoder()

const route = useRoute()
const context = getContext()

const loading = ref(true)
const loaded = ref(false)

const resultsComponent: Ref<HTMLDivElement | undefined> = ref()

const title = computed(() => {
  return `Bootstrap Manifest Sync for ${route.params.cluster}`
})

const syncParams: Ref<KubernetesSyncManifestRequest> = ref({
  dry_run: true,
})

const applyChangesDisabled = computed(() => {
  return !loaded.value || loading.value || !syncParams.value.dry_run || numChanges.value === 0
})

const numChanges = computed(() => {
  return syncResults.value.filter((item) => item.diff !== undefined && item.diff !== '').length
})

const applyChanges = () => {
  syncParams.value.dry_run = false
}

const syncResults: Ref<KubernetesSyncManifestResponse[]> = ref([])

const itemID = (item: KubernetesSyncManifestResponse) => {
  return `${item.response_type}-${item.path}`
}

const itemImportant = (item: KubernetesSyncManifestResponse) => {
  return (
    item.response_type === KubernetesSyncManifestResponseResponseType.UNKNOWN ||
    (item.response_type === KubernetesSyncManifestResponseResponseType.MANIFEST &&
      item.diff !== undefined &&
      item.diff !== '' &&
      syncParams.value.dry_run)
  )
}

const itemLabel = (item: KubernetesSyncManifestResponse) => {
  switch (item.response_type) {
    case KubernetesSyncManifestResponseResponseType.UNKNOWN:
      return 'ERROR'
    case KubernetesSyncManifestResponseResponseType.MANIFEST:
      return 'Manifest'
    case KubernetesSyncManifestResponseResponseType.ROLLOUT:
      return 'Rollout'
    default:
      return 'Unknown'
  }
}

const highlightDiff = (line: string) => {
  if (line.startsWith('@@')) {
    return 'text-talos-gray-500'
  }

  if (line.startsWith('- ')) {
    return 'text-red-500'
  }

  if (line.startsWith('+ ')) {
    return 'text-green-500'
  }

  return ''
}

const error = ref<string>()

const setupSyncStream = () => {
  const stream: Ref<
    Stream<KubernetesSyncManifestRequest, KubernetesSyncManifestResponse> | undefined
  > = ref()

  const reset = () => {
    syncResults.value = []
    loaded.value = false
  }

  const processItem = (resp: KubernetesSyncManifestResponse) => {
    loading.value = false

    syncResults.value = syncResults.value.concat(resp)

    nextTick(() => {
      if (resultsComponent.value && !loaded.value) {
        // scroll to the bottom
        resultsComponent.value.scrollTop = resultsComponent.value.scrollHeight
      }
    })
  }

  const init = () => {
    if (stream.value) {
      stream.value.shutdown()
      loading.value = true
    }

    reset()

    stream.value = subscribe(
      ManagementService.KubernetesSyncManifests,
      syncParams.value,
      processItem,
      [withRuntime(Runtime.Talos), withContext(context)],
      undefined,
      (e: Error) => {
        error.value = e.message
        loading.value = false
      },
      () => {
        loaded.value = true
        error.value = undefined

        if (!syncParams.value.dry_run) {
          showSuccess('Bootstrap manifests updated successfully')
        }
      },
    )
  }

  onMounted(init)

  onUnmounted(() => {
    if (stream.value) stream.value.shutdown()
  })

  watch(syncParams.value, init)

  return stream
}

setupSyncStream()
</script>

<style scoped>
.diff {
  @apply font-roboto whitespace-pre pt-2;
}

.bottom-line {
  border-bottom: 1px solid rgba(39, 41, 50);
  border-radius: 4px 4px 0 0;
}
.label {
  @apply uppercase font-bold text-xs bg-naturals-N3 text-naturals-N9 px-2 py-1 rounded-full mr-2;
}
</style>
