<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { computed, ref, toRefs, useId, watch } from 'vue'
import { useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type {
  ClusterMachineRequestStatusSpec,
  ClusterMachineStatusSpec,
  MachineClassSpec,
  MachineSetStatusSpec,
} from '@/api/omni/specs/omni.pb'
import { MachineSetSpecMachineAllocationType } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import {
  ClusterMachineRequestStatusType,
  ClusterMachineStatusType,
  DefaultNamespace,
  LabelCluster,
  LabelControlPlaneRole,
  LabelMachineSet,
  MachineClassType,
} from '@/api/resources'
import { itemID } from '@/api/watch'
import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/common/ActionsBox/TActionsBoxItem.vue'
import IconButton from '@/components/common/Button/IconButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { useWatch } from '@/components/common/Watch/useWatch'
import { setupClusterPermissions } from '@/methods/auth'
import {
  controlPlaneMachineSetId,
  defaultWorkersMachineSetId,
  machineSetTitle,
  scaleMachineSet,
} from '@/methods/machineset'
import { showError } from '@/notification'

import ClusterMachine from './ClusterMachine.vue'
import MachineRequest from './MachineRequest.vue'
import MachineSetPhase from './MachineSetPhase.vue'

const showMachinesCount = ref<number | undefined>(25)

const props = defineProps<{
  machineSet: Resource<MachineSetStatusSpec>
  nodesWithDiagnostics: Set<string>
  isSubgrid?: boolean
}>()

const { machineSet } = toRefs(props)

const clusterID = computed(() => machineSet.value.metadata.labels?.[LabelCluster] ?? '')
const editingMachinesCount = ref(false)
const machineCount = ref(machineSet.value.spec.machine_allocation?.machine_count ?? 1)
const scaling = ref(false)
const canUseAll = ref<boolean | undefined>()

watch(editingMachinesCount, async (enabled: boolean, wasEnabled: boolean) => {
  if (!machineSet.value.spec.machine_allocation?.name) {
    return
  }

  if (!wasEnabled && enabled && canUseAll.value === undefined) {
    const machineClass: Resource<MachineClassSpec> = await ResourceService.Get(
      {
        type: MachineClassType,
        id: machineSet.value.spec.machine_allocation?.name,
        namespace: DefaultNamespace,
      },
      withRuntime(Runtime.Omni),
    )

    canUseAll.value = machineClass.spec.auto_provision === undefined
  }
})

const hiddenMachinesCount = computed(() => {
  if (showMachinesCount.value === undefined) {
    return 0
  }

  return Math.max(0, (machineSet.value.spec.machines?.total || 0) - showMachinesCount.value)
})

const { data: machines } = useWatch<ClusterMachineStatusSpec>(() => ({
  resource: {
    namespace: DefaultNamespace,
    type: ClusterMachineStatusType,
  },
  runtime: Runtime.Omni,
  selectors: [
    `${LabelCluster}=${clusterID.value}`,
    `${LabelMachineSet}=${machineSet.value.metadata.id}`,
  ],
  limit: showMachinesCount.value,
}))

const { data: requests } = useWatch<ClusterMachineRequestStatusSpec>(() => ({
  skip: !machineSet.value.spec.machine_allocation,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterMachineRequestStatusType,
  },
  selectors: [
    `${LabelCluster}=${clusterID.value}`,
    `${LabelMachineSet}=${machineSet.value.metadata.id}`,
  ],
  runtime: Runtime.Omni,
  limit: showMachinesCount.value,
}))

const pendingRequests = computed(() => {
  return requests.value.filter(
    (item) => !machines.value.find((machine) => machine.metadata.id === item.spec.machine_uuid),
  )
})

const router = useRouter()

const openMachineSetDestroy = (machineSet: Resource) => {
  router.push({
    query: { modal: 'machineSetDestroy', machineSet: machineSet.metadata.id },
  })
}

const { canRemoveClusterMachines } = setupClusterPermissions(clusterID)

const canRemoveMachine = computed(() => {
  if (!canRemoveClusterMachines.value) {
    return false
  }

  // don't allow destroying machines if the machine set is using automatic allocation
  if (machineSet.value.spec.machine_allocation?.name) {
    return false
  }

  if (machineSet.value.metadata.labels?.[LabelControlPlaneRole] === undefined) {
    return true
  }

  return machines.value.length > 1
})

const canRemoveMachineSet = computed(() => {
  if (!canRemoveClusterMachines.value) {
    return false
  }

  const deleteProtected = new Set<string>([
    controlPlaneMachineSetId(clusterID.value),
    defaultWorkersMachineSetId(clusterID.value),
  ])

  return !deleteProtected.has(machineSet.value.metadata.id!)
})

