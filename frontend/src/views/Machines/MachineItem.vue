<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import * as semver from 'semver'
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import WordHighlighter from 'vue-word-highlighter'

import type { Resource } from '@/api/grpc'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import { MachineStatusSpecPowerState } from '@/api/omni/specs/omni.pb'
import { LabelCluster, MachineStatusLabelInstalled } from '@/api/resources'
import TActionsBox from '@/components/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/ActionsBox/TActionsBoxItem.vue'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import CopyButton from '@/components/CopyButton/CopyButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import MachineStage from '@/components/Status/MachineStage.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { useClusterPermissions, usePermissions } from '@/methods/auth'
import type { Label } from '@/methods/labels'
import { addMachineLabels, removeMachineLabels } from '@/methods/machine'
import { getMachineName } from '@/methods/node'
import { useDerivedMachineStage } from '@/methods/useDerivedMachineStage'
import ItemLabels from '@/views/ItemLabels/ItemLabels.vue'

const {
  machine,
  showUUID,
  searchQuery = '',
} = defineProps<{
  machine: Resource<MachineStatusLinkSpec>
  panelOpen?: boolean
  showUUID?: boolean
  searchQuery?: string
}>()

defineEmits<{
  openPanel: []
  openMaintenanceUpdate: [machine: string]
  openMaintenanceInstall: [machine: string]
  openMaintenanceUpgrade: [machine: string]
  openMachineRemove: [machine: string, clusters: string[]]
  filterLabels: [Label]
}>()

const selected = defineModel<boolean>('selected')

const { copy } = useClipboard()

const { canAccessMaintenanceNodes, canReadClusters, canReadMachineLogs, canRemoveMachines } =
  usePermissions()

const { canManageKernelArgs, canReadConfigPatches } = useClusterPermissions(
  () => machine.metadata.labels?.[LabelCluster],
)

const machineName = computed(() => {
  return showUUID ? machine.metadata.id : getMachineName(machine)
})

const clusterName = computed(() => machine.spec.message_status?.cluster)

const { installing, upgrading } = useDerivedMachineStage(() => machine.spec.snapshot)

const canDoMaintenanceUpdate = computed(() => {
  if (machine.spec.message_status?.cluster) {
    return false
  }

  if (installing.value || upgrading.value) {
    return false
  }

  // Agent-mode machines are rejected by both MaintenanceUpgrade and MaintenanceLifecycle on the
  // server, so don't expose a button that the server will refuse.
  if (machine.spec.message_status?.schematic?.in_agent_mode) {
    return false
  }

  return machine.metadata.labels?.[MachineStatusLabelInstalled] !== undefined
})

const maintenanceUpdateDescription = computed(() => {
  if (machine.spec.message_status?.cluster) {
    return 'Maintenance upgrade is not possible: machine is a part of a cluster'
  }

  if (installing.value) {
    return 'Maintenance upgrade is not possible: a Talos install is already in progress on this machine'
  }

  if (upgrading.value) {
    return 'Maintenance upgrade is not possible: a Talos upgrade is already in progress on this machine'
  }

  if (machine.spec.message_status?.schematic?.in_agent_mode) {
    return 'Maintenance upgrade is not possible: machine is in agent mode'
  }

  if (!canDoMaintenanceUpdate.value) {
    return 'Maintenance upgrade is not possible: Talos is not installed'
  }

  return 'Change Talos version'
})

const canShowMaintenanceInstall = computed(() => {
  if (!canAccessMaintenanceNodes.value) {
    return false
  }

  if (machine.spec.message_status?.cluster) {
    return false
  }

  if (machine.metadata.labels?.[MachineStatusLabelInstalled] !== undefined) {
    return false
  }

  if (machine.spec.message_status?.schematic?.in_agent_mode) {
    return false
  }

  const version = semver.coerce(machine.spec.message_status?.talos_version)

  return version !== null && semver.gte(version, '1.13.0')
})

// `MaintenanceLifecycle(OPERATION_UPGRADE)` only exists in Talos >= 1.13
const canUseLifecycleUpgrade = computed(() => {
  const version = semver.coerce(machine.spec.message_status?.talos_version)

  return version !== null && semver.gte(version, '1.13.0')
})
</script>

