<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, onUnmounted, ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { b64Decode } from '@/api/fetch.pb'
import type { Resource } from '@/api/grpc'
import { ManagementService } from '@/api/omni/management/management.pb'
import type { ClusterMachineIdentitySpec } from '@/api/omni/specs/omni.pb'
import { withAbortController } from '@/api/options'
import { ClusterMachineIdentityType, DefaultNamespace, LabelCluster } from '@/api/resources'
import IconButton from '@/components/common/Button/IconButton.vue'
import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import ProgressBar from '@/components/common/ProgressBar/ProgressBar.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { setupClusterPermissions } from '@/methods/auth'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError } from '@/notification'
import Modal from '@/views/omni/Modals/Modal.vue'

const { clusterId } = defineProps<{
  clusterId: string
}>()

const open = defineModel<boolean>('open', { default: false })
const expanded = ref<Record<string, boolean>>({})

interface State {
  error?: string
  text?: string
}

interface Progress {
  title: string
  states: State[]
  currentNum: number
  progress: number
  icon: IconType
  color: string
  info?: Resource<ClusterMachineIdentitySpec>
}

const sourceToProgress: Ref<Record<string, Progress>> = ref({})
const sortedSources = computed(() => Object.keys(sourceToProgress.value).sort())

const { data } = useResourceWatch<ClusterMachineIdentitySpec>(() => ({
  skip: !open.value,
  resource: {
    type: ClusterMachineIdentityType,
    namespace: DefaultNamespace,
  },
  selectors: [`${LabelCluster}=${clusterId}`],
  runtime: Runtime.Omni,
}))

const downloading = ref(false)
const done = ref(false)

const closeText = computed(() => {
  return done.value ? 'Close' : 'Cancel'
})

watchEffect(() => {
  // Reset the modal when opening
  if (open.value) {
    sourceToProgress.value = {}
    expanded.value = {}
    done.value = false
  }
})

let abortController: AbortController

onUnmounted(() => {
  abortController?.abort()
})

const download = async () => {
  try {
    abortController?.abort()
    abortController = new AbortController()

    downloading.value = true
    sourceToProgress.value = {}

    await ManagementService.GetSupportBundle(
      {
        cluster: clusterId,
      },
      ({ bundle_data, progress }) => {
        if (progress?.source) {
          const { source, total, error, state } = progress

          if (!sourceToProgress.value[source]) {
            const info = !['omni', 'cluster'].includes(source)
              ? data.value.find((c) => c.spec.node_ips?.includes(source))
              : undefined

            sourceToProgress.value[source] = {
              title: info?.spec.nodename ?? source,
              states: [],
              currentNum: 0,
              progress: 0,
              icon: 'loading',
              color: 'var(--color-yellow-y1)',
              info,
            }
          }

          const current = sourceToProgress.value[source]

          current.currentNum += 1
          current.progress = (current.currentNum / (total ?? 1)) * 100

          if (state) {
            current.states.push({
              error: error,
              text: state,
            })
          }

          if (progress.error) {
            current.color = 'var(--color-red-r1)'
            current.icon = 'warning'
          } else if (current.progress === 100 && current.color !== 'var(--color-red-r1)') {
            current.color = 'var(--color-green-g1)'
            current.icon = 'check-in-circle-classic'
          }
        }

        if (bundle_data) {
          const data = bundle_data as unknown as string // bundle_data is actually not a Uint8Array, but a base64 string
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
    if (abortController.signal.aborted) return

    showError('Download Failed', e.message)
  } finally {
    downloading.value = false
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

const { canDownloadSupportBundle } = setupClusterPermissions(computed(() => clusterId))
</script>

<template>
  <Modal
    v-model:open="open"
    title="Download Support Bundle"
    :cancel-label="closeText"
    action-label="Download"
    :action-disabled="!canDownloadSupportBundle"
    :loading="downloading"
    @confirm="download"
  >
    <template #description>Cluster: {{ clusterId }}</template>

    <div class="space-y-2">
      <div
        v-for="source in sortedSources"
        :key="source"
        class="flex flex-col divide-y divide-naturals-n6 rounded-md border border-naturals-n6 text-xs"
      >
        <div class="flex items-center gap-2 overflow-x-hidden p-3 px-3 text-naturals-n12">
          <IconButton
            icon="arrow-up"
            class="shrink-0 transition-transform"
            :class="{ 'rotate-180': expanded[source] }"
            @click="() => (expanded[source] = !expanded[source])"
          />

          <span class="truncate" :title="sourceToProgress[source].title">
            {{ sourceToProgress[source].title }}
          </span>

          <div class="grow" />

          <TIcon
            class="size-4 shrink-0"
            :icon="sourceToProgress[source].icon"
            :style="{ color: sourceToProgress[source].color }"
          />

          <ProgressBar
            v-model="sourceToProgress[source].progress"
            class="w-20 shrink-0 sm:w-40 md:w-80"
            :color="sourceToProgress[source].color"
          />
        </div>

        <div v-if="expanded[source]" class="py-4">
          <ul
            v-if="sourceToProgress[source].info"
            class="mx-4 mb-2 grid grid-cols-[auto_1fr] gap-x-2"
          >
            <li class="col-span-full grid grid-cols-subgrid">
              <span class="font-medium text-naturals-n14">UUID</span>
              {{ sourceToProgress[source].info?.metadata.id }}
            </li>
            <li class="col-span-full grid grid-cols-subgrid">
              <span class="font-medium text-naturals-n14">Node IP</span>
              {{ source }}
            </li>
          </ul>

          <Tooltip
            v-for="state in sourceToProgress[source].states"
            :key="state.text"
            :description="state.error"
          >
            <div class="flex cursor-pointer items-center gap-2 px-4 py-0.5 hover:bg-naturals-n4">
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
    </div>
  </Modal>
</template>
