<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
import type { PCIDeviceSpec } from '@/api/talos/hardware.pb'

const PCI_CLASS_ICONS: Record<string, IconType> = {
  '0x01': 'circle-stack',
  '0x02': 'server-network',
  '0x06': 'arrows-right-left',
  '0x07': 'cpu-chip',
  '0x08': 'settings',
  '0x0b': 'cpu-chip',
  '0x0d': 'server-network',
  '0x10': 'locked',
}

type DeviceTreeItem = DeviceTreeGroup | DeviceTreeDevice

interface DeviceTreeGroup {
  id: string
  label: string
  icon: IconType
  children: DeviceTreeItem[]
}

interface DeviceTreeDevice {
  id: string
  label: string
  device: Resource<PCIDeviceSpec>
}

function getSortKey(item: DeviceTreeItem) {
  return `${'children' in item ? '0' : '1'}-${item.label}`
}

function sortTreeItems(a: DeviceTreeItem, b: DeviceTreeItem) {
  return getSortKey(a).localeCompare(getSortKey(b))
}

function countDevices(item: DeviceTreeItem): number {
  if (!('children' in item)) return 1
  return item.children.reduce((sum, child) => sum + countDevices(child), 0)
}

function getDeviceClassIcon(classId?: string): IconType {
  if (!classId) return 'cpu-chip'
  return PCI_CLASS_ICONS[classId] ?? 'question-mark-circle'
}

function asDevice(value: DeviceTreeItem): DeviceTreeDevice | undefined {
  return value && 'device' in value ? value : undefined
}
</script>

<script setup lang="ts">
import { TreeItem, TreeRoot, TreeVirtualizer } from 'reka-ui'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import type { RuntimeContext } from '@/api/options'
import { TalosHardwareNamespace, TalosPCIDeviceType } from '@/api/resources'
import TIcon, { type IconType } from '@/components/Icon/TIcon.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'

const { machineId } = defineProps<{
  machineId: string
  clusterId?: string
}>()

const context = computed<RuntimeContext>(() => ({
  node: machineId,
}))

const {
  data: devices,
  loading,
  err,
  errCode,
} = useResourceWatch<PCIDeviceSpec>(() => ({
  resource: {
    namespace: TalosHardwareNamespace,
    type: TalosPCIDeviceType,
  },
  runtime: Runtime.Talos,
  context: context.value,
}))

const tree = computed(() =>
  devices.value
    .reduce<DeviceTreeItem[]>((map, device) => {
      const deviceItem = {
        id: device.metadata.id!,
        label: `${device.spec.vendor} - ${device.spec.product}`,
        device,
      } satisfies DeviceTreeDevice

      const groupId = `${device.spec.class_id}-${device.spec.subclass_id}`

      if (!map.find((c) => c.id === groupId)) {
        map.push({
          id: groupId,
          label: device.spec.subclass ?? 'Unknown',
          icon: getDeviceClassIcon(device.spec.class_id),
          children: [],
        })
      }

      const subclassNode = map.find((c): c is DeviceTreeGroup => c.id === groupId)!

      subclassNode.children.push(deviceItem)

      return map
    }, [])
    .sort(sortTreeItems)
    .map((item) => {
      if ('children' in item) item.children.sort(sortTreeItems)
      return item
    }),
)

function isLastChild(item?: DeviceTreeItem) {
  if (!item) return false

  return (
    tree.value
      .filter((d) => 'children' in d)
      .find((d) => d.children.some((d) => d.id === item.id))
      ?.children.at(-1)?.id === item.id
  )
}
</script>

<template>
  <PageContainer class="h-full">
    <TAlert v-if="errCode === Code.UNAVAILABLE" type="warn" title="Machine not ready">
      Talos API is not ready yet
    </TAlert>
    <TSpinner v-else-if="loading" class="mx-auto size-6" />
    <TAlert v-else-if="err" type="error" title="Error">{{ err }}</TAlert>
    <TAlert v-else-if="!devices.length" type="info" title="No Records">No devices found.</TAlert>

    <TreeRoot
      v-else
      class="h-full overflow-y-auto rounded-lg text-xs/none text-content-default"
      :items="tree"
      :get-key="(item) => item.id"
      :default-expanded="tree.map((t) => t.id)"
    >
      <TreeVirtualizer v-slot="{ item }" :estimate-size="32">
        <TreeItem
          :key="item._id"
          v-bind="item.bind"
          class="group/tree-item relative w-full py-micro"
          :class="!item.hasChildren ? 'pl-section' : 'cursor-pointer pl-micro'"
        >
          <div
            v-if="item.hasChildren"
            class="flex h-7.5 items-center justify-between gap-tight rounded-lg bg-surface-card pr-tight pl-compact hover:bg-surface-hover"
          >
            <div class="flex min-w-0 items-center gap-compact">
              <TIcon :icon="item.value.icon" class="size-4 shrink-0 text-content-emphasis" />
              <span class="truncate">{{ item.value.label }}</span>
            </div>

            <div class="flex items-center gap-compact">
              <div class="rounded-md bg-surface-hover p-micro font-medium">
                {{ countDevices(item.value as DeviceTreeItem) }}
              </div>

              <TIcon
                icon="drop-up"
                class="size-6 transition-transform group-data-expanded/tree-item:rotate-180"
              />
            </div>
          </div>

          <template v-else>
            <div
              class="pointer-events-none absolute top-px left-6.75 border-l-2 border-border-strong"
              :class="isLastChild(asDevice(item.value as DeviceTreeItem)) ? 'h-1/2' : 'h-full'"
            ></div>

            <div
              class="pointer-events-none absolute top-px left-6.75 h-1/2 w-2 border-b-2 border-border-strong"
            ></div>

            <div
              class="flex h-7.5 items-center gap-tight rounded px-micro group-focus-visible/tree-item:ring-2 group-focus-visible/tree-item:ring-border-strong"
            >
              <span class="min-w-0 truncate">{{ item.value.label }}</span>

              <div class="h-px grow bg-surface-hover pr-compact pl-tight"></div>

              <span
                v-if="asDevice(item.value as DeviceTreeItem)?.device.spec.driver"
                class="rounded bg-surface-hover px-tight py-tight whitespace-nowrap text-content-default"
              >
                {{ asDevice(item.value as DeviceTreeItem)?.device.spec.driver }}
              </span>

              <span class="font-mono text-content-muted">
                {{ asDevice(item.value as DeviceTreeItem)?.device.metadata.id }}
              </span>
            </div>
          </template>
        </TreeItem>
      </TreeVirtualizer>
    </TreeRoot>
  </PageContainer>
</template>
