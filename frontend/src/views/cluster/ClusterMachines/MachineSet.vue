<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { AccordionContent, AccordionHeader, AccordionItem, AccordionTrigger } from 'reka-ui'
import { computed, ref, useId } from 'vue'
import { useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type {
  ClusterMachineRequestStatusSpec,
  ClusterMachineStatusSpec,
  MachineSetStatusSpec,
} from '@/api/omni/specs/omni.pb'
import { MachineSetSpecMachineAllocationType } from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineRequestStatusType,
  ClusterMachineStatusType,
  DefaultNamespace,
  LabelCluster,
  LabelControlPlaneRole,
  LabelMachineRequestInUse,
  LabelMachineSet,
  MachineSetStatusType,
} from '@/api/resources'
import { itemID } from '@/api/watch'
import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/common/ActionsBox/TActionsBoxItem.vue'
import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import { setupClusterPermissions } from '@/methods/auth'
import {
  controlPlaneMachineSetId,
  defaultWorkersMachineSetId,
  machineSetTitle,
} from '@/methods/machineset'
import { useResourceWatch } from '@/methods/useResourceWatch'
import ScaleMachinesModal from '@/views/cluster/ClusterMachines/ScaleMachinesModal.vue'

import ClusterMachine from './ClusterMachine.vue'
import MachineRequest from './MachineRequest.vue'
import MachineSetPhase from './MachineSetPhase.vue'

const showMachinesCount = ref<number | undefined>(25)

const { machineSetId } = defineProps<{
  machineSetId: string
  nodesWithDiagnostics: Set<string>
  isSubgrid?: boolean
}>()

const { data: machineSet } = useResourceWatch<MachineSetStatusSpec>(() => ({
  resource: {
    namespace: DefaultNamespace,
    type: MachineSetStatusType,
    id: machineSetId,
  },
  runtime: Runtime.Omni,
}))

const clusterID = computed(() => machineSet.value?.metadata.labels?.[LabelCluster])
const scaleMachinesModalOpen = ref(false)

const hiddenMachinesCount = computed(() => {
  if (showMachinesCount.value === undefined) {
    return 0
  }

  return Math.max(0, (machineSet.value?.spec.machines?.total || 0) - showMachinesCount.value)
})

const { data: machines } = useResourceWatch<ClusterMachineStatusSpec>(() => ({
  skip: !clusterID.value,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterMachineStatusType,
  },
  runtime: Runtime.Omni,
  selectors: [`${LabelCluster}=${clusterID.value}`, `${LabelMachineSet}=${machineSetId}`],
  limit: showMachinesCount.value,
}))

const { data: requests } = useResourceWatch<ClusterMachineRequestStatusSpec>(() => ({
  skip: !clusterID.value || !isMachineSetScalable(machineSet.value),
  resource: {
    namespace: DefaultNamespace,
    type: ClusterMachineRequestStatusType,
  },
  selectors: [
    `${LabelCluster}=${clusterID.value}`,
    `${LabelMachineSet}=${machineSetId}`,
    `!${LabelMachineRequestInUse}`,
  ],
  runtime: Runtime.Omni,
  limit: showMachinesCount.value,
}))

const router = useRouter()

const openMachineSetDestroy = () => {
  router.push({
    query: { modal: 'machineSetDestroy', machineSet: machineSetId },
  })
}

const { canRemoveClusterMachines } = setupClusterPermissions(clusterID)

const canRemoveMachine = computed(() => {
  if (!canRemoveClusterMachines.value) {
    return false
  }

  if (machineSet.value?.metadata.labels?.[LabelControlPlaneRole] === undefined) {
    return true
  }

  return machines.value.length > 1
})

const canRemoveMachineSet = computed(() => {
  if (!canRemoveClusterMachines.value) {
    return false
  }

  const deleteProtected = new Set<string>([
    controlPlaneMachineSetId(clusterID.value ?? ''),
    defaultWorkersMachineSetId(clusterID.value ?? ''),
  ])

  return !deleteProtected.has(machineSetId)
})

const requestedMachines = computed(() => {
  if (
    machineSet.value?.spec.machine_allocation?.allocation_type ===
    MachineSetSpecMachineAllocationType.Unlimited
  ) {
    return '∞'
  }

  return machineSet.value?.spec?.machines?.requested || 0
})

const machineClassMachineCount = computed(() => {
  if (
    machineSet.value?.spec.machine_allocation?.allocation_type ===
    MachineSetSpecMachineAllocationType.Unlimited
  ) {
    return 'All Machines'
  }

  return pluralize('Machine', machineSet.value?.spec.machine_allocation?.machine_count ?? 0, true)
})

