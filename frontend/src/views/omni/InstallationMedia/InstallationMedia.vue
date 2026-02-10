<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { formatISO, parseISO } from 'date-fns'
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { ResourceService } from '@/api/grpc'
import type { InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, InstallationMediaConfigType } from '@/api/resources'
import { itemID } from '@/api/watch'
import IconButton from '@/components/common/Button/IconButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TableCell from '@/components/common/Table/TableCell.vue'
import TableRoot from '@/components/common/Table/TableRoot.vue'
import TableRow from '@/components/common/Table/TableRow.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showSuccess } from '@/notification'
import DownloadPresetModal from '@/views/omni/InstallationMedia/DownloadPresetModal.vue'
import { presetToFormState } from '@/views/omni/InstallationMedia/formStateToPreset'
import { useFormState } from '@/views/omni/InstallationMedia/useFormState'
import ConfirmModal from '@/views/omni/Modals/ConfirmModal.vue'

const router = useRouter()
const { formState } = useFormState()

const { data: presets, loading: presetsLoading } = useResourceWatch<InstallationMediaConfigSpec>({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: InstallationMediaConfigType,
  },
})

watch([presets, presetsLoading], ([presets, presetsLoading]) => {
  if (presetsLoading) return

  // Redirect to wizard immediately if no media have been created yet
  if (!presets.length) router.replace({ name: 'InstallationMediaCreate' })
})

const presetIdToDelete = ref<string>()
const confirmDeleteModalOpen = ref(false)

function openConfirmDeleteModal(id: string) {
  presetIdToDelete.value = id
  confirmDeleteModalOpen.value = true
}

const presetIdToDownload = ref<string>()
const downloadPresetModalOpen = ref(false)

function openDownloadPresetModal(id: string) {
  presetIdToDownload.value = id
  downloadPresetModalOpen.value = true
}

async function deleteItem(id: string) {
  await ResourceService.Delete(
    {
      namespace: DefaultNamespace,
      type: InstallationMediaConfigType,
      id,
    },
    withRuntime(Runtime.Omni),
  )

  showSuccess(`Deleted preset ${id}`)
  confirmDeleteModalOpen.value = false
}

function clonePreset(preset: (typeof presets.value)[number]) {
  formState.value = presetToFormState(preset.spec)

  router.push({ name: 'InstallationMediaCreate' })
}
</script>

<template>
  <div class="flex h-full flex-col gap-6">
    <TSpinner v-if="presetsLoading" class="size-8 self-center" />

    <div class="flex items-start justify-between gap-1">
      <PageHeader title="Installation Media" />
      <TButton
        is="router-link"
        variant="highlighted"
        :to="{ name: 'InstallationMediaCreate' }"
        @click="formState = {}"
      >
        Create New
      </TButton>
    </div>

    <TableRoot class="w-full">
      <template #head>
        <TableRow>
          <TableCell th>Name</TableCell>
          <TableCell th>Talos version</TableCell>
          <TableCell th>Date created</TableCell>
          <TableCell th>Action</TableCell>
        </TableRow>
      </template>

      <template #body>
        <TableRow v-for="preset in presets" :key="itemID(preset)">
          <TableCell>
            <RouterLink
              class="list-item-link"
              :to="{
                name: 'InstallationMediaReview',
                params: { presetId: preset.metadata.id! },
              }"
            >
              {{ preset.metadata.id }}
            </RouterLink>
          </TableCell>
          <TableCell>{{ preset.spec.talos_version }}</TableCell>
          <TableCell>
            {{
              preset.metadata.created &&
              formatISO(parseISO(preset.metadata.created), { representation: 'date' })
            }}
          </TableCell>

          <TableCell class="w-0">
            <div class="flex gap-1">
              <Tooltip description="Review">
                <IconButton
                  is="router-link"
                  aria-label="review"
                  icon="info"
                  :to="{
                    name: 'InstallationMediaReview',
                    params: { presetId: preset.metadata.id! },
                  }"
                />
              </Tooltip>

              <Tooltip description="Clone">
                <IconButton
                  aria-label="clone"
                  icon="arrow-up-on-square-stack"
                  @click="() => clonePreset(preset)"
                />
              </Tooltip>

              <Tooltip description="Download">
                <IconButton
                  aria-label="download"
                  icon="arrow-down-tray"
                  @click="() => openDownloadPresetModal(preset.metadata.id!)"
                />
              </Tooltip>

              <Tooltip description="Delete">
                <IconButton
                  aria-label="delete"
                  aria-haspopup="dialog"
                  icon="delete"
                  class="ml-4 text-red-r1"
                  @click="() => openConfirmDeleteModal(preset.metadata.id!)"
                />
              </Tooltip>
            </div>
          </TableCell>
        </TableRow>
      </template>
    </TableRoot>

    <ConfirmModal
      v-if="presetIdToDelete"
      v-model:open="confirmDeleteModalOpen"
      @confirm="deleteItem(presetIdToDelete)"
    >
      Are you sure you want to delete preset "{{ presetIdToDelete }}"?
    </ConfirmModal>

    <DownloadPresetModal
      v-if="presetIdToDownload"
      :id="presetIdToDownload"
      :open="downloadPresetModalOpen"
      @close="downloadPresetModalOpen = false"
    />
  </div>
</template>
