<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Disclosure, DisclosureButton, DisclosurePanel } from '@headlessui/vue'
import { DocumentIcon } from '@heroicons/vue/24/solid'
import { v4 as uuidv4 } from 'uuid'
import type { Ref } from 'vue'
import { computed, onMounted, ref, toRefs, watch } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import { useRoute, useRouter } from 'vue-router'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { ClusterSpec, ConfigPatchSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import {
  ClusterMachineStatusType,
  ClusterPermissionsType,
  ConfigPatchDescription,
  ConfigPatchName,
  ConfigPatchType,
  DefaultNamespace,
  LabelCluster,
  LabelClusterMachine,
  LabelHostname,
  LabelMachine,
  LabelMachineSet,
  VirtualNamespace,
} from '@/api/resources'
import { LabelSystemPatch } from '@/api/resources'
import type { WatchOptions } from '@/api/watch'
import Watch from '@/api/watch'
import IconButton from '@/components/common/Button/IconButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import TAlert from '@/components/TAlert.vue'
import { canManageMachineConfigPatches, canReadMachineConfigPatches } from '@/methods/auth'
import {
  controlPlaneTitle,
  defaultWorkersTitle,
  machineSetTitle,
  workersTitlePrefix,
} from '@/methods/machineset'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'

const route = useRoute()
const router = useRouter()
const filter = ref('')
const machineStatuses: Ref<Resource[]> = ref([])
const machineStatusesWatch = new Watch(machineStatuses)

type Props = {
  cluster?: Resource<ClusterSpec>
  machine?: Resource
}

const props = defineProps<Props>()

const { machine, cluster } = toRefs(props)

const selectors = computed<string[] | void>(() => {
  const res: string[] = []

  if (cluster.value) {
    res.push(`${LabelCluster}=${cluster.value.metadata.id}`)
  } else if (machine.value) {
    res.push(
      `${LabelMachine}=${machine.value.metadata.id}`,
      `${LabelClusterMachine}=${machine.value.metadata.id}`,
    )
  } else {
    return
  }

  return res.map((item) => item + `,!${LabelSystemPatch}`)
})

const patches: Ref<Resource<ConfigPatchSpec>[]> = ref([])
const patchesWatch = new Watch(patches)
const loading = computed(() => {
  return patchesWatch.loading.value || machineStatusesWatch.loading.value
})

patchesWatch.setup(
  computed<WatchOptions | undefined>(() => {
    if (!selectors.value) {
      return
    }

    return {
      runtime: Runtime.Omni,
      resource: { type: ConfigPatchType, namespace: DefaultNamespace },
      selectors: selectors.value,
      selectUsingOR: true,
    }
  }),
)

machineStatusesWatch.setup({
  runtime: Runtime.Omni,
  resource: { type: ClusterMachineStatusType, namespace: DefaultNamespace },
})

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

type item = { name: string; route: RouteLocationRaw; id: string; description?: string }

const patchTypeCluster = 'Cluster'

const routes = computed(() => {
  const hostnames: Record<string, string> = {}

  machineStatuses.value.forEach((item: Resource) => {
    hostnames[item.metadata.id!] = item.metadata.labels![LabelHostname]
  })

  const groups: Record<string, item[]> = {}

  const addToGroup = (name: string, r: item) => {
    groups[name] = (groups[name] ?? []).concat([r])
  }

  patches.value.forEach((item: Resource) => {
    const searchValues: string[] = [
      item.metadata.id!,
      (item.metadata.annotations || {})[ConfigPatchName],
      hostnames[(item.metadata.labels || {})[LabelClusterMachine]],
    ]

    if (filter.value !== '' && !includes(filter.value, searchValues)) {
      return
    }

    const patchEditPage = route.params.cluster ? 'ClusterMachinePatchEdit' : 'MachinePatchEdit'

    const r = {
      name: (item.metadata.annotations || {})[ConfigPatchName] || item.metadata.id!,
      icon: 'document',
      route: {
        name: machine?.value ? patchEditPage : 'ClusterPatchEdit',
        params: { patch: item.metadata.id! },
      },
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

      const title = machineSetTitle(route.params.cluster as string, id)

      addToGroup(`${title}`, r)
    } else if (labels[LabelCluster]) {
      addToGroup(`${patchTypeCluster}: ${labels[LabelCluster]}`, r)
    }
  })

  const result: { name: string; items: item[] }[] = []
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

const canReadConfigPatches = ref(false)
const canManageConfigPatches = ref(false)

const updatePermissions = async () => {
  if (cluster?.value) {
    const clusterPermissions = await ResourceService.Get(
      {
        namespace: VirtualNamespace,
        type: ClusterPermissionsType,
        id: cluster?.value.metadata.id,
      },
      withRuntime(Runtime.Omni),
    )

    canReadConfigPatches.value = clusterPermissions?.spec?.can_read_config_patches || false
    canManageConfigPatches.value = clusterPermissions?.spec?.can_manage_config_patches || false
  } else if (machine?.value) {
    canReadConfigPatches.value = canReadMachineConfigPatches.value
    canManageConfigPatches.value = canManageMachineConfigPatches.value
  }
}

const openPatchCreate = () => {
  if (!cluster.value && !machine.value) {
    return
  }

  router.push({
    name: cluster.value ? 'ClusterPatchEdit' : 'MachinePatchEdit',
    params: {
      patch: `500-${uuidv4()}`,
    },
  })
}

watch([() => machine.value, () => cluster.value], async () => {
  await updatePermissions()
})

onMounted(async () => {
  await updatePermissions()
})
</script>

<template>
  <div class="flex flex-col gap-4 overflow-y-auto">
    <ManagedByTemplatesWarning :cluster="cluster" />
    <div class="flex gap-4">
      <TInput v-model="filter" class="flex-1" placeholder="Search..." icon="search" />
      <TButton type="highlighted" :disabled="!canManageConfigPatches" @click="openPatchCreate"
        >Create Patch</TButton
      >
    </div>
    <div class="font-sm flex-1">
      <div v-if="loading" class="flex h-full w-full items-center justify-center">
        <TSpinner class="h-6 w-6" />
      </div>
      <TAlert v-else-if="patches.length === 0" title="No Config Patches" type="info">
        There are no config patches
        {{
          machine?.metadata.id
            ? `associated with machine ${machine.metadata.id}`
            : cluster?.metadata.id
              ? `associated with cluster ${cluster.metadata.id}`
              : `on the account`
        }}
      </TAlert>
      <Disclosure v-for="group in routes" v-else :key="group.name" as="div" :default-open="true">
        <template #default="{ open }">
          <DisclosureButton as="div" class="disclosure">
            <TIcon
              icon="arrow-up"
              class="absolute bottom-0 right-4 top-0 m-auto h-4 w-4"
              :class="{ 'rotate-180': open }"
            />
            <div class="grid grid-cols-4">
              <WordHighlighter
                :text-to-highlight="group.name"
                :query="filter"
                highlight-class="bg-naturals-N14"
              />
              <div>ID</div>
              <div class="col-span-2">Description</div>
            </div>
          </DisclosureButton>
          <DisclosurePanel>
            <div
              v-for="item in group.items"
              :key="item.name"
              class="relative my-2 grid w-full cursor-pointer grid-cols-4 items-center gap-2 px-4 py-2 text-xs transition-colors duration-200 hover:bg-naturals-N3 hover:text-naturals-N12"
              @click="() => $router.push(item.route)"
            >
              <IconButton
                :disabled="!canManageConfigPatches"
                icon="delete"
                class="absolute bottom-0 right-3 top-0 m-auto h-4 w-4"
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
                  highlight-class="bg-naturals-N14"
                />
              </div>
              <WordHighlighter
                :text-to-highlight="item.id"
                :query="filter"
                highlight-class="pointer-events-none bg-naturals-N14"
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
.disclosure {
  @apply relative cursor-pointer select-none bg-naturals-N1 px-4 py-3 text-xs font-bold text-naturals-N11 transition-colors duration-200 hover:text-naturals-N14;
}
</style>
