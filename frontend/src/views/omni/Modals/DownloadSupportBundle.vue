<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { b64Decode } from '@/api/fetch.pb'
import { ManagementService } from '@/api/omni/management/management.pb'
import type { TalosUpgradeStatusSpec } from '@/api/omni/specs/omni.pb'
import { withAbortController } from '@/api/options'
import { DefaultNamespace, TalosUpgradeStatusType } from '@/api/resources'
import IconButton from '@/components/common/Button/IconButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import ProgressBar from '@/components/common/ProgressBar/ProgressBar.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { setupClusterPermissions } from '@/methods/auth'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const route = useRoute()
const router = useRouter()
const expanded = ref<Record<string, boolean>>({})

const selectedVersion = ref('')

const clusterName = route.params.cluster as string

const { data: status } = useResourceWatch<TalosUpgradeStatusSpec>({
  resource: {
    namespace: DefaultNamespace,
    type: TalosUpgradeStatusType,
    id: clusterName,
  },
  runtime: Runtime.Omni,
})

watch(status, () => {
  if (selectedVersion.value === '') {
    selectedVersion.value = status.value?.spec.last_upgrade_version || ''
  }
})

let closed = false

const close = () => {
  if (closed) {
    return
  }

  closed = true

  router.go(-1)
}

interface State {
  error?: string
  text?: string
}

interface Progress {
  states: State[]
  currentNum: number
  progress: number
  icon: IconType
  color: string
}

const sourceToProgress: Ref<Record<string, Progress>> = ref({})
const sortedSources = computed(() => Object.keys(sourceToProgress.value).sort())

const started = ref(false)
const done = ref(false)

const closeText = computed(() => {
  return done.value ? 'Close' : 'Cancel'
})

const abortController = new AbortController()

let canceled = false

onUnmounted(() => {
  abortController.abort()

  canceled = true
})

const download = async () => {
  try {
    started.value = true
    sourceToProgress.value = {}

    await ManagementService.GetSupportBundle(
      {
        cluster: clusterName,
      },
      (resp) => {
        if (resp?.progress?.source) {
          let current = sourceToProgress.value[resp.progress.source]

          if (!current) {
            current = {
              states: [],
              currentNum: 0,
              progress: 0,
              icon: 'loading',
              color: 'var(--color-yellow-y1)',
            }

            sourceToProgress.value[resp.progress.source] = current
          }

          current.currentNum += 1
          current.progress = (current.currentNum / (resp.progress.total ?? 1)) * 100

          if (resp.progress.state) {
            current.states.push({
              error: resp.progress.error,
              text: resp.progress.state,
            })
          }

          if (resp.progress.error) {
            current.color = 'var(--color-red-r1)'
            current.icon = 'warning'
          } else if (current.progress === 100 && current.color !== 'var(--color-red-r1)') {
            current.color = 'var(--color-green-g1)'
            current.icon = 'check-in-circle-classic'
          }
        }

        if (resp.bundle_data) {
          const data = resp.bundle_data as unknown as string // bundle_data is actually not a Uint8Array, but a base64 string
          const rawData = b64Decode(data) as Uint8Array<ArrayBuffer>
          const blob = new Blob([rawData], { type: 'application/zip' })

          const url = window.URL.createObjectURL(blob)
          downloadURL(url, 'support.zip')

          done.value = true

          return
        }
      },
      withAbortController(abortController),
    )
  } catch (e) {
    if (canceled) {
      return
    }

    showError('Download Failed', e.message)
  }
}

const downloadURL = (data: string, fileName: string) => {
  const a = document.createElement('a')
  a.href = data
  a.download = fileName
  document.body.appendChild(a)
  a.style.display = 'none'
  a.click()
  a.remove()
}

const { canDownloadSupportBundle } = setupClusterPermissions(
  computed(() => route.params.cluster as string),
)
</script>

<template>
  <div
    class="z-30 flex h-screen items-center justify-center gap-2 overflow-hidden py-4"
    :class="{ 'w-3/5': Object.keys(sourceToProgress).length > 0 }"
  >
    <div class="flex max-h-full w-full min-w-fit flex-col gap-2 rounded bg-naturals-n2 p-8">
      <div class="heading">
        <h3 class="text-base text-naturals-n14">Download Support Bundle</h3>
        <CloseButton @click="close" />
      </div>

      <div class="flex-1 overflow-y-auto pr-2">
        <template v-if="sortedSources.length > 0">
          <div
            v-for="source in sortedSources"
            :key="source"
            class="my-1 flex flex-col divide-y divide-naturals-n6 overflow-hidden rounded-md border border-naturals-n6 text-xs"
          >
            <div class="flex gap-2 bg-naturals-n3 p-3 px-3">
              <IconButton
                icon="arrow-up"
                class="transition-transform"
                :class="{ 'rotate-180': expanded[source] }"
                @click="() => (expanded[source] = !expanded[source])"
              />
              <div class="list-grid flex-1 text-naturals-n12">
                <div class="truncate">{{ source }}</div>
                <div class="col-span-2 flex items-center gap-2">
                  <TIcon
                    class="h-4 w-4"
                    :icon="sourceToProgress[source].icon"
                    :style="{ color: sourceToProgress[source].color }"
                  />
                  <ProgressBar
                    class="flex-1"
                    :progress="sourceToProgress[source].progress"
                    :color="sourceToProgress[source].color"
                  />
                </div>
              </div>
            </div>
            <div v-if="expanded[source]" class="py-4">
              <Tooltip
                v-for="state in sourceToProgress[source].states"
                :key="state.text"
                :description="state.error"
              >
                <div
                  class="flex cursor-pointer items-center gap-2 px-4 py-0.5 hover:bg-naturals-n4"
                >
                  <TIcon
                    class="h-4 w-4"
                    :class="{
                      'text-green-g1': state.error === undefined,
                      'text-red-r1': state.error,
                    }"
                    :icon="state.error ? 'warning' : 'check'"
                  />
                  <div>{{ state.text }}</div>
                </div>
              </Tooltip>
            </div>
          </div>
        </template>
      </div>

      <div class="flex justify-end gap-4">
        <TButton
          class="h-9 w-32"
          :disabled="!canDownloadSupportBundle || started"
          type="highlighted"
          @click="download"
        >
          <TSpinner v-if="!status" class="h-5 w-5" />
          <span v-else>Download</span>
        </TButton>
        <TButton class="h-9 w-32" @click="close">
          <TSpinner v-if="!status" class="h-5 w-5" />
          <span v-else>{{ closeText }}</span>
        </TButton>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-n14;
}

optgroup {
  @apply font-bold text-naturals-n14;
}

.list-grid {
  @apply grid grid-cols-3 items-center justify-center gap-2;
}

.header {
  @apply mb-1 bg-naturals-n2 px-6 py-2 text-xs;
}
</style>
