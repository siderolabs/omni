<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import type { Resource } from '@/api/grpc'
import type {
  ClusterKubernetesManifestsStatusSpec,
  ClusterKubernetesManifestsStatusSpecGroupStatus,
} from '@/api/omni/specs/omni.pb'
import {
  ClusterKubernetesManifestsStatusSpecGroupStatusPhase,
  ClusterKubernetesManifestsStatusSpecManifestStatusPhase,
  KubernetesManifestGroupSpecMode,
} from '@/api/omni/specs/omni.pb'
import TListItem from '@/components/List/TListItem.vue'
import type { StatusAppearance } from '@/components/Status/StatusPill.vue'
import StatusPill from '@/components/Status/StatusPill.vue'

const { manifestsStatus } = defineProps<{
  manifestsStatus: Resource<ClusterKubernetesManifestsStatusSpec>
}>()

const groups = computed(() => {
  if (!manifestsStatus.spec.groups) return []

  return Object.entries(manifestsStatus.spec.groups).map(([id, group]) => ({
    id,
    ...group,
    manifestsList: Object.entries(group.manifests ?? {}).map(([manifestId, manifest]) => ({
      id: manifestId,
      ...manifest,
    })),
  }))
})

const groupPhaseName = (phase?: ClusterKubernetesManifestsStatusSpecGroupStatusPhase) => {
  switch (phase) {
    case ClusterKubernetesManifestsStatusSpecGroupStatusPhase.PENDING:
      return 'Pending'
    case ClusterKubernetesManifestsStatusSpecGroupStatusPhase.PROGRESSING:
      return 'Progressing'
    case ClusterKubernetesManifestsStatusSpecGroupStatusPhase.APPLIED:
      return 'Applied'
    case ClusterKubernetesManifestsStatusSpecGroupStatusPhase.DELETING:
      return 'Deleting'
    default:
      return 'Unknown'
  }
}

const manifestPhaseName = (phase?: ClusterKubernetesManifestsStatusSpecManifestStatusPhase) => {
  switch (phase) {
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.PENDING:
      return 'Pending'
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED:
      return 'Applied'
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.DELETING:
      return 'Deleting'
    default:
      return 'Unknown'
  }
}

const modeName = (mode?: KubernetesManifestGroupSpecMode) => {
  switch (mode) {
    case KubernetesManifestGroupSpecMode.FULL:
      return 'Full'
    case KubernetesManifestGroupSpecMode.ONE_TIME:
      return 'One-Time'
    default:
      return 'Unknown'
  }
}

const groupPhaseStatus = (
  phase?: ClusterKubernetesManifestsStatusSpecGroupStatusPhase,
): StatusAppearance => {
  switch (phase) {
    case ClusterKubernetesManifestsStatusSpecGroupStatusPhase.APPLIED:
      return { tone: 'success', glyph: 'success' }
    case ClusterKubernetesManifestsStatusSpecGroupStatusPhase.PENDING:
      return { tone: 'warning', glyph: 'progress' }
    case ClusterKubernetesManifestsStatusSpecGroupStatusPhase.PROGRESSING:
      return { tone: 'info', glyph: 'progress' }
    case ClusterKubernetesManifestsStatusSpecGroupStatusPhase.DELETING:
      return { tone: 'info', glyph: 'progress' }
    default:
      return { tone: 'info', glyph: 'unknown' }
  }
}

const manifestPhaseClass = (phase?: ClusterKubernetesManifestsStatusSpecManifestStatusPhase) => {
  switch (phase) {
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED:
      return 'text-status-success-text'
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.PENDING:
      return 'text-status-warning-text'
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.DELETING:
      return 'text-status-info-text'
    default:
      return 'text-content-muted'
  }
}

const manifestCount = (group: ClusterKubernetesManifestsStatusSpecGroupStatus) => {
  return Object.keys(group.manifests ?? {}).length
}

const groupInSyncCount = (group: ClusterKubernetesManifestsStatusSpecGroupStatus) => {
  return Object.values(group.manifests ?? {}).filter(
    (m) => m.phase === ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED,
  ).length
}
</script>

<template>
  <TListItem
    v-for="group in groups"
    :key="group.id"
    :aria-label="group.id"
    disable-border-on-expand
  >
    <div class="flex flex-1 items-center gap-4">
      <span class="font-bold">{{ group.id }}</span>
      <StatusPill v-bind="groupPhaseStatus(group.phase)">
        {{ groupPhaseName(group.phase) }}
      </StatusPill>
      <span class="text-xs text-content-muted">
        Mode:
        <span class="text-content-default">{{ modeName(group.mode) }}</span>
      </span>
      <span class="text-xs text-content-muted">
        {{ groupInSyncCount(group) }}/{{ manifestCount(group) }} in sync
      </span>
    </div>

    <template #details>
      <div v-if="group.manifestsList.length === 0">No manifests in this group.</div>
      <div v-else class="flex flex-col gap-1" role="table">
        <div role="rowgroup">
          <div
            class="grid grid-cols-[repeat(4,1fr)_auto] gap-2 px-2 py-1 text-xs font-bold text-content-muted"
            role="row"
          >
            <div role="columnheader">ID</div>
            <div role="columnheader">Kind</div>
            <div role="columnheader">Name</div>
            <div role="columnheader">Namespace</div>
            <div role="columnheader">Status</div>
          </div>
        </div>

        <div role="rowgroup">
          <div
            v-for="manifest in group.manifestsList"
            :key="manifest.id"
            class="grid grid-cols-[repeat(4,1fr)_auto] gap-2 rounded px-2 py-1.5 text-xs hover:bg-surface-raised"
            role="row"
            :aria-label="manifest.id"
          >
            <div class="truncate text-content-default" :title="manifest.id" role="cell">
              {{ manifest.id }}
            </div>
            <div class="text-content-secondary" role="cell">{{ manifest.kind || '—' }}</div>
            <div class="text-content-secondary" role="cell">{{ manifest.name || '—' }}</div>
            <div class="text-content-secondary" role="cell">{{ manifest.namespace || '—' }}</div>
            <div :class="manifestPhaseClass(manifest.phase)" role="cell">
              {{ manifestPhaseName(manifest.phase) }}
            </div>
          </div>
        </div>
      </div>
    </template>
  </TListItem>
</template>
