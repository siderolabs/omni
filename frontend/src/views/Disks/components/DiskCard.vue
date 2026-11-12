<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import prettyBytes from 'pretty-bytes'
import { CollapsibleContent, CollapsibleRoot, CollapsibleTrigger } from 'reka-ui'
import { computed, useTemplateRef } from 'vue'

import CopyButton from '@/components/CopyButton/CopyButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import StatusPill from '@/components/Status/StatusPill.vue'
import TAlert from '@/components/TAlert.vue'
import DeviceLink from '@/views/Disks/components/DeviceLink.vue'
import DiskPartitionTable from '@/views/Disks/components/DiskPartitionTable.vue'
import DiskUsageBar from '@/views/Disks/components/DiskUsageBar.vue'
import { useDeviceNavigationTarget } from '@/views/Disks/lib/deviceNavigation'
import { getDiskDetails, getDiskIdentity, getDiskMediaType } from '@/views/Disks/lib/diskDetails'
import { getHealthAlertType, getSMARTDetails } from '@/views/Disks/lib/diskHealth'
import type { DiskNode, DiskNodeKind } from '@/views/Disks/lib/diskTopology'

const { node } = defineProps<{
  node: DiskNode
  /** Returns the name to show for a device ID. */
  nameOf: (id: string) => string
}>()

const kindLabels: Partial<Record<DiskNodeKind, string>> = {
  cdrom: 'CD-ROM',
  md: 'RAID',
  lvm: 'LVM',
  multipath: 'Multipath',
  dm: 'Device mapper',
}

// An MD array is RAID by definition, so its level alone says enough: "RAID1",
// but "LVM RAID1".
const kindLabel = computed(() => {
  const level = node.raidLevel?.toUpperCase()

  if (node.kind === 'md' && level) return level

  return [kindLabels[node.kind], level].filter(Boolean).join(' ')
})

const identity = computed(() => getDiskIdentity(node.disk))
const details = computed(() => getDiskDetails(node.disk))
const smartDetails = computed(() => (node.smart ? getSMARTDetails(node.smart.spec) : []))

// Only a problem is worth spelling out; a healthy device's pill says enough.
const healthProblem = computed(() =>
  node.health?.tone !== 'success' && node.health?.message ? node.health : undefined,
)

// Talos reports a virtual device as rotational when its members are, so only a
// disk's own value means anything.
const mediaType = computed(() => (node.kind === 'disk' ? getDiskMediaType(node.disk) : undefined))

useDeviceNavigationTarget(() => node.id, useTemplateRef('card'))

const badgeClass = 'rounded bg-surface-inert px-tight py-micro text-xs text-content-default'
</script>

