<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type {
  ClusterKubernetesManifestsStatusSpec,
  ClusterKubernetesManifestsStatusSpecGroupStatus,
} from '@/api/omni/specs/omni.pb'
import {
  ClusterKubernetesManifestsStatusSpecGroupStatusPhase,
  ClusterKubernetesManifestsStatusSpecManifestStatusPhase,
  KubernetesManifestGroupSpecMode,
} from '@/api/omni/specs/omni.pb'
import { ClusterKubernetesManifestsStatusType, DefaultNamespace } from '@/api/resources'
import TListItem from '@/components/List/TListItem.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import StatsItem from '@/components/Stats/StatsItem.vue'
import TAlert from '@/components/TAlert.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'

const { cluster } = defineProps<{
  cluster: string
}>()

const { data, loading, err } = useResourceWatch<ClusterKubernetesManifestsStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterKubernetesManifestsStatusType,
    id: cluster,
  },
}))

const groups = computed(() => {
  if (!data.value?.spec.groups) return []

  return Object.entries(data.value.spec.groups).map(([id, group]) => ({
    id,
    ...group,
    manifestsList: Object.entries(group.manifests ?? {}).map(([manifestId, manifest]) => ({
      id: manifestId,
      ...manifest,
    })),
  }))
})

const inSyncCount = computed(() => {
  return (data.value?.spec.total ?? 0) - (data.value?.spec.out_of_sync ?? 0)
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

const groupPhaseClass = (phase?: ClusterKubernetesManifestsStatusSpecGroupStatusPhase) => {
  switch (phase) {
    case ClusterKubernetesManifestsStatusSpecGroupStatusPhase.APPLIED:
      return 'text-green-g1'
    case ClusterKubernetesManifestsStatusSpecGroupStatusPhase.PENDING:
      return 'text-yellow-y1'
    case ClusterKubernetesManifestsStatusSpecGroupStatusPhase.PROGRESSING:
      return 'text-primary-p3'
    case ClusterKubernetesManifestsStatusSpecGroupStatusPhase.DELETING:
      return 'text-red-r1'
    default:
      return 'text-naturals-n9'
  }
}

const manifestPhaseClass = (phase?: ClusterKubernetesManifestsStatusSpecManifestStatusPhase) => {
  switch (phase) {
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED:
      return 'text-green-g1'
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.PENDING:
      return 'text-yellow-y1'
    case ClusterKubernetesManifestsStatusSpecManifestStatusPhase.DELETING:
      return 'text-red-r1'
    default:
      return 'text-naturals-n9'
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
  <PageContainer class="flex flex-col">
    <PageHeader :title="`Manifests Status — ${cluster}`">
      <template v-if="data">
        <StatsItem title="Total" :value="data.spec.total ?? 0" icon="document-text" />
        <StatsItem title="In Sync" :value="inSyncCount" icon="check-in-circle" />
        <StatsItem
          v-if="data.spec.out_of_sync"
          title="Out of Sync"
          :value="data.spec.out_of_sync"
          icon="warning"
        />
      </template>
    </PageHeader>

    <div v-if="loading" class="flex h-40 items-center justify-center">
      <TSpinner class="h-6 w-6" />
    </div>

    <TAlert v-else-if="err" title="Error" type="error">
      {{ err }}
    </TAlert>

    <TAlert v-else-if="!data" title="No Manifests Status" type="info">
      No manifests status available for this cluster.
    </TAlert>

    <TAlert v-else-if="data.spec.last_error" title="Manifest Error" type="error">
      {{ data.spec.last_error }}
    </TAlert>

    <TAlert v-else-if="groups.length === 0" title="No Manifest Groups" type="info">
      No manifest groups found for this cluster.
    </TAlert>

    <TListItem
      v-for="group in groups"
      v-else
      :key="group.id"
      :aria-label="group.id"
      disable-border-on-expand
    >
      <div class="flex flex-1 items-center gap-4">
        <span class="font-bold">{{ group.id }}</span>
        <span class="resource-label label-green" :class="groupPhaseClass(group.phase)">
          {{ groupPhaseName(group.phase) }}
        </span>
        <span class="text-xs text-naturals-n9">
          Mode:
          <span class="text-naturals-n13">{{ modeName(group.mode) }}</span>
        </span>
        <span class="text-xs text-naturals-n9">
          {{ groupInSyncCount(group) }}/{{ manifestCount(group) }} in sync
        </span>
      </div>

      <template #details>
        <div v-if="group.manifestsList.length === 0">No manifests in this group.</div>
        <div v-else class="flex flex-col gap-1" role="table">
          <div role="rowgroup">
            <div
              class="grid grid-cols-[repeat(4,1fr)_auto] gap-2 px-2 py-1 text-xs font-bold text-naturals-n9"
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
              class="grid grid-cols-[repeat(4,1fr)_auto] gap-2 rounded px-2 py-1.5 text-xs hover:bg-naturals-n3"
              role="row"
              :aria-label="manifest.id"
            >
              <div class="truncate text-naturals-n13" :title="manifest.id" role="cell">
                {{ manifest.id }}
              </div>
              <div class="text-naturals-n11" role="cell">{{ manifest.kind || '—' }}</div>
              <div class="text-naturals-n11" role="cell">{{ manifest.name || '—' }}</div>
              <div class="text-naturals-n11" role="cell">{{ manifest.namespace || '—' }}</div>
              <div :class="manifestPhaseClass(manifest.phase)" role="cell">
                {{ manifestPhaseName(manifest.phase) }}
              </div>
            </div>
          </div>
        </div>
      </template>
    </TListItem>
  </PageContainer>
</template>