const updateMachineCount = async (
  allocationType: MachineSetSpecMachineAllocationType = MachineSetSpecMachineAllocationType.Static,
) => {
  scaling.value = true

  try {
    await scaleMachineSet(machineSet.value.metadata.id!, machineCount.value, allocationType)
  } catch (e) {
    showError(`Failed to Scale Machine Set ${machineSet.value.metadata.id}`, `Error: ${e.message}`)
  }

  scaling.value = false

  editingMachinesCount.value = false
}

const requestedMachines = computed(() => {
  if (
    machineSet.value.spec.machine_allocation?.allocation_type ===
    MachineSetSpecMachineAllocationType.Unlimited
  ) {
    return '∞'
  }

  return machineSet?.value?.spec?.machines?.requested || 0
})

const machineClassMachineCount = computed(() => {
  if (
    machineSet.value.spec?.machine_allocation?.allocation_type ===
    MachineSetSpecMachineAllocationType.Unlimited
  ) {
    return 'All Machines'
  }

  return pluralize('Machine', machineSet.value.spec?.machine_allocation?.machine_count ?? 0, true)
})

const sectionHeadingId = useId()
</script>

<template>
  <section
    v-if="machines.length > 0 || requests.length > 0"
    class="grid border-t-8 border-naturals-n4 text-naturals-n14"
    :class="
      isSubgrid ? 'col-span-full grid-cols-subgrid' : 'grid-cols-[repeat(4,1fr)_--spacing(18)]'
    "
    :aria-labelledby="sectionHeadingId"
  >
    <div class="col-span-full grid grid-cols-subgrid items-center p-2 pr-4 text-xs">
      <header class="flex max-w-40 items-center gap-2 truncate rounded bg-naturals-n4 px-3 py-2">
        <TIcon icon="server-stack" class="size-4 shrink-0" aria-hidden="true" />
        <h3 :id="sectionHeadingId" class="flex-1 truncate">
          {{ machineSetTitle(clusterID, machineSet?.metadata?.id) }}
        </h3>
      </header>

      <TSpinner v-if="scaling" class="size-4 shrink-0" aria-label="loading" />
      <div v-else-if="!editingMachinesCount" class="flex items-center gap-1">
        <div class="flex items-center">
          {{ machineSet?.spec?.machines?.healthy || 0 }}/
          <div :class="{ 'mt-0.5 text-lg': requestedMachines === '∞' }">
            {{ requestedMachines }}
          </div>
        </div>
        <IconButton
          v-if="machineSet.spec.machine_allocation?.name"
          icon="edit"
          @click="editingMachinesCount = !editingMachinesCount"
        />
      </div>
      <div v-else class="flex items-center gap-1">
        <div class="w-12">
          <TInput
            v-model="machineCount"
            :min="0"
            class="h-6"
            compact
            type="number"
            @keydown.enter="() => updateMachineCount()"
          />
        </div>
        <IconButton icon="check" @click="() => updateMachineCount()" />
        <TButton
          v-if="canUseAll"
          type="subtle"
          @click="() => updateMachineCount(MachineSetSpecMachineAllocationType.Unlimited)"
        >
          Use All
        </TButton>
      </div>

      <MachineSetPhase
        :item="machineSet"
        :class="{ 'col-span-2': !machineSet.spec.machine_allocation?.name }"
      />

      <div
        v-if="machineSet.spec.machine_allocation?.name"
        class="max-w-min rounded bg-naturals-n4 px-3 py-2 whitespace-nowrap"
      >
        Machine Class: {{ machineSet.spec.machine_allocation?.name }} ({{
          machineClassMachineCount
        }})
      </div>

      <TActionsBox v-if="canRemoveMachineSet" class="h-6 self-center justify-self-end" @click.stop>
        <TActionsBoxItem icon="delete" danger @click="() => openMachineSetDestroy(machineSet)">
          Destroy Machine Set
        </TActionsBoxItem>
      </TActionsBox>
    </div>

    <ClusterMachine
      v-for="machine in machines"
      :id="machine.metadata.id"
      :key="itemID(machine)"
      class="border-t border-naturals-n4 last-of-type:rounded-b-md"
      :machine-set="machineSet"
      :has-diagnostic-info="nodesWithDiagnostics?.has(machine.metadata.id!)"
      :machine="machine"
      :delete-disabled="!canRemoveMachine"
    />

    <MachineRequest
      v-for="request in pendingRequests"
      :key="itemID(request)"
      class="border-t border-naturals-n4 last-of-type:rounded-b-md"
      :request-status="request"
    />

    <div
      v-if="hiddenMachinesCount > 0"
      class="col-span-full flex items-center gap-1 border-t border-naturals-n4 p-4 pl-9 text-xs"
    >
      {{ pluralize('machine', hiddenMachinesCount, true) }} are hidden
      <TButton type="subtle" @click="showMachinesCount = undefined">
        <span class="text-xs">Show all...</span>
      </TButton>
    </div>
  </section>
</template>
