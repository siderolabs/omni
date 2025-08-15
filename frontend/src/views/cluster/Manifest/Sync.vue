<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { b64Decode } from '@/api/fetch.pb'
import type { Stream } from '@/api/grpc'
import { subscribe } from '@/api/grpc'
import type {
  KubernetesSyncManifestRequest,
  KubernetesSyncManifestResponse,
} from '@/api/omni/management/management.pb'
import {
  KubernetesSyncManifestResponseResponseType,
  ManagementService,
} from '@/api/omni/management/management.pb'
import { withContext, withRuntime } from '@/api/options'
import TButton from '@/components/common/Button/TButton.vue'
import TListItem from '@/components/common/List/TListItem.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { getContext } from '@/context'
import { showSuccess } from '@/notification'

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
    return 'text-neutral-500'
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

<template>
  <div class="flex flex-col gap-2">
    <div class="flex items-start">
      <PageHeader class="flex-1" :title="title" />
      <TButton type="highlighted" :disabled="applyChangesDisabled" @click="applyChanges"
        >Apply Changes ({{ numChanges }})</TButton
      >
    </div>
    <TAlert v-if="error" title="Manifest Sync Error" type="error"> {{ error }}. </TAlert>
    <div ref="resultsComponent" class="font-sm flex-1 overflow-y-auto">
      <div v-if="loading" class="flex h-full w-full items-center justify-center">
        <TSpinner class="h-6 w-6" />
      </div>
      <TListItem
        v-for="item in syncResults"
        :key="itemID(item)"
        :is-default-opened="itemImportant(item)"
      >
        <template #default>
          <span class="label">
            {{ itemLabel(item) }}
          </span>
          <span
            :class="{
              'text-primary-p3':
                item.diff &&
                syncParams.dry_run &&
                item.response_type !== KubernetesSyncManifestResponseResponseType.UNKNOWN,
              'text-green-g1':
                item.diff &&
                !syncParams.dry_run &&
                item.response_type !== KubernetesSyncManifestResponseResponseType.UNKNOWN,
              'text-red-r1':
                item.response_type === KubernetesSyncManifestResponseResponseType.UNKNOWN,
            }"
            >{{ item.path }}</span
          >
          <span v-if="!item.diff" class="text-naturals-n9"> (No changes) </span>
          <span v-if="item.diff && !syncParams.dry_run" class="text-green-g1"> (Updated) </span>
        </template>
        <template
          v-if="
            [
              KubernetesSyncManifestResponseResponseType.MANIFEST,
              KubernetesSyncManifestResponseResponseType.UNKNOWN,
            ].includes(item.response_type!)
          "
          #details
        >
          <div v-if="item.diff" class="diff bottom-line">
            <div v-for="diff in item.diff.split('\n')" :key="diff" :class="highlightDiff(diff)">
              {{ diff }}
            </div>
          </div>
          <div v-if="item.object" class="diff">
            {{
              // TODO: Fix the type? Marked as Uint8Array but we are receiving a b64 encoded string
              textDecoder.decode(b64Decode(item.object as unknown as string))
            }}
          </div>
        </template>
      </TListItem>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.diff {
  @apply pt-2 font-mono whitespace-pre;
}

.bottom-line {
  border-bottom: 1px solid rgba(39, 41, 50);
  border-radius: 4px 4px 0 0;
}
.label {
  @apply mr-2 rounded-full bg-naturals-n3 px-2 py-1 text-xs font-bold text-naturals-n9 uppercase;
}
</style>
