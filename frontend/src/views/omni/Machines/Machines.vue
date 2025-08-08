<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div>
    <t-list
      :opts="watchOpts"
      search
      pagination
      :sortOptions="sortOptions"
      :filterValue="filterValue"
    >
      <template #norecords>
        <t-alert
          type="info"
          title="No Infrastructure Providers Connected"
          v-if="filter === MachineFilterOption.Managed && infraProviderStatuses.length === 0"
        >
          <div class="flex gap-1">
            Check the
            <t-button type="subtle" @click="openDocs">documentation</t-button> on how to configure
            and use infrastructure providers.
          </div>
        </t-alert>

        <t-alert
          type="info"
          title="No Machines Found"
          v-else-if="filter === MachineFilterOption.Manual"
        >
          <div class="flex gap-1">
            Download and boot the
            <t-button
              type="subtle"
              @click="
                () =>
                  $router.push({ name: 'Overview', query: { modal: 'downloadInstallationMedia' } })
              "
              >installation media</t-button
            >
            to connect machines to your Omni instance.
          </div>
        </t-alert>

        <t-alert type="info" title="No Machines Found" v-else>
          <div class="flex gap-1">
            No entries of the requested resource type are found on the server.
          </div>
        </t-alert>
      </template>
      <template #header="{ itemsCount, filtered }">
        <div class="flex gap-4">
          <page-header title="Machines" v-if="!filter">
            <div class="flex gap-6 items-center">
              <stats-item
                pluralized-text="Machine"
                :count="itemsCount"
                icon="nodes"
                :text="filtered ? ' Found' : ' Total'"
              />
            </div>
            <watch
              :opts="{
                resource: {
                  type: MachineStatusMetricsType,
                  id: MachineStatusMetricsID,
                  namespace: EphemeralNamespace,
                },
                runtime: Runtime.Omni,
              }"
              v-if="!filtered"
            >
              <template #default="{ items }">
                <div class="flex items-center gap-6">
                  <stats-item
                    text="Allocated"
                    :count="items[0]?.spec.allocated_machines_count ?? 0"
                    icon="arrow-right-square"
                  />
                  <stats-item
                    text="Capacity Free"
                    units="%"
                    :count="getCapacity(items[0])"
                    icon="box"
                  />
                </div>
              </template>
            </watch>
          </page-header>
          <page-header
            title="Manually Joined Machines"
            v-else-if="filter === MachineFilterOption.Manual"
          />
          <page-header
            title="Machines Managed by the Infrastructure Providers"
            v-else-if="filter === MachineFilterOption.Managed"
          />
          <page-header
            :title="`Machines Managed by the Infrastructure Provider ${$route.params.provider}`"
            v-else-if="$route.params.provider"
          />
        </div>
      </template>
      <template #input>
        <labels-input
          :completions-resource="{
            id: MachineStatusType,
            type: LabelsCompletionType,
            namespace: VirtualNamespace,
          }"
          class="w-full"
          v-model:filter-labels="filterLabels"
          v-model:filter-value="filterValue"
        />
      </template>
      <template #default="{ items, searchQuery }">
        <machine-item
          v-for="item in items"
          :key="itemID(item)"
          :machine="item"
          @filterLabels="(label) => addLabel(filterLabels, label)"
          :search-query="searchQuery"
        />
      </template>
    </t-list>
  </div>
</template>

<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb'
import {
  MetricsNamespace,
  MachineStatusLinkType,
  LabelsCompletionType,
  VirtualNamespace,
  MachineStatusType,
  MachineStatusMetricsType,
  MachineStatusMetricsID,
  EphemeralNamespace,
  LabelIsManagedByStaticInfraProvider,
  LabelMachineRequest,
  LabelInfraProviderID,
  InfraProviderStatusType,
  InfraProviderNamespace,
} from '@/api/resources'
import type { WatchOptions } from '@/api/watch'
import { itemID } from '@/api/watch'

import TList from '@/components/common/List/TList.vue'
import MachineItem from '@/views/omni/Machines/MachineItem.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { computed, ref } from 'vue'
import LabelsInput from '@/views/omni/ItemLabels/LabelsInput.vue'
import type { Label } from '@/methods/labels'
import { addLabel, selectors as labelsToSelectors } from '@/methods/labels'
import Watch from '@/components/common/Watch/Watch.vue'
import type { Resource } from '@/api/grpc'
import type { MachineStatusMetricsSpec } from '@/api/omni/specs/omni.pb'
import StatsItem from '@/components/common/Stats/StatsItem.vue'
import { MachineFilterOption } from '@/methods/machine'
import { toRefs } from 'vue'
import { useRoute } from 'vue-router'
import type { InfraProviderStatusSpec } from '@/api/omni/specs/infra.pb'
import WatchResource from '@/api/watch'
import TAlert from '@/components/TAlert.vue'
import TButton from '@/components/common/Button/TButton.vue'

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
  if (registered == 0) {
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