<template>
  <section
    ref="card"
    class="scroll-mt-4 overflow-hidden rounded-lg border border-border-default bg-surface-card"
    :aria-labelledby="`disk-${node.id}-title`"
  >
    <div class="border-b border-border-strong bg-surface-raised p-compact">
      <div class="flex flex-wrap items-center justify-between gap-tight">
        <div class="flex items-center gap-snug">
          <TIcon icon="server" class="size-6 text-content-default" />
          <div class="space-y-micro text-sm/none">
            <p :id="`disk-${node.id}-title`" class="text-content-emphasis">
              {{ node.name }}
            </p>
            <p class="font-medium text-content-secondary">
              {{ node.disk.spec.dev_path }}
            </p>
            <p v-if="identity.length" class="text-xs text-content-muted">
              {{ identity.join(' · ') }}
            </p>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-compact">
          <span class="text-sm text-content-emphasis">
            {{ prettyBytes(node.disk.spec.size ?? 0) }}
          </span>
          <div class="flex flex-wrap items-center gap-tight">
            <StatusPill
              v-if="node.health"
              :tone="node.health.tone"
              :glyph="node.health.glyph"
              :title="node.health.message"
            >
              {{ node.health.label }}
            </StatusPill>
            <span
              v-if="node.system"
              class="rounded bg-status-info-subtle px-tight py-micro text-xs text-status-info-text ring-1 ring-status-info-subtle-border ring-inset"
            >
              System disk
            </span>
            <span v-if="node.backedBy.length" :class="badgeClass">
              Backed by
              <template v-for="(id, i) in node.backedBy" :key="id">
                <template v-if="i > 0">,&nbsp;</template>
                <DeviceLink :id :name="nameOf(id)" />
              </template>
            </span>
            <span v-if="node.usedBy.length" :class="badgeClass">
              Used by
              <template v-for="(id, i) in node.usedBy" :key="id">
                <template v-if="i > 0">,&nbsp;</template>
                <DeviceLink :id :name="nameOf(id)" />
              </template>
            </span>
            <span v-if="node.paths.length" :class="badgeClass">
              Paths: {{ node.paths.join(', ') }}
            </span>
            <span v-if="kindLabel" :class="badgeClass">
              {{ kindLabel }}
            </span>
            <span v-if="mediaType" :class="badgeClass">
              {{ mediaType }}
            </span>
            <span v-if="node.disk.spec.transport" :class="badgeClass">
              {{ node.disk.spec.transport }}
            </span>
            <span
              v-if="node.disk.spec.readonly"
              class="rounded bg-status-warning-subtle px-tight py-micro text-xs text-status-warning-text ring-1 ring-status-warning-subtle-border ring-inset"
            >
              Read-only
            </span>
          </div>
        </div>
      </div>
    </div>

    <div class="space-y-tight bg-surface-card p-compact">
      <TAlert
        v-if="healthProblem"
        :type="getHealthAlertType(healthProblem)"
        :title="`${node.name}: ${healthProblem.label}`"
      >
        <span class="whitespace-pre-wrap">{{ healthProblem.message }}</span>
      </TAlert>

      <DiskUsageBar :disk="node.disk" :node-volumes="node.volumes" />
      <DiskPartitionTable v-if="node.volumes.length" :partitions="node.volumes" :name-of />

      <CollapsibleRoot v-if="details.length || smartDetails.length" v-slot="{ open }">
        <CollapsibleTrigger
          class="flex items-center gap-micro text-xs text-content-secondary hover:text-content-emphasis"
        >
          <TIcon
            icon="chevron-right"
            class="size-3 transition-transform"
            :class="{ 'rotate-90': open }"
            aria-hidden
          />
          Details
        </CollapsibleTrigger>

        <CollapsibleContent>
          <dl
            v-if="details.length"
            class="mt-tight grid grid-cols-[auto_1fr] gap-x-compact gap-y-micro text-xs"
          >
            <template v-for="detail in details" :key="detail.label">
              <dt class="text-content-secondary">{{ detail.label }}</dt>
              <dd class="min-w-0 space-y-micro">
                <div
                  v-for="value in detail.values"
                  :key="value"
                  class="flex items-center gap-tight"
                >
                  <span class="truncate font-mono text-content-default" :title="value">
                    {{ value }}
                  </span>
                  <CopyButton :text="value" class="shrink-0" />
                </div>
              </dd>
            </template>
          </dl>

          <template v-if="smartDetails.length">
            <p
              class="text-xs font-medium text-content-emphasis"
              :class="details.length ? 'mt-compact' : 'mt-tight'"
            >
              SMART
            </p>
            <dl class="mt-tight grid grid-cols-[auto_1fr] gap-x-compact gap-y-micro text-xs">
              <template v-for="detail in smartDetails" :key="detail.label">
                <dt class="text-content-secondary">{{ detail.label }}</dt>
                <dd class="text-content-default">{{ detail.values.join(', ') }}</dd>
              </template>
            </dl>
          </template>
        </CollapsibleContent>
      </CollapsibleRoot>
    </div>
  </section>
</template>
