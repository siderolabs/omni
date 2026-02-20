<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type {
  ExtensionsConfigurationSpec,
  MachineExtensionsSpec,
  MachineExtensionsStatusSpec,
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
} from '@/api/resources'
import type { WatchOptions } from '@/api/watch'
import Watch from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TListItem from '@/components/common/List/TListItem.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import TAlert from '@/components/TAlert.vue'
import { setupClusterPermissions } from '@/methods/auth'

const machineExtensionsStatus = ref<Resource<MachineExtensionsStatusSpec>>()
const machineExtensionsStatusWatch = new Watch(machineExtensionsStatus)
const searchString = ref('')
const router = useRouter()
const route = useRoute()

const machineExtensionsStatusWatchLoading = machineExtensionsStatusWatch.loading

const ready = computed(() => {
  return !machineExtensionsStatusWatchLoading.value
})

const { canUpdateTalos } = setupClusterPermissions(computed(() => route.params.cluster as string))

machineExtensionsStatusWatch.setup(
  computed((): WatchOptions | undefined => {
    return {
      resource: {
        namespace: DefaultNamespace,
        type: MachineExtensionsStatusType,
        id: route.params.machine as string,
      },
      runtime: Runtime.Omni,
    }
  }),
)

const machineExtensions = ref<Resource<MachineExtensionsSpec>>()

const machineExtensionsWatch = new Watch(machineExtensions)

machineExtensionsWatch.setup(
  computed(() => {
    if (!machineExtensionsStatus.value) {
      return
    }

    return {
      resource: {
        id: machineExtensionsStatus.value.metadata.id!,
        namespace: DefaultNamespace,
        type: MachineExtensionsType,
      },
      runtime: Runtime.Omni,
    }
  }),
)

const extensionsConfiguration = ref<Resource<ExtensionsConfigurationSpec>>()

const configurationsWatch = new Watch(extensionsConfiguration)

configurationsWatch.setup(
  computed((): WatchOptions | undefined => {
    const id = machineExtensions.value?.metadata.labels?.[ExtensionsConfigurationLabel]
    if (!id) {
      return
    }

    return {
      resource: {
        namespace: DefaultNamespace,
        type: ExtensionsConfigurationType,
        id: id,
      },
      runtime: Runtime.Omni,
    }
  }),
)

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

const openExtensionsUpdate = () => {
  router.push({
    query: {
      machine: route.params.machine as string,
      modal: 'updateExtensions',
    },
  })
}
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="flex grow flex-col gap-4 overflow-y-auto px-4 py-4 md:px-6">
      <TInput v-model="searchString" icon="search" />
      <div class="flex flex-1 flex-col overflow-y-auto">
        <template v-if="ready && extensionsState.length > 0">
          <div
            class="mb-1 grid grid-cols-3 items-center justify-center bg-naturals-n2 px-6 py-2 text-xs"
          >
            <div>Name</div>
            <div>State</div>
            <div>Level</div>
          </div>
          <TListItem v-for="item in extensionsState" :key="item.name">
            <div class="flex gap-2 px-3">
              <div class="grid flex-1 grid-cols-3 items-center justify-center text-naturals-n12">
                <WordHighlighter
                  :query="searchString"
                  :text-to-highlight="item.name"
                  highlight-class="bg-naturals-n14"
                  class="text-naturals-n14"
                />
                <div class="flex">
                  <div
                    class="flex items-center gap-2 rounded bg-naturals-n3 px-2 py-1 text-xs text-naturals-n13"
                  >
                    <template v-if="item.phase === MachineExtensionsStatusSpecItemPhase.Installing">
                      <TIcon icon="loading" class="h-4 w-4 animate-spin text-yellow-y1" />
                      <span>Installing</span>
                    </template>
                    <template
                      v-else-if="item.phase === MachineExtensionsStatusSpecItemPhase.Removing"
                    >
                      <TIcon icon="delete" class="h-4 w-4 animate-pulse text-red-r1" />
                      <span>Removing</span>
                    </template>

                    <template
                      v-else-if="item.phase === MachineExtensionsStatusSpecItemPhase.Installed"
                    >
                      <TIcon icon="check-in-circle" class="h-4 w-4 text-green-g1" />
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
          <TAlert title="No Extensions Found" type="info" />
        </div>
      </div>
    </div>

    <div
      class="flex h-16 shrink-0 items-center justify-end border-t border-naturals-n5 bg-naturals-n1 px-12"
    >
      <TButton variant="highlighted" :disabled="!canUpdateTalos" @click="openExtensionsUpdate">
        Update Extensions
      </TButton>
    </div>
  </div>
</template>
