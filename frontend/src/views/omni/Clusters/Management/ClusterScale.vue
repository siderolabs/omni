<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import * as semver from 'semver'
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { ClusterSpec, MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  DefaultNamespace,
  LabelNoManualAllocation,
  MachineConfigGenOptionsType,
  MachineStatusLabelAvailable,
  MachineStatusLabelInstalled,
  MachineStatusLabelInvalidState,
  MachineStatusLabelReadyToUse,
  MachineStatusLabelReportingEvents,
  MachineStatusType,
} from '@/api/resources'
import { itemID } from '@/api/watch'
import PageHeader from '@/components/common/PageHeader.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import Watch from '@/components/common/Watch/Watch.vue'
import TAlert from '@/components/TAlert.vue'
import { clusterSync } from '@/methods/cluster'
import { showError, showSuccess } from '@/notification'
import { populateExisting, state } from '@/states/cluster-management'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'
import ClusterMenu from '@/views/omni/Clusters/ClusterMenu.vue'
import ClusterMachineItem from '@/views/omni/Clusters/Management/ClusterMachineItem.vue'
import MachineSets from '@/views/omni/Clusters/Management/MachineSets.vue'

type Props = {
  currentCluster: Resource<ClusterSpec>
}

defineProps<Props>()

const route = useRoute()
const router = useRouter()

const quorumWarning = computed(() => {
  if (typeof state.value.controlPlanesCount === 'string') {
    return undefined
  }

  const totalMachines = state.value.controlPlanesCount as number

  if ((totalMachines + 1) % 2 === 0) {
    return undefined
  }

  return `${pluralize('Control Plane', totalMachines, true)} will not provide fault-tolerance with etcd quorum requirements. The total number of control plane machines must be an odd number to ensure etcd stability. Please add one more machine or remove one.`
})

const clusterName = route.params.cluster

const scaleCluster = async () => {
  try {
    await clusterSync(state.value.resources(), existingResources.value)
  } catch (e) {
    if (e.errorNotification) {
      showError(e.errorNotification.title, e.errorNotification.details)

      return
    }

    showError('Failed to Scale the Cluster', e.message)

    return
  }

  await router.push({ name: 'ClusterOverview', params: { cluster: clusterName as string } })

  showSuccess(
    'Updated Cluster Configuration',
    `Cluster name: ${clusterName}, control planes: ${state.value.controlPlanesCount}, workers: ${state.value.workersCount}`,
  )
}

const detectVersionMismatch = (machine: Resource<MachineStatusSpec>) => {
  const clusterVersion = semver.parse(state.value.cluster.talosVersion)
  const machineVersion = semver.parse(machine.spec.talos_version)

  const installed = machine.metadata.labels?.[MachineStatusLabelInstalled] !== undefined
  const inAgentMode = !!machine.spec.schematic?.in_agent_mode

  if (!installed) {
    if (
      machineVersion?.major === clusterVersion?.major &&
      machineVersion?.minor === clusterVersion?.minor
    ) {
      return null
    }

    if (inAgentMode) {
      return null
    }

    return 'The machine running from ISO or PXE must have the same major and minor version as the cluster it is going to be added to. Please use another ISO or change the cluster Talos version'
  }

  if (
    (machineVersion?.major ?? 0) <= (clusterVersion?.major ?? 0) &&
    (machineVersion?.minor ?? 0) <= (clusterVersion?.minor ?? 0)
  ) {
    return null
  }

  return 'The machine has newer Talos version installed: downgrade is not allowed. Upgrade the machine or change Talos cluster version'
}

const resource = {
  namespace: DefaultNamespace,
  type: MachineStatusType,
}

const existingResources = ref<Resource[]>([])
onMounted(async () => {
  existingResources.value = await populateExisting(clusterName as string)
})
</script>

<template>
  <div class="flex flex-col gap-3">
    <PageHeader :title="`Add Machines to Cluster ${$route.params.cluster}`" />
    <ManagedByTemplatesWarning :cluster="currentCluster" />
    <template v-if="existingResources.length > 0">
      <div class="text-naturals-n13">Machine Sets</div>
      <MachineSets />
      <div class="text-naturals-n13">Available Machines</div>
      <Watch
        :opts="[
          {
            resource: resource,
            selectors: [
              `${MachineStatusLabelAvailable}`,
              `${MachineStatusLabelReadyToUse}`,
              `!${MachineStatusLabelInvalidState}`,
              `${MachineStatusLabelReportingEvents}`,
              `!${LabelNoManualAllocation}`,
            ],
            runtime: Runtime.Omni,
            sortByField: 'created',
          },
          {
            resource: {
              type: MachineConfigGenOptionsType,
              namespace: DefaultNamespace,
            },
            runtime: Runtime.Omni,
          },
        ]"
        errors-alert
        no-records-alert
        spinner
      >
        <template #norecords>
          <TAlert v-if="!$slots.norecords" type="info" title="No Machines Available">
            Machine is available when it is connected, not allocated and is reporting Talos events.
          </TAlert>
        </template>
        <template #default="{ data }">
          <ClusterMachineItem
            v-for="item in data"
            :key="itemID(item)"
            :item="item"
            :version-mismatch="detectVersionMismatch(item)"
          />
        </template>
      </Watch>
      <div
        class="-mx-6 -mb-6 flex h-16 items-center border-t border-naturals-n4 bg-naturals-n1 px-5 py-3"
      >
        <ClusterMenu
          class="w-full"
          :control-planes="state.controlPlanesCount"
          :workers="state.workersCount"
          :on-submit="scaleCluster"
          action="Update"
          :warning="quorumWarning"
        />
      </div>
    </template>
    <div v-else class="flex flex-1 items-center justify-center">
      <TSpinner class="h-6 w-6" />
    </div>
  </div>
</template>
