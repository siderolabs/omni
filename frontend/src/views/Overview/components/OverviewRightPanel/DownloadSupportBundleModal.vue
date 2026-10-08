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
import IconButton from '@/components/Button/IconButton.vue'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import type { IconType } from '@/components/Icon/TIcon.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import Modal from '@/components/Modals/Modal.vue'
import ProgressBar from '@/components/ProgressBar/ProgressBar.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { downloadFile } from '@/methods'
import { useClusterPermissions } from '@/methods/auth'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError } from '@/notification'

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
const downloadedURL = ref<string>()
const encrypt = ref(true)

// the filename has to match what was actually downloaded, so freeze the toggle once the
// request goes out.
const encryptLocked = computed(() => downloading.value || downloadedURL.value !== undefined)

const closeText = computed(() => {
  return downloadedURL.value ? 'Close' : 'Cancel'
})

let abortController: AbortController

onUnmounted(() => {
  abortController?.abort()
})

watchEffect(() => {
  if (!open.value) {
    abortController?.abort()

    if (downloadedURL.value) {
      window.URL.revokeObjectURL(downloadedURL.value)
    }

    return
  }

  // Reset the modal when opening
  sourceToProgress.value = {}
  expanded.value = {}
  downloading.value = false
  downloadedURL.value = undefined
  encrypt.value = true
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
        encrypt: encrypt.value,
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
              color: 'var(--color-status-warning-default)',
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
            current.color = 'var(--color-status-danger-default)'
            current.icon = 'exclamation-triangle'
          } else if (
            current.progress === 100 &&
            current.color !== 'var(--color-status-danger-default)'
          ) {
            current.color = 'var(--color-status-success-default)'
            current.icon = 'check-circle'
          }
        }

        if (bundle_data) {
          const data = bundle_data as unknown as string // bundle_data is actually not a Uint8Array, but a base64 string
          const rawData = b64Decode(data) as Uint8Array<ArrayBuffer>
          const blob = new Blob([rawData], {
            type: encrypt.value ? 'application/octet-stream' : 'application/zip',
          })

          downloadedURL.value = window.URL.createObjectURL(blob)
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

async function confirm() {
  if (!downloadedURL.value) {
    await download()
  }

  if (downloadedURL.value) {
    downloadFile(downloadedURL.value, encrypt.value ? 'support.zip.age' : 'support.zip')
  }
}

const { canDownloadSupportBundle } = useClusterPermissions(computed(() => clusterId))
</script>

<template>
  <Modal
    v-model:open="open"
    title="Download Support Bundle"
    :cancel-label="closeText"
    :action-label="downloadedURL ? 'Save' : 'Download'"
    :action-disabled="!canDownloadSupportBundle"
    :loading="downloading"
    @confirm="confirm"
  >
    <template #description>Cluster: {{ clusterId }}</template>

    <div class="mb-compact">
      <Tooltip
        description="To encrypt to your own key as well, so that you can open the bundle too, use omnictl support --encryption-recipients."
      >
        <TCheckbox v-model="encrypt" :disabled="encryptLocked" label="Encrypt for Sidero Labs" />
      </Tooltip>

      <p class="mt-micro ml-5.5 text-xs text-content-muted">
        {{
          encrypt
            ? 'Only Sidero Labs team will be able to open the bundle, so it is safe to attach to an issue or a ticket.'
            : 'The bundle is downloaded as a plain archive. Anyone you share it with can read it.'
        }}
      </p>
    </div>

    <div class="space-y-tight">
      <div
        v-for="source in sortedSources"
        :key="source"
        class="flex flex-col divide-y divide-border-strong rounded-md border border-border-strong text-xs"
      >
        <div
          class="flex items-center gap-tight overflow-x-hidden p-snug px-snug text-content-default"
        >
          <IconButton
            icon="chevron-up"
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

        <div v-if="expanded[source]" class="py-compact">
          <ul
            v-if="sourceToProgress[source].info"
            class="mx-compact mb-tight grid grid-cols-[auto_1fr] gap-x-tight"
          >
            <li class="col-span-full grid grid-cols-subgrid">
              <span class="font-medium text-content-emphasis">UUID</span>
              {{ sourceToProgress[source].info?.metadata.id }}
            </li>
            <li class="col-span-full grid grid-cols-subgrid">
              <span class="font-medium text-content-emphasis">Node IP</span>
              {{ source }}
            </li>
          </ul>

          <Tooltip
            v-for="state in sourceToProgress[source].states"
            :key="state.text"
            :description="state.error"
          >
            <div
              class="flex cursor-pointer items-center gap-tight px-compact py-0.5 hover:bg-surface-hover"
            >
              <TIcon
                class="h-4 w-4"
                :class="{
                  'text-status-success-text': state.error === undefined,
                  'text-status-danger-text': state.error,
                }"
                :icon="state.error ? 'exclamation-triangle' : 'check'"
              />
              <div>{{ state.text }}</div>
            </div>
          </Tooltip>
        </div>
      </div>
    </div>
  </Modal>
</template>
