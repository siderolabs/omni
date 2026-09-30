<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
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
import Modal from '@/components/Modals/Modal.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TableCell from '@/components/Table/TableCell.vue'
import TableRoot from '@/components/Table/TableRoot.vue'
import TableRow from '@/components/Table/TableRow.vue'
import TAlert from '@/components/TAlert.vue'
import TalosVersionSelect from '@/components/TalosVersionSelect/TalosVersionSelect.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { useResourceGet } from '@/methods/useResourceGet'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { resolveTalosVersion } from '@/views/InstallationMedia/useFormState'
import { usePresetDownloadLinks } from '@/views/InstallationMedia/usePresetDownloadLinks'
import { usePresetSchematic } from '@/views/InstallationMedia/usePresetSchematic'

const { id } = defineProps<{
  id: string
}>()

const open = defineModel<boolean>('open', { default: false })

const { data } = useResourceGet<InstallationMediaConfigSpec>(() => ({
  skip: !open.value,
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: InstallationMediaConfigType,
    id,
  },
}))

const { copy } = useClipboard({ copiedDuring: 1000 })

const { data: joinTokenList } = useResourceWatch<JoinTokenStatusSpec>(() => ({
  skip: !open.value,
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

const talosVersion = computed(
  () => selectedVersion.value ?? resolveTalosVersion(data.value?.spec.talos_version),
)

const defaultTokenId = computed(
  () => joinTokenList.value.find((t) => t.spec.is_default)?.metadata.id,
)

const { data: talosVersionSpec } = useResourceGet<TalosVersionSpec>(() => ({
  skip: !open.value || !talosVersion.value,
  runtime: Runtime.Omni,
  resource: {
    type: TalosVersionType,
    namespace: DefaultNamespace,
    id: talosVersion.value,
  },
}))

watchEffect(() => {
  // Reset modal state on close
  if (!open.value) {
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

  // The selected Talos version determines which factory serves the image (primary wins on the merged
  // list). Fall back to the preset's stored factory URL for versions no longer listed.
  const versionFactoryURL = talosVersionSpec.value?.spec.image_factory_url

  return {
    ...spec,
    talos_version: talosVersion.value,
    join_token: selectedToken.value ?? spec.join_token,
    architecture: selectedArch.value ?? spec.architecture,
    image_factory_url: versionFactoryURL || spec.image_factory_url,
  }
})

const schematicId = computed(() => schematic.value?.id ?? '')

const { schematic } = usePresetSchematic(resolvedPreset)
const { links, loading, error, orphaned } = usePresetDownloadLinks(schematicId, resolvedPreset)

const orphanedError = 'The factory used to create this preset is no longer configured with Omni'
</script>

<template>
  <Modal v-model:open="open" title="Download" cancel-label="Close">
    <template #description>Files for {{ id }}</template>

    <div class="flex flex-col gap-4">
      <div
        v-if="orphaned"
        class="rounded border border-l-3 border-status-danger-border border-l-status-danger-fill bg-status-danger-surface p-3 text-xs text-content-default"
        role="alert"
      >
        The image factory this preset uses is no longer configured in Omni, so its images can no
        longer be downloaded. Pick a Talos version served by a configured factory, or recreate the
        preset.
      </div>

      <div class="flex flex-wrap gap-4">
        <TalosVersionSelect v-model="selectedVersion" title="Talos Version" overhead-title />

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
                <Tooltip :description="orphaned ? orphanedError : 'Download'">
                  <IconButton
                    is="a"
                    :href="link"
                    :disabled="!schematicId || orphaned"
                    target="_blank"
                    rel="noopener noreferrer"
                    aria-label="download"
                    icon="arrow-down-tray"
                  />
                </Tooltip>

                <Tooltip :description="orphaned ? orphanedError : 'Copy link'">
                  <IconButton
                    aria-label="copy link"
                    icon="copy"
                    :disabled="!schematicId || orphaned"
                    @click="copy(link)"
                  />
                </Tooltip>
              </div>
            </TableCell>
          </TableRow>
        </template>
      </TableRoot>

      <TAlert v-if="error" title="Failed to generate download links" type="error">
        {{ error.message }}
      </TAlert>

      <p v-else-if="loading" class="flex items-center gap-1.5 text-xs">
        <TSpinner class="size-4" />
        Generating links...
      </p>
    </div>
  </Modal>
</template>
