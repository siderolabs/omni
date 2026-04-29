<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Disclosure, DisclosureButton, DisclosurePanel } from '@headlessui/vue'
import { DocumentIcon } from '@heroicons/vue/24/solid'
import { v4 as uuidv4 } from 'uuid'
import { computed, ref } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import type {
  ClusterMachineStatusSpec,
  ClusterSpec,
  ConfigPatchSpec,
  MachineStatusSpec,
} from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineStatusType,
  ClusterType,
  ConfigPatchDescription,
  ConfigPatchName,
  ConfigPatchType,
  DefaultNamespace,
  LabelCluster,
  LabelClusterMachine,
  LabelHostname,
  LabelMachine,
  LabelMachineSet,
  LabelSystemPatch,
  MachineStatusType,
} from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import TButton from '@/components/Button/TButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import { useClusterPermissions, usePermissions } from '@/methods/auth'
import {
  controlPlaneTitle,
  defaultWorkersTitle,
  machineSetTitle,
  workersTitlePrefix,
} from '@/methods/machineset'
import { useResourceWatch } from '@/methods/useResourceWatch'

const filter = ref('')

type Props = {
  cluster?: string
  machine?: string
}

const { machine: machineId, cluster: propClusterId } = defineProps<Props>()

const { data: machine } = useResourceWatch<MachineStatusSpec>(() => ({
  skip: !machineId,
  resource: {
    namespace: DefaultNamespace,
    type: MachineStatusType,
    id: machineId!,
  },
  runtime: Runtime.Omni,
}))

const clusterId = computed(() => machine.value?.metadata.labels?.[LabelCluster] || propClusterId)

const { data: cluster } = useResourceWatch<ClusterSpec>(() => ({
  skip: !clusterId.value,
  resource: {
    type: ClusterType,
    namespace: DefaultNamespace,
    id: clusterId.value!,
  },
  runtime: Runtime.Omni,
}))

const { canManageMachineConfigPatches } = usePermissions()
const { canManageConfigPatches: canManageClusterMachineConfigPatches } =
  useClusterPermissions(clusterId)

const selectors = computed(() => {
  if (!clusterId.value && !machineId) return

  const res: string[] = []

  if (clusterId.value) {
    res.push(`${LabelCluster}=${clusterId.value}`)
  }

  if (machineId) {
    res.push(`${LabelMachine}=${machineId}`, `${LabelClusterMachine}=${machineId}`)
  }

  return res.map((item) => item + `,!${LabelSystemPatch}`)
})

const { data: patches, loading: patchesLoading } = useResourceWatch<ConfigPatchSpec>(() => ({
  skip: !selectors.value,
  runtime: Runtime.Omni,
  resource: {
    type: ConfigPatchType,
    namespace: DefaultNamespace,
  },
  selectors: selectors.value,
  selectUsingOR: true,
}))

const { data: machineStatuses, loading: machineStatusesLoading } =
  useResourceWatch<ClusterMachineStatusSpec>(() => ({
    skip: !clusterId.value,
    runtime: Runtime.Omni,
    resource: {
      type: ClusterMachineStatusType,
      namespace: DefaultNamespace,
    },
    selectors: [`${LabelCluster}=${clusterId.value}`],
  }))

const loading = computed(() => patchesLoading.value || machineStatusesLoading.value)

const includes = (filter: string, values: string[]) => {
  for (const value of values) {
    if (!value) {
      continue
    }

    if (value.indexOf(filter) !== -1) {
      return true
    }
  }

  return false
}

interface RouteItem {
  name: string
  route: RouteLocationRaw
  id: string
  description?: string
}

const patchTypeCluster = 'Cluster'

function getRouteForPatch(patch = `500-${uuidv4()}`): RouteLocationRaw {
  if (clusterId.value && machineId) {
    return {
      name: 'ClusterMachinePatchEdit',
      params: {
        cluster: clusterId.value,
        machine: machineId,
        patch,
      },
    }
  }

  if (clusterId.value) {
    return {
      name: 'ClusterPatchEdit',
      params: {
        cluster: clusterId.value,
        patch,
      },
    }
  }

  if (machineId) {
    return {
      name: 'MachinePatchEdit',
      params: {
        machine: machineId,
        patch,
      },
    }
  }

  throw new Error('Unable to determine patch path')
}

