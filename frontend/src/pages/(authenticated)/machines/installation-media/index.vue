<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { formatISO, parseISO } from 'date-fns'
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { ResourceService } from '@/api/grpc'
import type { InstallationMediaConfigSpec } from '@/api/omni/specs/omni.pb'
import { type JoinTokenStatusSpec, JoinTokenStatusSpecState } from '@/api/omni/specs/siderolink.pb'
import { withRuntime } from '@/api/options'
import {
  DefaultNamespace,
  DefaultTalosVersion,
  InstallationMediaConfigType,
  JoinTokenStatusType,
} from '@/api/resources'
import { itemID } from '@/api/watch'
import IconButton from '@/components/Button/IconButton.vue'
import TButton from '@/components/Button/TButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TableCell from '@/components/Table/TableCell.vue'
import TableRoot from '@/components/Table/TableRoot.vue'
import TableRow from '@/components/Table/TableRow.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { TCommonStatuses } from '@/constants'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showSuccess } from '@/notification'
import DownloadPresetModal from '@/views/InstallationMedia/DownloadPresetModal.vue'
import { presetToFormState } from '@/views/InstallationMedia/formStateToPreset'
import { useFormState } from '@/views/InstallationMedia/useFormState'

definePage({ name: 'InstallationMedia' })

const router = useRouter()
const { formState } = useFormState()

const { data: presets, loading: presetsLoading } = useResourceWatch<InstallationMediaConfigSpec>({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: InstallationMediaConfigType,
  },
})

const { data: joinTokens, loading: joinTokensLoading } = useResourceWatch<JoinTokenStatusSpec>({
  runtime: Runtime.Omni,
  resource: {
    type: JoinTokenStatusType,
    namespace: DefaultNamespace,
  },
})

const loading = computed(() => presetsLoading.value || joinTokensLoading.value)
const defaultToken = computed(() => joinTokens.value.find((t) => t.spec.is_default))

const presetList = computed(() => {
  if (loading.value) return

  return presets.value.map((p) => ({
    ...p,
    tokenAutomatic: !p.spec.join_token,
    token: p.spec.join_token
      ? joinTokens.value.find((t) => t.metadata.id === p.spec.join_token)
      : defaultToken.value,
  }))
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
  <PageContainer class="flex h-full flex-col gap-6">
    <TSpinner v-if="loading" class="size-8 self-center" />

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
          <TableCell th>Join token</TableCell>
          <TableCell th>Date created</TableCell>
          <TableCell th>Action</TableCell>
        </TableRow>
      </template>

      <template #body>
        <TableRow v-for="{ token, tokenAutomatic, ...preset } in presetList" :key="itemID(preset)">
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
          <TableCell>
            <div class="flex items-center gap-2">
              {{ preset.spec.talos_version || DefaultTalosVersion }}
              <span v-if="!preset.spec.talos_version" class="resource-label label-green">
                Automatic
              </span>
            </div>
          </TableCell>
          <TableCell>
            <div class="flex items-center gap-2">
              <template v-if="token">
                {{ token.spec.name }}
              </template>

              <span v-else class="resource-label label-red flex items-center gap-1">
                <TIcon icon="warning" />
                Deleted
              </span>

              <span
                v-if="token?.spec.state === JoinTokenStatusSpecState.REVOKED"
                class="resource-label label-red flex items-center gap-1"
              >
                <TIcon icon="warning" />
                {{ TCommonStatuses.REVOKED }}
              </span>

              <span
                v-if="token?.spec.state === JoinTokenStatusSpecState.EXPIRED"
                class="resource-label label-red flex items-center gap-1"
              >
                <TIcon icon="warning" />
                {{ TCommonStatuses.EXPIRED }}
              </span>

              <span v-if="tokenAutomatic" class="resource-label label-green">Automatic</span>
            </div>
          </TableCell>
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
  </PageContainer>
</template>
