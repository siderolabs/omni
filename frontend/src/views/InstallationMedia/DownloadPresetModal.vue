<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { compare } from 'semver'
import { computed, ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { InstallationMediaConfigSpec, TalosVersionSpec } from '@/api/omni/specs/omni.pb'
import type { JoinTokenStatusSpec } from '@/api/omni/specs/siderolink.pb'
import { PlatformConfigSpecArch } from '@/api/omni/specs/virtual.pb'
import {
  DefaultNamespace,
  DefaultTalosVersion,
  InstallationMediaConfigType,
  JoinTokenStatusType,
  TalosVersionType,
} from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import TButton from '@/components/Button/TButton.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TableCell from '@/components/Table/TableCell.vue'
import TableRoot from '@/components/Table/TableRoot.vue'
import TableRow from '@/components/Table/TableRow.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { useDownloadImage } from '@/methods/useDownloadImage'
import { useResourceGet } from '@/methods/useResourceGet'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError } from '@/notification'
import { resolveTalosVersion } from '@/views/InstallationMedia/useFormState'
import { usePresetDownloadLinks } from '@/views/InstallationMedia/usePresetDownloadLinks'
import { usePresetSchematic } from '@/views/InstallationMedia/usePresetSchematic'
import CloseButton from '@/views/Modals/CloseButton.vue'

const { open, id } = defineProps<{
  open?: boolean
  id: string
}>()

const emit = defineEmits<{
  close: []
}>()

const { data } = useResourceGet<InstallationMediaConfigSpec>(() => ({
  skip: !open,
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: InstallationMediaConfigType,
    id,
  },
}))

const { copy } = useClipboard({ copiedDuring: 1000 })

// Fetch available versions and tokens for pickers
const { data: talosVersionList } = useResourceWatch<TalosVersionSpec>(() => ({
  skip: !open,
  runtime: Runtime.Omni,
  resource: {
    type: TalosVersionType,
    namespace: DefaultNamespace,
  },
}))

const { data: joinTokenList } = useResourceWatch<JoinTokenStatusSpec>(() => ({
  skip: !open,
  runtime: Runtime.Omni,
  resource: {
    type: JoinTokenStatusType,
    namespace: DefaultNamespace,
  },
}))

// Picker state
const selectedVersion = ref<string>()
const selectedToken = ref<string>()
const selectedArch = ref<PlatformConfigSpecArch>()

const defaultTokenId = computed(
  () => joinTokenList.value.find((t) => t.spec.is_default)?.metadata.id,
)

watchEffect(() => {
  // Reset modal state on close
  if (!open) {
    selectedVersion.value = undefined
    selectedToken.value = undefined
    selectedArch.value = undefined
    return
  }

  if (data.value) {
    selectedVersion.value = data.value.spec.talos_version || DefaultTalosVersion
    selectedToken.value = data.value.spec.join_token || defaultTokenId.value
    selectedArch.value = data.value.spec.architecture
  }
})

// Picker options
const talosVersionOptions = computed(() =>
  talosVersionList.value
    .filter((v) => !v.spec.deprecated)
    .map((v) => v.spec.version!)
    .sort(compare),
)

const joinTokenOptions = computed(() =>
  joinTokenList.value
    .toSorted((a, b) => {
      if (a.spec.is_default) return -1
      if (b.spec.is_default) return 1

      return 0
    })
    .map((t) => ({
      label: t.spec.name || t.metadata.id || '',
      value: t.metadata.id || '',
    })),
)

const isSBC = computed(() => !!data.value?.spec.sbc)

const archOptions = computed(() => {
  if (isSBC.value) {
    return [{ label: 'arm64', value: PlatformConfigSpecArch.ARM64 }]
  }

  return [
    { label: 'amd64', value: PlatformConfigSpecArch.AMD64 },
    { label: 'arm64', value: PlatformConfigSpecArch.ARM64 },
  ]
})

// Resolved preset: merges original preset with picker overrides
const resolvedPreset = computed<InstallationMediaConfigSpec>(() => {
  const spec = data.value?.spec ?? {}

  return {
    ...spec,
    talos_version: selectedVersion.value ?? resolveTalosVersion(spec.talos_version),
    join_token: selectedToken.value ?? spec.join_token,
    architecture: selectedArch.value ?? spec.architecture,
  }
})

const schematicId = computed(() => schematic.value?.id ?? '')

const { schematic } = usePresetSchematic(resolvedPreset)
const { links } = usePresetDownloadLinks(schematicId, resolvedPreset)

const {
  isGenerating: imageIsGenerating,
  abort: abortImageDownload,
  download: _downloadImage,
} = useDownloadImage()

async function downloadImage(url: string) {
  try {
    await _downloadImage(url)
  } catch (error) {
    showError('Image download failed', error instanceof Error ? error.message : String(error))
  }
}

async function close() {
  abortImageDownload()
  emit('close')
}
</script>

<template>
  <div
    class="fixed inset-0 z-30 flex items-center justify-center bg-naturals-n0/90"
    :class="!open && 'hidden'"
  >
    <div class="modal-window gap-4">
      <div class="flex items-start justify-between">
        <div class="flex flex-col gap-2 text-naturals-n14">
          <h3 class="text-base font-medium">Download</h3>
          <p class="text-sm">Files for {{ id }}</p>
        </div>

        <CloseButton class="shrink-0" @click="close" />
      </div>

      <div class="flex flex-wrap gap-4">
        <TSelectList
          v-model="selectedVersion"
          :values="talosVersionOptions"
          title="Talos Version"
          overhead-title
        />

        <TSelectList
          v-model="selectedToken"
          :values="joinTokenOptions"
          title="Join Token"
          overhead-title
        />

        <TSelectList
          v-model="selectedArch"
          :values="archOptions"
          title="Architecture"
          overhead-title
        />
      </div>

      <TableRoot class="w-full">
        <template #head>
          <TableRow>
            <TableCell th>File type</TableCell>
            <TableCell th>Action</TableCell>
          </TableRow>
        </template>

        <template #body>
          <TableRow v-for="{ label, link } in links" :key="link">
            <TableCell>{{ label }}</TableCell>

            <TableCell class="w-0">
              <div class="flex gap-1">
                <Tooltip description="Download">
                  <IconButton
                    is="a"
                    :href="link"
                    :disabled="imageIsGenerating"
                    target="_blank"
                    rel="noopener noreferrer"
                    aria-label="download"
                    icon="arrow-down-tray"
                    @click.prevent="downloadImage(link)"
                  />
                </Tooltip>

                <Tooltip description="Copy link">
                  <IconButton aria-label="copy link" icon="copy" @click="copy(link)" />
                </Tooltip>
              </div>
            </TableCell>
          </TableRow>
        </template>
      </TableRoot>

      <div class="flex gap-1">
        <p v-if="imageIsGenerating" class="flex items-center gap-1.5 text-xs">
          <TSpinner class="size-3" />
          <span>Generating image...</span>
        </p>

        <div class="grow"></div>

        <TButton @click="close">Close</TButton>
      </div>
    </div>
  </div>
</template>