<template>
  <div class="border-b-naturals-n5 not-last-of-type:border-b">
    <div
      class="grid grid-cols-[auto_1fr] gap-1 border-l-4 px-2 py-4"
      :class="panelOpen ? 'border-l-primary-p2' : 'border-l-transparent'"
    >
      <TCheckbox v-model="selected" class="shrink-0 justify-self-center" />

      <div
        class="flex items-center gap-2 overflow-hidden text-xs text-naturals-n13"
        :class="{ 'opacity-50': machine.spec.tearing_down }"
      >
        <h2 class="list-item-link truncate">
          <RouterLink :to="{ name: 'MachineLogs', params: { machine: machine.metadata.id! } }">
            <WordHighlighter :query="searchQuery" split-by-space highlight-class="bg-naturals-n14">
              {{ machineName }}
            </WordHighlighter>
          </RouterLink>
        </h2>

        <Tooltip :description="showUUID ? 'Copy machine UUID' : 'Copy machine name'">
          <CopyButton :text="machineName" />
        </Tooltip>

        <Tooltip
          v-if="
            machine.spec.message_status?.power_state === MachineStatusSpecPowerState.POWER_STATE_ON
          "
          description="Powered on"
        >
          <TIcon
            icon="power"
            class="size-4 shrink-0 text-green-g1"
            aria-label="machine powered on"
          />
        </Tooltip>

        <Tooltip
          v-if="
            machine.spec.message_status?.power_state === MachineStatusSpecPowerState.POWER_STATE_OFF
          "
          description="Powered off"
        >
          <TIcon
            icon="power-off"
            class="size-4 shrink-0 text-red-r1"
            aria-label="machine powered off"
          />
        </Tooltip>

        <MachineStage :machine icon-only />

        <div class="grow" />

        <Tooltip v-if="machine.spec.tearing_down" description="The machine is being destroyed">
          <TIcon icon="delete" class="h-4 w-4 text-red-r1" />
        </Tooltip>

        <div v-else class="flex items-center gap-1">
          <RouterLink
            v-if="clusterName && canReadClusters"
            :to="{ name: 'ClusterOverview', params: { cluster: clusterName } }"
            class="flex items-center gap-2 rounded-md px-2 py-1 text-xs font-medium whitespace-nowrap text-naturals-n11 hover:bg-naturals-n4 hover:text-naturals-n14"
          >
            <span class="max-md:hidden">{{ clusterName }}</span>
            <TIcon icon="clusters" aria-hidden="true" />
          </RouterLink>

          <RouterLink
            v-if="canReadMachineLogs"
            :to="{ name: 'MachineLogs', params: { machine: machine.metadata.id! } }"
            class="flex items-center gap-2 rounded-md px-2 py-1 text-xs font-medium whitespace-nowrap text-naturals-n11 hover:bg-naturals-n4 hover:text-naturals-n14"
          >
            <span class="max-md:hidden">Logs</span>
            <TIcon icon="log" aria-hidden="true" />
          </RouterLink>

          <Tooltip v-if="canAccessMaintenanceNodes" :description="maintenanceUpdateDescription">
            <button
              :disabled="!canDoMaintenanceUpdate"
              class="flex items-center gap-2 rounded-md px-2 py-1 text-xs font-medium whitespace-nowrap text-naturals-n11 hover:not-disabled:bg-naturals-n4 hover:not-disabled:text-naturals-n14 disabled:opacity-40"
              @click="
                canUseLifecycleUpgrade
                  ? $emit('openMaintenanceUpgrade', machine.metadata.id!)
                  : $emit('openMaintenanceUpdate', machine.metadata.id!)
              "
            >
              <span class="max-md:hidden">Update Talos</span>
              <TIcon icon="upgrade" aria-hidden="true" />
            </button>
          </Tooltip>

          <TActionsBox>
            <TActionsBoxItem
              v-if="canShowMaintenanceInstall"
              icon="upgrade"
              :disabled="installing || upgrading"
              @select="$emit('openMaintenanceInstall', machine.metadata.id!)"
            >
              Install Talos
            </TActionsBoxItem>

            <TActionsBoxItem
              icon="settings"
              :disabled="!canReadConfigPatches"
              @select="
                $router.push({
                  name: 'MachineConfigPatches',
                  params: { machine: machine.metadata.id! },
                })
              "
            >
              Config Patches
            </TActionsBoxItem>

            <TActionsBoxItem
              icon="settings"
              :disabled="!canManageKernelArgs"
              @select="
                $router.push({
                  name: 'MachineKernelArgs',
                  params: {
                    machine: machine.metadata.id!,
                  },
                })
              "
            >
              Update Kernel Args
            </TActionsBoxItem>

            <TActionsBoxItem icon="copy" @select="copy(machine.metadata.id!)">
              Copy Machine ID
            </TActionsBoxItem>

            <TActionsBoxItem
              icon="delete"
              danger
              :disabled="!canRemoveMachines"
              @select="
                $emit('openMachineRemove', machine.metadata.id!, clusterName ? [clusterName!] : [])
              "
            >
              Remove Machine
            </TActionsBoxItem>
          </TActionsBox>
        </div>
      </div>

      <button
        class="mt-0.75 size-4 shrink-0 justify-self-center text-naturals-n10 transition-colors hover:text-naturals-n14 active:text-naturals-n9"
        aria-label="details"
        @click="$emit('openPanel')"
      >
        <TIcon icon="info" class="size-full" />
      </button>

      <ItemLabels
        :resource="machine"
        :add-label-func="machine.spec.tearing_down ? undefined : addMachineLabels"
        :remove-label-func="removeMachineLabels"
        @filter-label="(label) => $emit('filterLabels', label)"
      />
    </div>
  </div>
</template>
