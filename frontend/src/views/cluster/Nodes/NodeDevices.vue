<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
export interface TalosPCIDeviceSpec {
  class?: string
  subclass?: string
  vendor?: string
  product?: string
  class_id?: string
  subclass_id?: string
  vendor_id?: string
  product_id?: string
  driver?: string
}

const PCI_CLASS_ICONS: Record<string, IconType> = {
  '0x01': 'circle-stack',
  '0x02': 'server-network',
  '0x06': 'arrows-right-left',
  '0x07': 'terminal',
  '0x08': 'settings',
  '0x0b': 'cpu-chip',
  '0x0d': 'server-network',
  '0x10': 'locked',
}

type DeviceTreeItem = DeviceTreeFolder | DeviceTreeDevice

interface DeviceTreeFolder {
  id: string
  label: string
  children: DeviceTreeItem[]
}

interface DeviceTreeDevice {
  id: string
  label: string
  device: Resource<TalosPCIDeviceSpec>
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

function getDeviceIcon(classId?: string): IconType {
  if (!classId) return 'cpu-chip'
  return PCI_CLASS_ICONS[classId] ?? 'cpu-chip'
}

function asDevice(value: DeviceTreeItem): DeviceTreeDevice | undefined {
  return 'device' in value ? value : undefined
}
</script>
<script setup lang="ts">
import { TreeItem, TreeRoot, TreeVirtualizer } from 'reka-ui'
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { TalosHardwareNamespace, TalosPCIDeviceType } from '@/api/resources'
import TIcon, { type IconType } from '@/components/common/Icon/TIcon.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import Tag from '@/components/common/Tag/Tag.vue'
import TAlert from '@/components/TAlert.vue'
import { getContext } from '@/context'
import { useResourceWatch } from '@/methods/useResourceWatch'

const {
  data: devices,
  loading,
  err,
} = useResourceWatch<TalosPCIDeviceSpec>({
  resource: {
    namespace: TalosHardwareNamespace,
    type: TalosPCIDeviceType,
  },
  runtime: Runtime.Talos,
  context: getContext(),
})

const tree = computed(() =>
  devices.value
    .reduce<DeviceTreeItem[]>((map, device) => {
      const deviceItem = {
        id: device.metadata.id!,
        label: `${device.spec.vendor} - ${device.spec.product}`,
        device,
      } satisfies DeviceTreeDevice

      const classId = `${device.spec.class_id}-${device.spec.subclass_id}`

      if (!map.find((c) => c.id === classId)) {
        map.push({
          id: classId,
          label: device.spec.subclass ?? 'Unknown',
          children: [],
        })
      }

      const subclassNode = map.find((c): c is DeviceTreeFolder => c.id === classId)!

      subclassNode.children.push(deviceItem)

      return map
    }, [])
    .sort(sortTreeItems)
    .map((item) => {
      if ('children' in item) item.children.sort(sortTreeItems)
      return item
    }),
)
</script>

<template>
  <div class="h-full overflow-hidden py-4">
    <TSpinner v-if="loading" class="size-4" />
    <TAlert v-else-if="err" type="error" title="Error">{{ err }}</TAlert>
    <TAlert v-else-if="!devices.length" type="info" title="No Records">
      No entries of the requested resource type are found on the server.
    </TAlert>

    <TreeRoot
      v-else
      class="h-full overflow-y-auto rounded-lg px-2 text-sm font-medium select-none"
      :items="tree"
      :get-key="(item) => item.id"
      :default-expanded="tree.map((t) => t.id)"
    >
      <TreeVirtualizer v-slot="{ item }" :estimate-size="36">
        <TreeItem
          v-slot="{ isExpanded }"
          :key="item._id"
          :style="{ 'padding-left': `${item.level - 0.5}rem` }"
          v-bind="item.bind"
          class="my-0.5 flex cursor-pointer items-center gap-2 rounded px-2 py-1 outline-none hover:bg-naturals-n6 focus-visible:ring-2 focus-visible:ring-naturals-n6"
        >
          <template v-if="item.hasChildren">
            <TIcon v-if="isExpanded" icon="folder-open" class="size-4 shrink-0" />
            <TIcon v-else icon="folder" class="size-4 shrink-0" />
            <span class="min-w-0 flex-1 truncate">{{ item.value.label }}</span>
            <Tag class="shrink-0 font-normal">
              {{ countDevices(item.value as DeviceTreeItem) }}
            </Tag>
          </template>

          <template v-else>
            <TIcon
              :icon="getDeviceIcon(asDevice(item.value as DeviceTreeItem)?.device.spec.class_id)"
              class="size-4 shrink-0 text-naturals-n10"
            />
            <span class="min-w-0 flex-1 truncate">{{ item.value.label }}</span>
            <Tag
              v-if="asDevice(item.value as DeviceTreeItem)?.device.spec.driver"
              class="shrink-0 font-normal"
            >
              {{ asDevice(item.value as DeviceTreeItem)?.device.spec.driver }}
            </Tag>
            <span class="shrink-0 font-mono text-xs text-naturals-n8">
              {{ asDevice(item.value as DeviceTreeItem)?.device.metadata.id }}
            </span>
          </template>
        </TreeItem>
      </TreeVirtualizer>
    </TreeRoot>
  </div>
</template>
