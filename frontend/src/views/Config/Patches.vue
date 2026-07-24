<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Disclosure, DisclosureButton, DisclosurePanel } from '@headlessui/vue'
import { DocumentIcon } from '@heroicons/vue/24/solid'
import { useRouteQuery } from '@vueuse/router'
import { v4 as uuidv4 } from 'uuid'
import { computed, ref } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import type { ClusterMachineStatusSpec, ConfigPatchSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineStatusType,
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
} from '@/api/resources'
import TActionsBox from '@/components/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/ActionsBox/TActionsBoxItem.vue'
import TButton from '@/components/Button/TButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TStatus from '@/components/Status/TStatus.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import { TCommonStatuses } from '@/constants'
import { useClusterPermissions, usePermissions } from '@/methods/auth'
import { isConfigPatchDisabled, setConfigPatchDisabled } from '@/methods/config-patch'
import {
  controlPlaneTitle,
  defaultWorkersTitle,
  machineSetTitle,
  workersTitlePrefix,
} from '@/methods/machineset'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError } from '@/notification'
import ConfigPatchDestroyModal from '@/views/Config/components/ConfigPatchDestroyModal.vue'

const { machine, cluster } = defineProps<{
  cluster?: string
  machine?: string
}>()

const filter = useRouteQuery('q', '')
const configPatchDestroyModalOpen = ref(false)
const configPatchDestroyModalPatchId = ref<string>()

const { canManageMachineConfigPatches } = usePermissions()
const { canManageConfigPatches: canManageClusterMachineConfigPatches } = useClusterPermissions(
  () => cluster,
)

const selectors = computed(() => {
  const res: string[] = []

  if (machine) {
    res.push(`${LabelMachine}=${machine}`, `${LabelClusterMachine}=${machine}`)
  } else if (cluster) {
    res.push(`${LabelCluster}=${cluster}`)
  } else {
    return
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
  useResourceWatch<ClusterMachineStatusSpec>({
    runtime: Runtime.Omni,
    resource: {
      type: ClusterMachineStatusType,
      namespace: DefaultNamespace,
    },
  })

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
  disabled: boolean
}

const patchTypeCluster = 'Cluster'

function getRouteForPatch(patch = `500-${uuidv4()}`): RouteLocationRaw {
  if (cluster && machine) {
    return {
      name: 'ClusterMachinePatchEdit',
      params: {
        cluster,
        machine,
        patch,
      },
    }
  }

  if (cluster) {
    return {
      name: 'ClusterPatchEdit',
      params: {
        cluster,
        patch,
      },
    }
  }

  if (machine) {
    return {
      name: 'MachinePatchEdit',
      params: {
        machine,
        patch,
      },
    }
  }

  throw new Error('Unable to determine patch path')
}

const routes = computed(() => {
  const hostnames: Record<string, string> = {}

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
      disabled: isConfigPatchDisabled(item.metadata.labels),
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

      const title = machineSetTitle(cluster, id)

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
  if (cluster) return canManageClusterMachineConfigPatches.value
  if (machine) return canManageMachineConfigPatches.value

  return false
})

const togglingPatchId = ref<string>()

const toggleDisabled = async (item: RouteItem) => {
  togglingPatchId.value = item.id

  try {
    await setConfigPatchDisabled(item.id, !item.disabled)
  } catch (e) {
    showError(
      `Failed to ${item.disabled ? 'Enable' : 'Disable'} The Config Patch`,
      e instanceof Error ? e.message : String(e),
    )
  } finally {
    togglingPatchId.value = undefined
  }
}
</script>

<template>
  <div class="flex flex-col gap-4 overflow-y-auto">
    <ManagedByTemplatesWarning />

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
          machine
            ? `associated with machine ${machine}`
            : cluster
              ? `associated with cluster ${cluster}`
              : `on the account`
        }}
      </TAlert>
      <Disclosure
        v-for="group in routes"
        v-else
        :key="group.name"
        as="div"
        :default-open="true"
        class="[--actions-col:--spacing(6)]"
      >
        <template #default="{ open }">
          <DisclosureButton
            as="div"
            class="grid cursor-pointer grid-cols-[repeat(4,1fr)_var(--actions-col)] gap-4 bg-naturals-n1 px-4 py-3 text-xs font-bold text-naturals-n11 transition-colors duration-200 select-none hover:text-naturals-n14"
          >
            <WordHighlighter
              :text-to-highlight="group.name"
              :query="filter"
              highlight-class="bg-naturals-n14"
            />

            <div>ID</div>
            <div class="col-span-2">Description</div>

            <TIcon
              icon="arrow-up"
              class="size-4 justify-self-center"
              :class="{ 'rotate-180': open }"
            />
          </DisclosureButton>

          <DisclosurePanel>
            <div
              v-for="item in group.items"
              :key="item.name"
              class="my-1 grid w-full cursor-pointer grid-cols-[repeat(4,1fr)_var(--actions-col)] items-center gap-2 px-4 py-2 text-xs transition-colors duration-200 select-none hover:bg-naturals-n3 hover:text-naturals-n12"
              :class="{ 'opacity-50': item.disabled }"
              @click="() => $router.push(item.route)"
            >
              <div class="flex min-w-0 items-center gap-4">
                <DocumentIcon class="size-4 shrink-0" />

                <WordHighlighter
                  :text-to-highlight="item.name"
                  :query="filter"
                  highlight-class="bg-naturals-n14"
                  class="truncate"
                />

                <TStatus v-if="item.disabled" :title="TCommonStatuses.DISABLED" />
              </div>

              <WordHighlighter
                :text-to-highlight="item.id"
                :query="filter"
                highlight-class="bg-naturals-n14"
              />

              <div class="col-span-2 truncate">
                {{ item.description }}
              </div>

              <TActionsBox aria-label="patch actions">
                <TActionsBoxItem
                  :icon="item.disabled ? 'check-in-circle-classic' : 'no-symbol'"
                  @select="toggleDisabled(item)"
                >
                  {{ item.disabled ? 'Enable' : 'Disable' }}
                </TActionsBoxItem>

                <TActionsBoxItem
                  danger
                  icon="delete"
                  @select="
                    () => {
                      configPatchDestroyModalOpen = true
                      configPatchDestroyModalPatchId = item.id
                    }
                  "
                >
                  Delete
                </TActionsBoxItem>
              </TActionsBox>
            </div>
          </DisclosurePanel>
        </template>
      </Disclosure>
    </div>

    <ConfigPatchDestroyModal
      v-if="configPatchDestroyModalPatchId"
      v-model:open="configPatchDestroyModalOpen"
      :patch-id="configPatchDestroyModalPatchId"
    />
  </div>
</template>
