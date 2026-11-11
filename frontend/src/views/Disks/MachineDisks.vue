<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { coerce, gte } from 'semver'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import type { RuntimeContext } from '@/api/options'
import {
  DefaultNamespace,
  MachineStatusType,
  TalosBlockDeviceType,
  TalosDiscoveredVolumeType,
  TalosDiskType,
  TalosLVMLogicalVolumeStatusType,
  TalosRuntimeNamespace,
  TalosStorageNamespace,
  TalosSystemDiskType,
  TalosVolumeStatusType,
} from '@/api/resources'
import type {
  DeviceSpec,
  DiscoveredVolumeSpec,
  DiskSpec,
  SystemDiskSpec,
  VolumeStatusSpec,
} from '@/api/talos/block.pb'
import type { LVMLogicalVolumeStatusSpec } from '@/api/talos/storage.pb'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import DiskCard from '@/views/Disks/components/DiskCard.vue'
import { provideDeviceNavigation } from '@/views/Disks/lib/deviceNavigation'
import { buildDiskTopology } from '@/views/Disks/lib/diskTopology'
import { getFailedVolumes } from '@/views/Disks/lib/volumeStatus'

const { machineId } = defineProps<{
  machineId: string
  clusterId?: string
}>()

const context = computed<RuntimeContext>(() => ({
  node: machineId,
}))

const { data: machineStatus, loading: machineStatusLoading } = useResourceWatch<MachineStatusSpec>(
  () => ({
    runtime: Runtime.Omni,
    resource: {
      id: machineId,
      type: MachineStatusType,
      namespace: DefaultNamespace,
    },
  }),
)

const hasLogicalVolumeStatuses = computed(() => {
  const version = coerce(machineStatus.value?.spec.talos_version)

  return version !== null && gte(version, '1.14.0')
})

const {
  data: disks,
  loading: disksLoading,
  err: disksErr,
  errCode: disksErrCode,
} = useResourceWatch<DiskSpec>(() => ({
  resource: {
    namespace: TalosRuntimeNamespace,
    type: TalosDiskType,
  },
  runtime: Runtime.Talos,
  context: context.value,
}))

const {
  data: volumes,
  loading: volumesLoading,
  err: volumesErr,
  errCode: volumesErrCode,
} = useResourceWatch<DiscoveredVolumeSpec>(() => ({
  resource: {
    namespace: TalosRuntimeNamespace,
    type: TalosDiscoveredVolumeType,
  },
  runtime: Runtime.Talos,
  context: context.value,
}))

const {
  data: volumeStatuses,
  loading: volumeStatusesLoading,
  err: volumeStatusesErr,
  errCode: volumeStatusesErrCode,
} = useResourceWatch<VolumeStatusSpec>(() => ({
  resource: {
    namespace: TalosRuntimeNamespace,
    type: TalosVolumeStatusType,
  },
  runtime: Runtime.Talos,
  context: context.value,
}))

const {
  data: blockDevices,
  loading: blockDevicesLoading,
  err: blockDevicesErr,
  errCode: blockDevicesErrCode,
} = useResourceWatch<DeviceSpec>(() => ({
  resource: {
    namespace: TalosRuntimeNamespace,
    type: TalosBlockDeviceType,
  },
  runtime: Runtime.Talos,
  context: context.value,
}))

// Missing in maintenance mode, where Talos isn't installed.
const {
  data: systemDisks,
  loading: systemDisksLoading,
  err: systemDisksErr,
  errCode: systemDisksErrCode,
} = useResourceWatch<SystemDiskSpec>(() => ({
  resource: {
    namespace: TalosRuntimeNamespace,
    type: TalosSystemDiskType,
  },
  runtime: Runtime.Talos,
  context: context.value,
}))

// Only needed to recognise logical volumes on Talos 1.14, which doesn't report
// the device-mapper kind of a disk.
const {
  data: logicalVolumes,
  loading: logicalVolumesLoading,
  err: logicalVolumesErr,
  errCode: logicalVolumesErrCode,
} = useResourceWatch<LVMLogicalVolumeStatusSpec>(() => ({
  resource: {
    namespace: TalosStorageNamespace,
    type: TalosLVMLogicalVolumeStatusType,
  },
  runtime: Runtime.Talos,
  context: context.value,
  skip: !hasLogicalVolumeStatuses.value,
}))

const loading = computed(
  () =>
    machineStatusLoading.value ||
    disksLoading.value ||
    volumesLoading.value ||
    volumeStatusesLoading.value ||
    blockDevicesLoading.value ||
    systemDisksLoading.value ||
    logicalVolumesLoading.value,
)

const err = computed(
  () =>
    disksErr.value ||
    volumesErr.value ||
    volumeStatusesErr.value ||
    blockDevicesErr.value ||
    systemDisksErr.value ||
    logicalVolumesErr.value,
)

const errCode = computed(
  () =>
    disksErrCode.value ||
    volumesErrCode.value ||
    volumeStatusesErrCode.value ||
    blockDevicesErrCode.value ||
    systemDisksErrCode.value ||
    logicalVolumesErrCode.value,
)

const topology = computed(() =>
  buildDiskTopology({
    disks: disks.value,
    volumes: volumes.value,
    volumeStatuses: volumeStatuses.value,
    blockDevices: blockDevices.value,
    logicalVolumes: logicalVolumes.value,
    systemDisks: systemDisks.value,
  }),
)

const nameOf = (id: string) => topology.value.byID.get(id)?.name ?? id

const sections = computed(() =>
  [
    { title: 'Physical disks', nodes: topology.value.physical },
    { title: 'Virtual devices', nodes: topology.value.virtual },
  ].filter((s) => s.nodes.length),
)

provideDeviceNavigation()

const failedVolumes = computed(() => getFailedVolumes(volumeStatuses.value))
</script>

<template>
  <PageContainer class="space-y-compact">
    <template v-if="!loading && !err">
      <TAlert
        v-for="volume in failedVolumes"
        :key="volume.metadata.id"
        type="error"
        :title="`Volume ${volume.metadata.id} failed`"
      >
        {{ volume.spec.errorMessage }}
      </TAlert>
    </template>

    <TAlert v-if="errCode === Code.UNAVAILABLE" type="warn" title="Machine not ready">
      Talos API is not ready yet
    </TAlert>
    <TSpinner v-else-if="loading" class="mx-auto size-6" />
    <TAlert v-else-if="err" type="error" title="Error">{{ err }}</TAlert>
    <TAlert v-else-if="!sections.length" type="info" title="No Records">No disks found.</TAlert>

    <template v-for="section in sections" v-else :key="section.title">
      <!-- Headings only help to tell the sections apart when there are two. -->
      <h2 v-if="sections.length > 1" class="pt-tight text-sm font-medium text-content-emphasis">
        {{ section.title }}
      </h2>

      <DiskCard v-for="node in section.nodes" :key="node.id" :node :name-of />
    </template>
  </PageContainer>
</template>