const sectionHeadingId = useId()

function isMachineSetScalable(
  machineSet?: Resource<MachineSetStatusSpec>,
): machineSet is Resource<
  MachineSetStatusSpec & Required<Pick<MachineSetStatusSpec, 'machine_allocation'>>
> {
  return !!machineSet?.spec.machine_allocation
}
</script>

<template>
  <AccordionItem
    v-if="machines.length > 0 || requests.length > 0"
    as="section"
    :value="machineSetId"
    class="grid border-t-8 border-naturals-n4 text-naturals-n14"
    :class="
      isSubgrid ? 'col-span-full grid-cols-subgrid' : 'grid-cols-[repeat(4,1fr)_--spacing(18)]'
    "
    :aria-labelledby="sectionHeadingId"
    v-bind="$attrs"
  >
    <AccordionHeader class="col-span-full grid grid-cols-subgrid items-center p-2 pr-4 text-xs">
      <AccordionTrigger
        :id="sectionHeadingId"
        class="group/accordion flex shrink-0 items-stretch gap-0.5 truncate text-left"
      >
        <div class="flex shrink-0 items-center rounded-l bg-naturals-n4 px-0.5">
          <TIcon
            class="size-5 transition-transform duration-250 group-data-[state=open]/accordion:rotate-180"
            icon="drop-up"
            aria-hidden="true"
          />
        </div>

        <div class="flex min-w-0 items-center gap-1 rounded-r bg-naturals-n4 px-2 py-1.5">
          <TIcon icon="server-stack" class="size-4 shrink-0" aria-hidden="true" />
          <span class="grow truncate">
            {{ machineSetTitle(clusterID, machineSetId) }}
          </span>
        </div>
      </AccordionTrigger>

      <div class="flex items-center">
        {{ machineSet?.spec.machines?.healthy || 0 }}/
        <div :class="{ 'mt-0.5 text-lg': requestedMachines === '∞' }">
          {{ requestedMachines }}
        </div>
      </div>

      <MachineSetPhase
        v-if="machineSet"
        :item="machineSet"
        :class="{ 'col-span-2': !isMachineSetScalable(machineSet) }"
      />

      <TButton
        v-if="isMachineSetScalable(machineSet)"
        size="sm"
        class="max-w-full justify-self-start overflow-hidden text-xs"
        variant="primary"
        icon="edit"
        @click="scaleMachinesModalOpen = true"
      >
        Machine Class: {{ machineSet.spec.machine_allocation.name }} ({{
          machineClassMachineCount
        }})
      </TButton>

      <div class="flex items-center justify-end">
        <TActionsBox v-if="canRemoveMachineSet && machineSet">
          <TActionsBoxItem icon="delete" danger @select="() => openMachineSetDestroy()">
            Destroy Machine Set
          </TActionsBoxItem>
        </TActionsBox>
      </div>
    </AccordionHeader>

    <AccordionContent
      class="accordion-content col-span-full grid grid-cols-subgrid overflow-hidden"
    >
      <ClusterMachine
        v-for="machine in machines"
        :id="machine.metadata.id"
        :key="itemID(machine)"
        class="border-t border-naturals-n4 last-of-type:rounded-b-md"
        :has-diagnostic-info="nodesWithDiagnostics?.has(machine.metadata.id!)"
        :machine="machine"
        :delete-disabled="!canRemoveMachine"
      />

      <MachineRequest
        v-for="request in requests"
        :key="itemID(request)"
        class="border-t border-naturals-n4 last-of-type:rounded-b-md"
        :request-status="request"
      />

      <div
        v-if="hiddenMachinesCount > 0"
        class="col-span-full flex items-center gap-1 border-t border-naturals-n4 p-4 pl-9 text-xs"
      >
        {{ pluralize('machine', hiddenMachinesCount, true) }} are hidden
        <TButton variant="subtle" size="xs" @click="showMachinesCount = undefined">
          <span class="text-xs">Show all...</span>
        </TButton>
      </div>
    </AccordionContent>
  </AccordionItem>

  <ScaleMachinesModal
    v-if="isMachineSetScalable(machineSet)"
    v-model:open="scaleMachinesModalOpen"
    :machine-set="machineSet"
  />
</template>

<style scoped>
.accordion-content[data-state='open'] {
  animation: slideDown 200ms ease-out;
}

.accordion-content[data-state='closed'] {
  animation: slideUp 200ms ease-out;
}

@keyframes slideDown {
  from {
    height: 0;
  }
  to {
    height: var(--reka-accordion-content-height);
  }
}

@keyframes slideUp {
  from {
    height: var(--reka-accordion-content-height);
  }
  to {
    height: 0;
  }
}
</style>
