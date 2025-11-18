<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import type { InfraProviderStatusSpec } from '@/api/omni/specs/infra.pb'
import type { MachineStatusMetricsSpec } from '@/api/omni/specs/omni.pb'
import {
  EphemeralNamespace,
  InfraProviderNamespace,
  InfraProviderStatusType,
  LabelInfraProviderID,
  LabelIsManagedByStaticInfraProvider,
  LabelMachineRequest,
  LabelsCompletionType,
  MachineStatusLinkType,
  MachineStatusMetricsID,
  MachineStatusMetricsType,
  MachineStatusType,
  MetricsNamespace,
  VirtualNamespace,
} from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TList from '@/components/common/List/TList.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatsItem from '@/components/common/Stats/StatsItem.vue'
import { useWatch } from '@/components/common/Watch/useWatch'
import TAlert from '@/components/TAlert.vue'
import { getDocsLink } from '@/methods'
import type { Label } from '@/methods/labels'
import { addLabel, selectors as labelsToSelectors } from '@/methods/labels'
import { MachineFilterOption } from '@/methods/machine'
import LabelsInput from '@/views/omni/ItemLabels/LabelsInput.vue'
import MachineDetailsPanel from '@/views/omni/Machines/MachineDetailsPanel.vue'
import MachineItem from '@/views/omni/Machines/MachineItem.vue'

const { filter = undefined } = defineProps<{
  filter?: MachineFilterOption
}>()

const route = useRoute()
const router = useRouter()

const { data: infraProviderStatuses } = useWatch<InfraProviderStatusSpec>({
  resource: {
    type: InfraProviderStatusType,
    namespace: InfraProviderNamespace,
  },
  runtime: Runtime.Omni,
})

const selectors = computed(() => {
  const selectors: string[] = []

  switch (filter) {
    case MachineFilterOption.Manual:
      selectors.push(`!${LabelMachineRequest},!${LabelIsManagedByStaticInfraProvider}`)
      break
    case MachineFilterOption.Managed:
      selectors.push(`${LabelMachineRequest}`, `${LabelIsManagedByStaticInfraProvider}`)
      break
  }

  if (route.params.provider) {
    selectors.push(`${LabelInfraProviderID}=${route.params.provider}`)
  }

  const q = labelsToSelectors(filterLabels.value)?.join(',')

  if (!q) return selectors
  if (!selectors.length) return [q]

  return selectors.map((item) => `${item},${q}`)
})

const getCapacity = (item?: Resource<MachineStatusMetricsSpec>) => {
  const registered = item?.spec.registered_machines_count ?? 0
  if (registered === 0) {
    return 0
  }

  const allocated = item?.spec.allocated_machines_count ?? 0
  const free = registered - allocated

  return parseInt(((free / registered) * 100).toFixed(0))
}

const sortOptions = [
  { id: 'machine_created_at', desc: 'Creation Time ⬆', descending: true },
  { id: 'machine_created_at', desc: 'Creation Time ⬇' },
  { id: 'hostname', desc: 'Hostname ⬆' },
  { id: 'hostname', desc: 'Hostname ⬇', descending: true },
  { id: 'id', desc: 'ID ⬆' },
  { id: 'id', desc: 'ID ⬇', descending: true },
  { id: 'last_alive', desc: 'Last Active Time ⬆', descending: true },
  { id: 'last_alive', desc: 'Last Active Time ⬇' },
]

const filterLabels = ref<Label[]>([])
const filterValue = ref('')

const docsLink = getDocsLink('omni', '/explanation/infrastructure-providers')
const openDocs = () => window.open(docsLink, '_blank')?.focus()

const { data: machineStatusMetrics } = useWatch<MachineStatusMetricsSpec>({
  resource: {
    type: MachineStatusMetricsType,
    id: MachineStatusMetricsID,
    namespace: EphemeralNamespace,
  },
  runtime: Runtime.Omni,
})

function deleteItems() {
  const machines = [...selectedMachines.value.keys()]
  const clusters = [...selectedMachines.value.values()]
    .map((m) => m.spec.message_status?.cluster)
    .filter((m) => typeof m === 'string')

  router.push({
    query: {
      modal: 'machineRemove',
      machine: machines,
      cluster: [...new Set(clusters)],
    },
  })
}

