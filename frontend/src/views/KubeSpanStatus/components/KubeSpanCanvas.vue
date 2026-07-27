<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import '@vue-flow/core/dist/style.css'

import { Background } from '@vue-flow/background'
import { type Edge, type GraphNode, type Node, useVueFlow, VueFlow } from '@vue-flow/core'
import prettyBytes from 'pretty-bytes'
import { computed } from 'vue'

import type { Resource } from '@/api/grpc'
import type { ClusterMachineIdentitySpec } from '@/api/omni/specs/omni.pb'
import { LabelControlPlaneRole } from '@/api/resources'
import type { PeerStatusSpec } from '@/api/talos/kubespan.pb'
import KubeSpanPeerNode, {
  type KubeSpanPeerNodeData,
} from '@/views/KubeSpanStatus/components/KubeSpanPeerNode.vue'
import KubeSpanRootNode, {
  type KubeSpanRootNodeData,
} from '@/views/KubeSpanStatus/components/KubeSpanRootNode.vue'

const { selectedMachine, machineNodenameMap, peers, peerMatches } = defineProps<{
  selectedMachine: Resource<ClusterMachineIdentitySpec>
  machineNodenameMap: Map<string, Resource<ClusterMachineIdentitySpec>>
  peers: Resource<PeerStatusSpec>[]
  peerMatches: Set<string>
}>()

const emit = defineEmits<{
  peerClick: [Resource<PeerStatusSpec>]
}>()

const { onNodesInitialized, onNodeClick, zoomIn, zoomOut, fitView } = useVueFlow()

defineExpose({
  zoomIn,
  zoomOut,
  fitView,
})

const totalTrafficOut = computed(() =>
  peers.reduce((prev, p) => prev + (p.spec.transmitBytes ?? 0), 0),
)
const totalTrafficIn = computed(() =>
  peers.reduce((prev, p) => prev + (p.spec.receiveBytes ?? 0), 0),
)
const totalTraffic = computed(() => totalTrafficIn.value + totalTrafficOut.value)

type KubeSpanNode = Node<KubeSpanRootNodeData> | Node<KubeSpanPeerNodeData>

const nodes = computed<KubeSpanNode[]>(() => [buildRootNode(), ...peers.map(buildPeerNode)])
const edges = computed(() => peers.map(buildEdge))

function isOnline(peer: Resource<PeerStatusSpec>) {
  return peer.spec.state === 'up'
}

function buildRootNode(): Node<KubeSpanRootNodeData> {
  const height = 32

  return {
    id: 'root',
    type: 'root',
    width: 200,
    height,
    position: {
      x: 0,
      y: -height / 2,
    },
    selectable: false,
    data: {
      machine: selectedMachine,
      traffic: `${prettyBytes(totalTrafficIn.value)} / ${prettyBytes(totalTrafficOut.value)}`,
    },
  }
}

function buildPeerNode(peer: Resource<PeerStatusSpec>, index: number): Node<KubeSpanPeerNodeData> {
  const key = peer.metadata.id!
  const height = 66
  const spacing = 16

  const peerCount = peers.length
  const totalHeight = peerCount * height + (peerCount - 1) * spacing

  return {
    id: key,
    type: 'peer',
    width: 200,
    height,
    position: {
      x: 360,
      y: -totalHeight / 2 + index * (height + spacing),
    },
    data: {
      peer,
      isOnline: isOnline(peer),
      dimmed: !peerMatches.has(key),
      label:
        typeof machineNodenameMap.get(peer.spec.label!)?.metadata.labels?.[
          LabelControlPlaneRole
        ] === 'string'
          ? 'Control Plane'
          : 'Worker',
    },
  }
}

function buildEdge(peer: Resource<PeerStatusSpec>): Edge {
  const online = isOnline(peer)

  const traffic = (peer.spec.receiveBytes ?? 0) + (peer.spec.transmitBytes ?? 0)
  const trafficPct = totalTraffic.value ? traffic / totalTraffic.value : 0

  return {
    id: `edge-${peer.metadata.id}`,
    source: 'root',
    sourceHandle: 'right',
    target: peer.metadata.id!,
    targetHandle: 'left',
    type: 'default',
    style: {
      stroke: online ? 'var(--color-green-g1)' : 'var(--color-red-r1)',
      strokeWidth: 1.5 + trafficPct * 3,
      strokeDasharray: online ? undefined : '4 4',
      opacity: !online || !peerMatches.has(peer.metadata.id!) ? 0.35 : 0.7,
    },
  }
}

function isPeerGraphNode(node: GraphNode): node is GraphNode<KubeSpanPeerNodeData> {
  return node.type === 'peer'
}

onNodesInitialized(() => {
  fitView({ minZoom: 1, maxZoom: 1 })
})

onNodeClick(({ node }) => {
  if (isPeerGraphNode(node)) emit('peerClick', node.data.peer)
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

    <template #node-root="nodeProps">
      <KubeSpanRootNode v-bind="nodeProps" />
    </template>

    <template #node-peer="nodeProps">
      <KubeSpanPeerNode v-bind="nodeProps" />
    </template>
  </VueFlow>
</template>
