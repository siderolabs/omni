<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computedAsync } from '@vueuse/core'
import pluralize from 'pluralize'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type {
  ClusterConfigVersionSpec,
  ClusterSpec,
  MachineInstallDiskConfigSpec,
  MachineInstallDiskStatusSpec,
  MachineStatusSpec,
} from '@/api/omni/specs/omni.pb'
import type { VersionContractSpec } from '@/api/omni/specs/virtual.pb'
import {
  ClusterConfigVersionType,
  DefaultNamespace,
  LabelNoManualAllocation,
  MachineInstallDiskConfigType,
  MachineInstallDiskStatusType,
  MachineStatusLabelAvailable,
  MachineStatusLabelInvalidState,
  MachineStatusLabelReadyToUse,
  MachineStatusLabelReportingEvents,
  MachineStatusType,
  VersionContractType,
  VirtualNamespace,
} from '@/api/resources'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import Pagination from '@/components/Pagination/Pagination.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { ClusterCommandError, clusterSync, reconcileInstallDiskConfigs } from '@/methods/cluster'
import { machineCompatibleWithCluster } from '@/methods/compat'
import { addLabel, type Label, selectors } from '@/methods/labels'
import { useResourcePagination } from '@/methods/resource/useResourcePagination'
import { useResourceSearch } from '@/methods/resource/useResourceSearch'
import { useLabelCompletions } from '@/methods/useLabelCompletions'
import { useResourceGet } from '@/methods/useResourceGet'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'
import { populateExisting, state } from '@/states/cluster-management'
import ClusterMenu from '@/views/Clusters/ClusterMenu.vue'
import ClusterMachineItem from '@/views/Clusters/Management/ClusterMachineItem.vue'
import MachineSets from '@/views/Clusters/Management/MachineSets.vue'
import LabelsInput from '@/views/ItemLabels/LabelsInput.vue'

definePage({ name: 'ClusterScale' })

const { currentCluster } = defineProps<{
  currentCluster: Resource<ClusterSpec>
}>()

const router = useRouter()

const loadingResources = ref(true)
const existingResources = computedAsync(() => populateExisting(currentCluster.metadata.id!), [], {
  evaluating: loadingResources,
})

const quorumWarning = computed(() => {
  if (loadingResources.value) return
  if (typeof state.value.controlPlanesCount === 'string') {
    return undefined
  }

  const totalMachines = state.value.controlPlanesCount as number

  if ((totalMachines + 1) % 2 === 0) {
    return undefined
  }

  return `${pluralize('Control Plane', totalMachines, true)} will not provide fault-tolerance with etcd quorum requirements. The total number of control plane machines must be an odd number to ensure etcd stability. Please add one more machine or remove one.`
})

const scaleCluster = async () => {
  try {
    // commit the install disk selections first, so they are in place before any MachineSetNode
    // makes a machine installable (the config resource never goes through clusterSync)
    await reconcileInstallDiskConfigs(state.value.pendingInstallDisks())

    await clusterSync(state.value.resources(), existingResources.value)
  } catch (e) {
    if (e instanceof ClusterCommandError) {
      showError(e.errorNotification.title, e.errorNotification.details)

      return
    }

    showError('Failed to Scale the Cluster', e.message)

    return
  }

  await router.push({
    name: 'ClusterOverview',
    params: { cluster: currentCluster.metadata.id! },
  })

  showSuccess(
    'Updated Cluster Configuration',
    `Cluster name: ${currentCluster.metadata.id}, control planes: ${state.value.controlPlanesCount}, workers: ${state.value.workersCount}`,
  )
}

const detectVersionMismatch = (machine: Resource<MachineStatusSpec>) => {
  const compat = machineCompatibleWithCluster(machine, state.value.cluster.talosVersion!)
  return compat.ok ? null : compat.reason
}

const detectAutoInstallNotice = (machine: Resource<MachineStatusSpec>) => {
  const compat = machineCompatibleWithCluster(machine, state.value.cluster.talosVersion!)
  return compat.ok && compat.willAutoInstall ? compat.reason : null
}

const filterLabels = ref<Label[]>([])
const filterValue = ref('')

const { match, completions } = useLabelCompletions({
  resourceType: MachineStatusType,
  filterValue,
})

const { watchOptions: searchState, searchQuery } = useResourceSearch({ filterValue })

const {
  total,
  watchOptions: paginationState,
  currentPage,
  currentPageSize: selectedItemsPerPage,
  pageCount,
  pageSizeSelectValues: itemsPerPage,
} = useResourcePagination({
  resetOn: [searchState],
})

