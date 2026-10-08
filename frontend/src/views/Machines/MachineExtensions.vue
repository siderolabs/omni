<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRouteQuery } from '@vueuse/router'
import { computed, ref } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import type {
  ExtensionsConfigurationSpec,
  MachineExtensionsSpec,
  MachineExtensionsStatusSpec,
  MachineStatusSpec,
} from '@/api/omni/specs/omni.pb'
import { MachineExtensionsStatusSpecItemPhase } from '@/api/omni/specs/omni.pb'
import {
  DefaultNamespace,
  ExtensionsConfigurationLabel,
  ExtensionsConfigurationType,
  LabelCluster,
  LabelClusterMachine,
  LabelMachineSet,
  MachineExtensionsStatusType,
  MachineExtensionsType,
  MachineStatusType,
} from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import TListItem from '@/components/List/TListItem.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import { useClusterPermissions } from '@/methods/auth'
import { useResourceWatch } from '@/methods/useResourceWatch'
import UpdateExtensionsModal from '@/views/Machines/components/UpdateExtensionsModal.vue'

const { clusterId, machineId, readOnly } = defineProps<{
  clusterId?: string
  machineId: string
  readOnly?: boolean
}>()

const { data: machineExtensionsStatus, loading: machineExtensionsStatusWatchLoading } =
  useResourceWatch<MachineExtensionsStatusSpec>(() => ({
    resource: {
      namespace: DefaultNamespace,
      type: MachineExtensionsStatusType,
      id: machineId,
    },
    runtime: Runtime.Omni,
  }))

const updateExtensionsModalOpen = ref(false)
const searchString = useRouteQuery('q', '')

const ready = computed(() => {
  return !machineExtensionsStatusWatchLoading.value
})

const { canUpdateTalos } = useClusterPermissions(() => clusterId)

const { data: machineStatus } = useResourceWatch<MachineStatusSpec>(() => ({
  resource: {
    id: machineId,
    namespace: DefaultNamespace,
    type: MachineStatusType,
  },
  runtime: Runtime.Omni,
}))

const invalidSchematic = computed(() => machineStatus.value?.spec.schematic?.invalid === true)

const { data: machineExtensions } = useResourceWatch<MachineExtensionsSpec>(() => {
  const id = machineExtensionsStatus.value?.metadata.id

  return {
    skip: !id,
    resource: {
      id: id!,
      namespace: DefaultNamespace,
      type: MachineExtensionsType,
    },
    runtime: Runtime.Omni,
  }
})

const { data: extensionsConfiguration } = useResourceWatch<ExtensionsConfigurationSpec>(() => {
  const id = machineExtensions.value?.metadata.labels?.[ExtensionsConfigurationLabel]

  return {
    skip: !id,
    resource: {
      namespace: DefaultNamespace,
      type: ExtensionsConfigurationType,
      id: id!,
    },
    runtime: Runtime.Omni,
  }
})

const extensionsState = computed(() => {
  if (!machineExtensionsStatus.value?.spec.extensions) {
    return []
  }

  type Item = {
    name: string
    source: string | null
    phase: MachineExtensionsStatusSpecItemPhase
  }

  const res: Item[] = []

  for (const extension of machineExtensionsStatus.value.spec.extensions) {
    if (searchString.value !== '' && !extension.name?.includes(searchString.value)) {
      continue
    }

    res.push({
      name: extension.name!,
      source: extension.immutable ? 'Automatically installed by Omni' : extensionsLevel.value,
      phase: extension.phase ?? MachineExtensionsStatusSpecItemPhase.Installed,
    })
  }

  return res
})

const extensionsLevel = computed(() => {
  if (!extensionsConfiguration.value) {
    return null
  }

  for (const label of [LabelClusterMachine, LabelMachineSet, LabelCluster]) {
    const value = extensionsConfiguration.value.metadata.labels?.[label]
    if (value) {
      switch (label) {
        case LabelClusterMachine:
          return `Defined for the machine ${value}`
        case LabelMachineSet:
          return `Inherited from the machine set ${value}`
        case LabelCluster:
          return `Inherited from the cluster ${value}`
      }
    }
  }

  return null
})
</script>

<template>
  <div class="flex h-full flex-col">
    <PageContainer class="flex grow flex-col gap-compact overflow-y-auto">
      <TInput v-model="searchString" icon="search" />
      <div class="flex flex-1 flex-col overflow-y-auto">
        <template v-if="ready && extensionsState.length > 0">
          <div
            class="mb-micro grid grid-cols-3 items-center justify-center bg-surface-card px-base py-tight text-xs"
          >
            <div>Name</div>
            <div>State</div>
            <div>Level</div>
          </div>
          <TListItem v-for="item in extensionsState" :key="item.name">
            <div class="flex gap-tight px-snug">
              <div class="grid flex-1 grid-cols-3 items-center justify-center text-content-default">
                <WordHighlighter
                  :query="searchString"
                  :text-to-highlight="item.name"
                  highlight-class="search-match"
                  class="text-content-emphasis"
                />
                <div class="flex">
                  <div
                    class="flex items-center gap-tight rounded bg-surface-raised px-tight py-micro text-xs text-content-default"
                  >
                    <template v-if="item.phase === MachineExtensionsStatusSpecItemPhase.Installing">
                      <TIcon
                        icon="loading"
                        class="h-4 w-4 animate-spin text-status-warning-default"
                      />
                      <span>Installing</span>
                    </template>
                    <template
                      v-else-if="item.phase === MachineExtensionsStatusSpecItemPhase.Removing"
                    >
                      <TIcon
                        icon="delete"
                        class="h-4 w-4 animate-pulse text-status-danger-default"
                      />
                      <span>Removing</span>
                    </template>

                    <template
                      v-else-if="item.phase === MachineExtensionsStatusSpecItemPhase.Installed"
                    >
                      <TIcon icon="check-circle" class="h-4 w-4 text-status-success-default" />
                      <span>Installed</span>
                    </template>
                  </div>
                </div>
                <div v-if="item.source">
                  {{ item.source }}
                </div>
              </div>
            </div>
          </TListItem>
        </template>
        <div v-else-if="!ready" class="flex flex-1 items-center justify-center">
          <TSpinner class="h-6 w-6" />
        </div>
        <div v-else class="flex-1">
          <TAlert v-if="invalidSchematic" title="Non-factory Machine" type="warn">
            This machine was not provisioned using an image factory image. Extensions cannot be
            managed by Omni for this machine.
          </TAlert>
          <TAlert v-else title="No Extensions Found" type="info" />
        </div>
      </div>
    </PageContainer>

    <div
      v-if="!readOnly"
      class="flex h-16 shrink-0 items-center justify-end border-t border-border-strong bg-surface-chrome px-12"
    >
      <TButton
        variant="highlighted"
        :disabled="!canUpdateTalos || invalidSchematic"
        @click="updateExtensionsModalOpen = true"
      >
        Update Extensions
      </TButton>
    </div>

    <UpdateExtensionsModal
      v-if="clusterId && !readOnly"
      v-model:open="updateExtensionsModalOpen"
      :cluster-id="clusterId"
      :machine-id="machineId"
    />
  </div>
</template>
