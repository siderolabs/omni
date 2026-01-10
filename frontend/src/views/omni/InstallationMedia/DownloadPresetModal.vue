<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, InstallationMediaConfigType } from '@/api/resources'
import IconButton from '@/components/common/Button/IconButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import TableCell from '@/components/common/Table/TableCell.vue'
import TableRoot from '@/components/common/Table/TableRoot.vue'
import TableRow from '@/components/common/Table/TableRow.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { useResourceGet } from '@/methods/useResourceGet'
import { usePresetDownloadLinks } from '@/views/omni/InstallationMedia/usePresetDownloadLinks'
import { usePresetSchematic } from '@/views/omni/InstallationMedia/usePresetSchematic'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const { open, id } = defineProps<{
  open?: boolean
  id: string
}>()

defineEmits<{
  close: []
}>()

const { data } = useResourceGet<InstallationMediaConfigSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: InstallationMediaConfigType,
    id,
  },
}))

const { copy } = useClipboard({ copiedDuring: 1000 })

const preset = computed(() => data.value?.spec ?? {})
const schematicId = computed(() => schematic.value?.id ?? '')

const { schematic } = usePresetSchematic(preset)
const { links } = usePresetDownloadLinks(schematicId, preset)
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

        <CloseButton @click="$emit('close')" />
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
                  <IconButton is="a" :href="link" aria-label="download" icon="arrow-down-tray" />
                </Tooltip>

                <Tooltip description="Copy link">
                  <IconButton aria-label="copy link" icon="copy" @click="copy(link)" />
                </Tooltip>
              </div>
            </TableCell>
          </TableRow>
        </template>
      </TableRoot>

      <TButton class="self-end" @click="$emit('close')">Close</TButton>
    </div>
  </div>
</template>
