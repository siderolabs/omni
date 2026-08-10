<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import '@vue-flow/core/dist/style.css'

import { Background } from '@vue-flow/background'
import { type Edge, type GraphNode, type Node, useVueFlow, VueFlow } from '@vue-flow/core'
import { computed } from 'vue'

import {
  type ClusterKubernetesManifestsStatusSpecGroupStatus,
  type ClusterKubernetesManifestsStatusSpecManifestStatus,
  ClusterKubernetesManifestsStatusSpecManifestStatusPhase,
} from '@/api/omni/specs/omni.pb'

import ClusterManifestsGroupNode, {
  type ClusterManifestsGroupNodeData,
} from './ClusterManifestsGroupNode.vue'
import ClusterManifestsManifestNode, {
  type ClusterManifestsManifestNodeData,
} from './ClusterManifestsManifestNode.vue'

const { groups } = defineProps<{
  groups: Map<string, ClusterKubernetesManifestsStatusSpecGroupStatus>
}>()

const emit = defineEmits<{
  manifestClick: [string, ClusterKubernetesManifestsStatusSpecManifestStatus]
}>()

const { onNodesInitialized, onNodeClick, zoomIn, zoomOut, fitView } = useVueFlow()

defineExpose({
  zoomIn,
  zoomOut,
  fitView,
})

const GROUP_HEIGHT = 32
const GROUP_GAP = 48

const MANIFEST_HEIGHT = 79
const MANIFEST_SPACING = 16

interface GroupLayout {
  id: string
  group: ClusterKubernetesManifestsStatusSpecGroupStatus
  manifests: Map<string, ClusterKubernetesManifestsStatusSpecManifestStatus>
  centerY: number
}

const groupLayouts = computed<GroupLayout[]>(() => {
  let cursorY = 0

  return groups
    .entries()
    .toArray()
    .map(([groupId, group]) => {
      const manifests = new Map(Object.entries(group.manifests ?? {}))

      const n = manifests.size
      const subtreeHeight = Math.max(
        n * MANIFEST_HEIGHT + Math.max(n - 1, 0) * MANIFEST_SPACING,
        GROUP_HEIGHT,
      )
      const centerY = cursorY + subtreeHeight / 2

      cursorY += subtreeHeight + GROUP_GAP

      return { id: groupId, group, manifests, centerY }
    })
})

type ClusterManifestsNode =
  Node<ClusterManifestsGroupNodeData> | Node<ClusterManifestsManifestNodeData>

const nodes = computed<ClusterManifestsNode[]>(() =>
  groupLayouts.value.flatMap((layout) => [
    buildGroupNode(layout),
    ...layout.manifests
      .entries()
      .map(([manifestId, manifest], index) =>
        buildManifestNode(layout, manifestId, manifest, index),
      ),
  ]),
)

const edges = computed(() =>
  groupLayouts.value.flatMap((layout) =>
    layout.manifests
      .entries()
      .map(([id, m]) => buildEdge(layout, id, m))
      .toArray(),
  ),
)

function buildGroupNode(layout: GroupLayout): Node<ClusterManifestsGroupNodeData> {
  const inSyncCount = Object.values(layout.group.manifests ?? {}).filter(
    (m) => m.phase === ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED,
  ).length

  return {
    id: layout.id,
    type: 'group',
    width: 220,
    height: GROUP_HEIGHT,
    position: {
      x: 0,
      y: layout.centerY - GROUP_HEIGHT / 2,
    },
    selectable: false,
    data: {
      id: layout.id,
      group: layout.group,
      inSyncCount,
      manifestCount: layout.manifests.size,
    },
  }
}

function manifestNodeId(groupId: string, manifestId: string) {
  return `${groupId}::${manifestId}`
}

function buildManifestNode(
  layout: GroupLayout,
  manifestId: string,
  manifest: ClusterKubernetesManifestsStatusSpecManifestStatus,
  index: number,
): Node<ClusterManifestsManifestNodeData> {
  const n = layout.manifests.size
  const subtreeHeight = n * MANIFEST_HEIGHT + Math.max(n - 1, 0) * MANIFEST_SPACING

  return {
    id: manifestNodeId(layout.id, manifestId),
    type: 'manifest',
    width: 220,
    height: MANIFEST_HEIGHT,
    position: {
      x: 360,
      y: layout.centerY - subtreeHeight / 2 + index * (MANIFEST_HEIGHT + MANIFEST_SPACING),
    },
    selectable: false,
    data: {
      groupId: layout.id,
      manifest,
    },
  }
}

function buildEdge(
  layout: GroupLayout,
  manifestId: string,
  manifest: ClusterKubernetesManifestsStatusSpecManifestStatus,
): Edge {
  const applied = manifest.phase === ClusterKubernetesManifestsStatusSpecManifestStatusPhase.APPLIED
  const pending = manifest.phase === ClusterKubernetesManifestsStatusSpecManifestStatusPhase.PENDING

  return {
    id: `edge-${layout.id}-${manifestId}`,
    source: layout.id,
    sourceHandle: 'right',
    target: manifestNodeId(layout.id, manifestId),
    targetHandle: 'left',
    type: 'default',
    class: pending ? 'manifest-edge-pending' : undefined,
    style: {
      stroke: applied
        ? 'var(--color-green-g1)'
        : pending
          ? 'var(--color-yellow-y1)'
          : 'var(--color-red-r1)',
      strokeWidth: 1.5,
      strokeDasharray: applied ? undefined : '4 4',
      opacity: !applied ? 0.35 : 0.7,
    },
  }
}

function isManifestGraphNode(node: GraphNode): node is GraphNode<ClusterManifestsManifestNodeData> {
  return node.type === 'manifest'
}

onNodesInitialized(() => {
  fitView({ minZoom: 1, maxZoom: 1 })
})

onNodeClick(({ node }) => {
  if (isManifestGraphNode(node)) emit('manifestClick', node.data.groupId, node.data.manifest)
})
</script>

<template>
  <VueFlow
    :nodes
    :edges
    :nodes-draggable="false"
    :min-zoom="0.2"
    :max-zoom="3"
    class="min-h-80 overflow-hidden rounded-sm border border-naturals-n4 bg-naturals-n0"
  >
    <Background variant="dots" :gap="22" :size="2" pattern-color="var(--color-naturals-n4)" />

    <template #node-group="nodeProps">
      <ClusterManifestsGroupNode v-bind="nodeProps" />
    </template>

    <template #node-manifest="nodeProps">
      <ClusterManifestsManifestNode
        v-bind="nodeProps"
        @manifest-click="(groupId, manifest) => $emit('manifestClick', groupId, manifest)"
      />
    </template>
  </VueFlow>
</template>

<style scoped>
:deep(.manifest-edge-pending path) {
  animation: manifest-edge-dashdraw 0.5s linear infinite;
}

@keyframes manifest-edge-dashdraw {
  from {
    stroke-dashoffset: 8;
  }
}
</style>
