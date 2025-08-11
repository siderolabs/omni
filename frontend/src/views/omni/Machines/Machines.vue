<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { toRefs } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
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
import type { WatchOptions } from '@/api/watch'
import { itemID } from '@/api/watch'
import WatchResource from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TList from '@/components/common/List/TList.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatsItem from '@/components/common/Stats/StatsItem.vue'
import Watch from '@/components/common/Watch/Watch.vue'
import TAlert from '@/components/TAlert.vue'
import type { Label } from '@/methods/labels'
import { addLabel, selectors as labelsToSelectors } from '@/methods/labels'
import { MachineFilterOption } from '@/methods/machine'
import LabelsInput from '@/views/omni/ItemLabels/LabelsInput.vue'
import MachineItem from '@/views/omni/Machines/MachineItem.vue'

const props = defineProps<{
  filter?: MachineFilterOption
}>()

const { filter } = toRefs(props)

const route = useRoute()

const infraProviderStatuses = ref<Resource<InfraProviderStatusSpec>[]>([])
const infraProviderStatusesWatch = new WatchResource(infraProviderStatuses)

infraProviderStatusesWatch.setup({
  resource: {
    type: InfraProviderStatusType,
    namespace: InfraProviderNamespace,
  },
  runtime: Runtime.Omni,
})

const watchOpts = computed<WatchOptions>(() => {
  let selectors: string[] = []

  switch (filter.value) {
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

  const labelSelectors = labelsToSelectors(filterLabels.value)
  if (labelSelectors) {
    const q = labelSelectors.join(',')

    if (selectors.length === 0) {
      selectors = [q]
    } else {
      selectors = selectors.map((item) => item + ',' + q)
    }
  }

  return {
    runtime: Runtime.Omni,
    resource: {
      type: MachineStatusLinkType,
      namespace: MetricsNamespace,
    },
    selectors,
    selectUsingOR: true,
  }
})

const getCapacity = (item: Resource<MachineStatusMetricsSpec>) => {
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

const openDocs = () => {
  window.open('https://omni.siderolabs.com/explanation/infrastructure-providers', '_blank')?.focus()
}
</script>

<template>
  <div>
    <TList
      :opts="watchOpts"
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
            <TButton type="subtle" @click="openDocs">documentation</TButton> on how to configure and
            use infrastructure providers.
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
              @click="
                () =>
                  $router.push({ name: 'Overview', query: { modal: 'downloadInstallationMedia' } })
              "
              >installation media</TButton
            >
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
        <div class="flex gap-4">
          <PageHeader v-if="!filter" title="Machines">
            <div class="flex items-center gap-6">
              <StatsItem
                pluralized-text="Machine"
                :count="itemsCount"
                icon="nodes"
                :text="filtered ? ' Found' : ' Total'"
              />
            </div>
            <Watch
              v-if="!filtered"
              :opts="{
                resource: {
                  type: MachineStatusMetricsType,
                  id: MachineStatusMetricsID,
                  namespace: EphemeralNamespace,
                },
                runtime: Runtime.Omni,
              }"
            >
              <template #default="{ items }">
                <div class="flex items-center gap-6">
                  <StatsItem
                    text="Allocated"
                    :count="items[0]?.spec.allocated_machines_count ?? 0"
                    icon="arrow-right-square"
                  />
                  <StatsItem
                    text="Capacity Free"
                    units="%"
                    :count="getCapacity(items[0])"
                    icon="box"
                  />
                </div>
              </template>
            </Watch>
          </PageHeader>
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
        </div>
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
        />
      </template>
      <template #default="{ items, searchQuery }">
        <MachineItem
          v-for="item in items"
          :key="itemID(item)"
          :machine="item"
          :search-query="searchQuery"
          @filter-labels="(label) => addLabel(filterLabels, label)"
        />
      </template>
    </TList>
  </div>
</template>