const routes = computed(() => {
  const hostnames: Record<string, string> = {}

  if (machine.value?.spec.network?.hostname) {
    hostnames[machine.value.metadata.id!] = machine.value.spec.network.hostname
  }

  machineStatuses.value.forEach((item) => {
    hostnames[item.metadata.id!] = item.metadata.labels![LabelHostname]
  })

  const groups: Record<string, RouteItem[]> = {}

  const addToGroup = (name: string, r: RouteItem) => {
    groups[name] = (groups[name] ?? []).concat([r])
  }

  patches.value.forEach((item) => {
    const searchValues: string[] = [
      item.metadata.id!,
      (item.metadata.annotations || {})[ConfigPatchName],
      hostnames[(item.metadata.labels || {})[LabelClusterMachine]],
    ]

    if (filter.value !== '' && !includes(filter.value, searchValues)) {
      return
    }

    const r: RouteItem = {
      name: item.metadata.annotations?.[ConfigPatchName] || item.metadata.id!,
      route: getRouteForPatch(item.metadata.id),
      id: item.metadata.id!,
      description: item.metadata.annotations?.[ConfigPatchDescription],
    }

    const labels = item.metadata.labels || {}
    const machineID = labels[LabelMachine]
    if (machineID) {
      addToGroup(`Machine: ${machineID}`, r)
    } else if (labels[LabelClusterMachine]) {
      const id = labels[LabelClusterMachine]
      addToGroup(`Cluster Machine: ${hostnames[id] || id}`, r)
    } else if (labels[LabelMachineSet]) {
      const id = labels[LabelMachineSet]

      const title = machineSetTitle(clusterId.value, id)

      addToGroup(`${title}`, r)
    } else if (labels[LabelCluster]) {
      addToGroup(`${patchTypeCluster}: ${labels[LabelCluster]}`, r)
    }
  })

  const result: { name: string; items: RouteItem[] }[] = []
  for (const key in groups) {
    result.push({ name: key, items: groups[key] })
  }

  const clusterPrefix = `${patchTypeCluster}: `

  const categoryIndex = (name: string) => {
    if (name.startsWith(clusterPrefix)) {
      return 0
    }

    if (name === controlPlaneTitle) {
      return 1
    }

    if (name === defaultWorkersTitle) {
      return 2
    }

    if (name.startsWith(workersTitlePrefix)) {
      return 3
    }

    return 4
  }

  result.sort((a, b): number => {
    const categoryA = categoryIndex(a.name)
    const categoryB = categoryIndex(b.name)

    if (categoryA !== categoryB) {
      return categoryA - categoryB
    }

    return a.name.localeCompare(b.name)
  })

  return result
})

const canManageConfigPatches = computed(() => {
  if (clusterId.value) return canManageClusterMachineConfigPatches.value
  if (machineId) return canManageMachineConfigPatches.value

  return false
})
</script>

<template>
  <div class="flex flex-col gap-4 overflow-y-auto">
    <ManagedByTemplatesWarning :resource="cluster" />

    <div class="flex gap-4">
      <TInput v-model="filter" class="flex-1" placeholder="Search..." icon="search" />
      <TButton
        is="router-link"
        variant="highlighted"
        :disabled="!canManageConfigPatches"
        :to="getRouteForPatch()"
      >
        Create Patch
      </TButton>
    </div>
    <div class="font-sm flex-1">
      <div v-if="loading" class="flex h-full w-full items-center justify-center">
        <TSpinner class="h-6 w-6" />
      </div>
      <TAlert v-else-if="patches.length === 0" title="No Config Patches" type="info">
        There are no config patches
        {{
          machineId
            ? `associated with machine ${machineId}`
            : clusterId
              ? `associated with cluster ${clusterId}`
              : `on the account`
        }}
      </TAlert>
      <Disclosure v-for="group in routes" v-else :key="group.name" as="div" :default-open="true">
        <template #default="{ open }">
          <DisclosureButton as="div" class="disclosure">
            <TIcon
              icon="arrow-up"
              class="absolute top-0 right-4 bottom-0 m-auto h-4 w-4"
              :class="{ 'rotate-180': open }"
            />
            <div class="grid grid-cols-4">
              <WordHighlighter
                :text-to-highlight="group.name"
                :query="filter"
                highlight-class="bg-naturals-n14"
              />
              <div>ID</div>
              <div class="col-span-2">Description</div>
            </div>
          </DisclosureButton>
          <DisclosurePanel>
            <div
              v-for="item in group.items"
              :key="item.name"
              class="relative my-2 grid w-full cursor-pointer grid-cols-4 items-center gap-2 px-4 py-2 text-xs transition-colors duration-200 hover:bg-naturals-n3 hover:text-naturals-n12"
              @click="() => $router.push(item.route)"
            >
              <IconButton
                :disabled="!canManageConfigPatches"
                icon="delete"
                class="absolute top-0 right-3 bottom-0 m-auto"
                @click.stop="
                  () => {
                    $router.push({ query: { modal: 'configPatchDestroy', id: item.id } })
                  }
                "
              />
              <div class="pointer-events-none flex items-center gap-4">
                <DocumentIcon class="h-4 w-4" />
                <WordHighlighter
                  :text-to-highlight="item.name"
                  :query="filter"
                  highlight-class="bg-naturals-n14"
                />
              </div>
              <WordHighlighter
                :text-to-highlight="item.id"
                :query="filter"
                highlight-class="pointer-events-none bg-naturals-n14"
              />
              <div class="pointer-events-none col-span-2 truncate">
                {{ item.description }}
              </div>
            </div>
          </DisclosurePanel>
        </template>
      </Disclosure>
    </div>
  </div>
</template>

<style scoped>
@reference "../../index.css";

.disclosure {
  @apply relative cursor-pointer bg-naturals-n1 px-4 py-3 text-xs font-bold text-naturals-n11 transition-colors duration-200 select-none hover:text-naturals-n14;
}
</style>