const selectedMachines = ref<Map<string, Resource<MachineStatusLinkSpec>>>(new Map())
function updateSelected(machine: Resource<MachineStatusLinkSpec>, v?: boolean) {
  const id = machine.metadata.id ?? ''

  if (v) {
    selectedMachines.value.set(id, machine)
  } else {
    selectedMachines.value.delete(id)
  }
}
</script>

<template>
  <TList
    :opts="{
      runtime: Runtime.Omni,
      resource: {
        type: MachineStatusLinkType,
        namespace: MetricsNamespace,
      },
      selectors,
      selectUsingOR: true,
    }"
    search
    pagination
    :sort-options="sortOptions"
    :filter-value="filterValue"
  >
    <template #norecords>
      <TAlert
        v-if="filter === MachineFilterOption.Managed && infraProviderStatuses.length === 0"
        type="info"
        title="No Infrastructure Providers Connected"
      >
        <div class="flex gap-1">
          Check the
          <TButton type="subtle" size="xs" @click="openDocs">documentation</TButton>
          on how to configure and use infrastructure providers.
        </div>
      </TAlert>

      <TAlert
        v-else-if="filter === MachineFilterOption.Manual"
        type="info"
        title="No Machines Found"
      >
        <div class="flex gap-1">
          Download and boot the
          <TButton
            type="subtle"
            size="xs"
            @click="$router.push({ name: 'Home', query: { modal: 'downloadInstallationMedia' } })"
          >
            installation media
          </TButton>
          to connect machines to your Omni instance.
        </div>
      </TAlert>

      <TAlert v-else type="info" title="No Machines Found">
        <div class="flex gap-1">
          No entries of the requested resource type are found on the server.
        </div>
      </TAlert>
    </template>

    <template #header="{ itemsCount, filtered }">
      <div v-if="!filter" class="flex flex-wrap items-center gap-x-6 gap-y-2">
        <h1 class="text-xl font-medium text-naturals-n14 max-md:basis-full">Machines</h1>

        <StatsItem title="Total" :value="itemsCount" icon="nodes" />

        <template v-if="!filtered">
          <StatsItem
            title="Allocated"
            :value="machineStatusMetrics?.spec.allocated_machines_count ?? 0"
            icon="arrow-right-square"
          />

          <StatsItem title="Capacity" :value="`${getCapacity(machineStatusMetrics)}%`" icon="box" />
        </template>
      </div>

      <PageHeader
        v-else-if="filter === MachineFilterOption.Manual"
        title="Manually Joined Machines"
      />

      <PageHeader
        v-else-if="filter === MachineFilterOption.Managed"
        title="Machines Managed by the Infrastructure Providers"
      />

      <PageHeader
        v-else-if="$route.params.provider"
        :title="`Machines Managed by the Infrastructure Provider ${$route.params.provider}`"
      />
    </template>

    <template #input>
      <LabelsInput
        v-model:filter-labels="filterLabels"
        v-model:filter-value="filterValue"
        :completions-resource="{
          id: MachineStatusType,
          type: LabelsCompletionType,
          namespace: VirtualNamespace,
        }"
        class="w-full"
        placeholder="Search ..."
      />
    </template>

    <template #extra-controls>
      <TButton type="primary" icon="delete" :disabled="!selectedMachines.size" @click="deleteItems">
        <span class="contents max-md:hidden">Delete selected</span>
      </TButton>
    </template>

    <template #default="{ items, searchQuery, sidePanelOpen, sidePanelSelectedItemId, openPanel }">
      <MachineItem
        v-for="item in items"
        :key="item.metadata.id"
        :machine="item"
        :search-query="searchQuery"
        :panel-open="sidePanelOpen && item.metadata.id === sidePanelSelectedItemId"
        :selected="selectedMachines.has(item.metadata.id ?? '')"
        @update:selected="(v) => updateSelected(item, v)"
        @open-panel="openPanel(item.metadata.id ?? '')"
        @filter-labels="(label) => addLabel(filterLabels, label)"
      />
    </template>

    <template #sidePanel="{ items, searchQuery, sidePanelSelectedItemId, closePanel }">
      <MachineDetailsPanel
        :machine="items.find((i) => i.metadata.id === sidePanelSelectedItemId)"
        :search-query="searchQuery"
        class="h-full"
        @close="closePanel"
      />
    </template>
  </TList>
</template>
