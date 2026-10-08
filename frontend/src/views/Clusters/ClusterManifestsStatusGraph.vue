<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { dump, loadAll } from 'js-yaml'
import type { ObjectMeta } from 'kubernetes-types/meta/v1'
import { computed, ref, useTemplateRef } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type {
  ClusterKubernetesManifestsStatusSpec,
  ClusterKubernetesManifestsStatusSpecManifestStatus,
  KubernetesManifestGroupSpec,
} from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, KubernetesManifestGroupType } from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { useResourceGet } from '@/methods/useResourceGet'
import ClusterManifestPhase from '@/views/Clusters/components/ClusterManifestPhase.vue'
import ClusterManifestsCanvas from '@/views/Clusters/components/ClusterManifestsCanvas.vue'

const { manifestsStatus } = defineProps<{
  manifestsStatus: Resource<ClusterKubernetesManifestsStatusSpec>
}>()

const groupStatuses = computed(() => new Map(Object.entries(manifestsStatus.spec.groups ?? {})))

const canvasRef = useTemplateRef('canvasRef')

const selectedManifest = ref<{
  groupId: string
  manifest: ClusterKubernetesManifestsStatusSpecManifestStatus
}>()
const selectedGroupId = computed(() => selectedManifest.value?.groupId ?? '')

const {
  data: selectedManifestGroup,
  loading: selectedManifestGroupLoading,
  error: selectedManifestGroupErr,
} = useResourceGet<KubernetesManifestGroupSpec>(() => ({
  skip: !selectedGroupId.value,
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: KubernetesManifestGroupType,
    id: selectedGroupId.value,
  },
}))

interface ManifestDoc {
  kind?: string
  metadata?: ObjectMeta
}

const manifestYAML = computed(() => {
  const manifest = selectedManifest.value?.manifest
  const groupData = selectedManifestGroup.value?.spec.data

  if (!manifest || !groupData) return

  const docs = loadAll(groupData) as ManifestDoc[]

  const doc = docs.find(
    (doc) =>
      doc?.metadata &&
      doc.kind === manifest.kind &&
      doc.metadata.name === manifest.name &&
      doc.metadata.namespace === manifest.namespace,
  )

  return dump(doc)
})
</script>

<template>
  <div class="flex grow flex-col gap-tight @3xl:flex-row">
    <div class="flex min-w-0 grow flex-col gap-tight">
      <div
        class="flex flex-wrap items-center justify-between rounded-lg border border-border-default bg-surface-card p-tight"
      >
        <div class="flex items-center gap-compact text-xs">
          <div class="flex items-center gap-1.5">
            <div class="h-0 w-5 border-t-2 border-status-success-default"></div>
            <span>Applied</span>
          </div>

          <div class="flex items-center gap-1.5">
            <div class="h-0 w-5 border-t-2 border-dashed border-status-danger-default"></div>
            <span>Deleting</span>
          </div>

          <div class="flex items-center gap-1.5">
            <div class="h-0 w-5 border-t-2 border-dashed border-status-warning-default"></div>
            <span>Pending</span>
          </div>
        </div>

        <div class="text-xs text-content-muted">Drag to pan · scroll to zoom</div>

        <div class="flex overflow-hidden rounded border border-border-default bg-surface-chrome">
          <IconButton
            icon="plus"
            aria-label="zoom in"
            class="rounded-none"
            @click="canvasRef?.zoomIn"
          />
          <IconButton
            icon="minus"
            aria-label="zoom out"
            class="rounded-none"
            @click="canvasRef?.zoomOut"
          />
          <IconButton
            icon="fullscreen"
            aria-label="fit view"
            class="rounded-none"
            @click="canvasRef?.fitView"
          />
        </div>
      </div>

      <ClusterManifestsCanvas
        ref="canvasRef"
        :groups="groupStatuses"
        @manifest-click="(groupId, manifest) => (selectedManifest = { groupId, manifest })"
      />
    </div>

    <div
      v-if="selectedManifest"
      class="flex w-full shrink-0 flex-col overflow-hidden rounded-lg border border-border-default @3xl:w-md"
    >
      <div
        class="flex justify-between gap-tight border-b border-border-default bg-surface-chrome px-compact py-tight"
      >
        <div class="flex flex-col gap-micro leading-tight">
          <div class="flex items-center gap-compact">
            <h3 class="text-sm font-medium text-content-emphasis">
              {{ selectedManifest.manifest.name }}
            </h3>
            <ClusterManifestPhase :phase="selectedManifest.manifest.phase" />
          </div>

          <span class="text-xs text-content-muted">
            {{
              [selectedManifest.manifest.kind, selectedManifest.manifest.namespace]
                .filter((s) => s?.trim())
                .join(' · ')
            }}
          </span>
        </div>

        <IconButton
          icon="close"
          class="shrink-0"
          aria-label="close manifest"
          @click="selectedManifest = undefined"
        />
      </div>

      <div class="flex-1 overflow-y-auto p-tight">
        <div v-if="selectedManifestGroupLoading" class="flex h-40 items-center justify-center">
          <TSpinner class="h-6 w-6" />
        </div>

        <TAlert v-else-if="selectedManifestGroupErr || !manifestYAML" title="Error" type="error">
          {{
            selectedManifestGroupErr
              ? selectedManifestGroupErr
              : 'Manifest YAML not found in the manifest group.'
          }}
        </TAlert>

        <CodeEditor
          v-else
          :model-value="manifestYAML"
          :options="{ readOnly: true }"
          disable-config-validation
          class="size-full min-h-40"
        />
      </div>
    </div>
  </div>
</template>