const {
  data: machineStatuses,
  loading: machineStatusesLoading,
  err: machineStatusesErr,
} = useResourceWatch<MachineStatusSpec>(
  () => ({
    resource: {
      namespace: DefaultNamespace,
      type: MachineStatusType,
    },
    runtime: Runtime.Omni,
    sortByField: 'created',
    ...paginationState.value,
    ...searchState.value,
    selectors: [
      `${MachineStatusLabelAvailable}`,
      `${MachineStatusLabelReadyToUse}`,
      `!${MachineStatusLabelInvalidState}`,
      `${MachineStatusLabelReportingEvents}`,
      `!${LabelNoManualAllocation}`,
      ...(selectors(filterLabels.value) ?? []),
      ...(searchState.value.selectors ?? []),
    ],
  }),
  { total },
)

const {
  data: installDiskStatuses,
  loading: installDiskStatusesLoading,
  err: installDiskStatusesErr,
} = useResourceWatch<MachineInstallDiskStatusSpec>(() => ({
  resource: {
    type: MachineInstallDiskStatusType,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
}))

const {
  data: installDiskConfigs,
  loading: installDiskConfigsLoading,
  err: installDiskConfigsErr,
} = useResourceWatch<MachineInstallDiskConfigSpec>(() => ({
  resource: {
    type: MachineInstallDiskConfigType,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
}))

const { data: clusterConfigVersion } = useResourceGet<ClusterConfigVersionSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterConfigVersionType,
    id: currentCluster.metadata.id,
  },
}))

const {
  data: versionContract,
  loading: versionContractLoading,
  error: versionContractErr,
} = useResourceGet<VersionContractSpec>(() => ({
  skip: !clusterConfigVersion.value?.spec.version,
  runtime: Runtime.Omni,
  resource: {
    namespace: VirtualNamespace,
    type: VersionContractType,
    id: clusterConfigVersion.value?.spec.version,
  },
}))

const diskStatusMap = computed(() =>
  Object.fromEntries(installDiskStatuses.value.map((d) => [d.metadata.id!, d])),
)

const diskConfigMap = computed(() =>
  Object.fromEntries(installDiskConfigs.value.map((d) => [d.metadata.id!, d])),
)

const loading = computed(
  () =>
    machineStatusesLoading.value ||
    installDiskStatusesLoading.value ||
    installDiskConfigsLoading.value ||
    versionContractLoading.value,
)

const err = computed(
  () =>
    machineStatusesErr.value ||
    installDiskStatusesErr.value ||
    installDiskConfigsErr.value ||
    versionContractErr.value,
)
</script>

<template>
  <PageContainer disable-padding class="flex h-full flex-col pt-6">
    <PageHeader :title="`Add Machines to Cluster ${$route.params.cluster}`" class="px-6" />

    <div
      v-if="existingResources.length > 0"
      class="flex grow flex-col gap-4 overflow-y-auto px-6 pb-6"
    >
      <ManagedByTemplatesWarning :resource="currentCluster" />

      <div class="text-naturals-n13">Machine Sets</div>
      <MachineSets />
      <div class="text-naturals-n13">Available Machines</div>

      <div class="flex h-max max-w-full shrink-0 flex-col gap-2">
        <div class="flex grow flex-col gap-4 overflow-hidden">
          <LabelsInput
            v-model:filter-labels="filterLabels"
            v-model:filter-value="filterValue"
            :match
            :completions
            class="w-full"
          />

          <TSelectList
            v-model="selectedItemsPerPage"
            class="self-end"
            title="Items per Page"
            :values="itemsPerPage"
          />

          <div class="grow overflow-auto">
            <div v-if="loading" class="flex size-full items-center justify-center">
              <TSpinner class="size-6" />
            </div>
            <TAlert v-else-if="err" title="Failed to Fetch Data" type="error">{{ err }}.</TAlert>
            <TAlert v-else-if="!machineStatuses.length" type="info" title="No Machines Available">
              Machine is available when it is connected, not allocated and is reporting Talos
              events.
            </TAlert>

            <div v-else-if="versionContract" class="size-full">
              <ClusterMachineItem
                v-for="item in machineStatuses"
                :key="item.metadata.id"
                :item="item"
                :install-disk-status="diskStatusMap[item.metadata.id!]"
                :install-disk-config="diskConfigMap[item.metadata.id!]"
                :search-query
                :version-contract
                :version-mismatch="detectVersionMismatch(item)"
                :auto-install-notice="detectAutoInstallNotice(item)"
                @filter-label="filterLabels = addLabel(filterLabels, $event)"
              />
            </div>
          </div>
        </div>

        <Pagination v-model:current-page="currentPage" :page-count="pageCount" />
      </div>
    </div>

    <div v-else class="flex flex-1 items-center justify-center">
      <TSpinner class="h-6 w-6" />
    </div>

    <div
      class="flex h-16 shrink-0 items-center border-t border-naturals-n4 bg-naturals-n1 px-5 py-3"
    >
      <ClusterMenu
        class="w-full"
        :loading="loadingResources"
        :control-planes="state.controlPlanesCount"
        :workers="state.workersCount"
        :on-submit="scaleCluster"
        :disabled="!existingResources.length"
        action="Update"
        :warning="quorumWarning"
      />
    </div>
  </PageContainer>
</template>
