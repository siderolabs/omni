<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { NodeSpec as V1NodeSpec, NodeStatus as V1NodeStatus } from 'kubernetes-types/core/v1'
import { computed } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import type { Resource } from '@/api/grpc'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterMachineStatusLabelNodeName } from '@/api/resources'
import TStatus from '@/components/common/Status/TStatus.vue'
import Tag from '@/components/common/Tag/Tag.vue'
import { getStatus } from '@/methods'
import NodeContextMenu from '@/views/common/NodeContextMenu.vue'

type MemberSpec = {
  operatingSystem: string
  addresses: string[]
  nodeId?: string
}

const { item } = defineProps<{
  clusterId: string
  item: Resource<ClusterMachineStatusSpec & V1NodeSpec & MemberSpec, V1NodeStatus>
  searchOption?: string
}>()

const os = computed(() => {
  return item.spec.operatingSystem || 'unknown'
})

const nodeName = computed(() => {
  return (
    item.metadata.labels?.[ClusterMachineStatusLabelNodeName] ??
    item.spec.nodeId ??
    item.metadata.id
  )
})

const status = computed(() => getStatus(item))

const ip = computed(() => {
  return item.spec.addresses[0] ?? ''
})

const roles = computed(() =>
  Object.keys(item.metadata.labels ?? {})
    .filter((label) => label.includes('node-role.kubernetes.io/'))
    .map((label) => label.split('/')[1]),
)
</script>

<template>
  <div class="nodes-list-item">
    <p class="node-name">
      <RouterLink
        :to="{ name: 'NodeOverview', params: { cluster: clusterId, machine: item.metadata.id! } }"
      >
        <WordHighlighter
          :query="searchOption"
          :text-to-highlight="nodeName"
          highlight-class="bg-naturals-n14"
        />
      </RouterLink>
    </p>

    <p>
      <WordHighlighter
        :query="searchOption"
        :text-to-highlight="ip"
        highlight-class="bg-naturals-n14"
      />
    </p>
    <p>
      <WordHighlighter
        :query="searchOption"
        :text-to-highlight="os"
        highlight-class="bg-naturals-n14"
      />
    </p>
    <p class="flex flex-wrap">
      <Tag v-for="role in roles" :key="role" class="nodes-list-item-role">
        {{ role }}
      </Tag>
    </p>
    <p>
      <TStatus :title="status" />
    </p>
    <div class="nodes-list-item-menu -ml-6">
      <NodeContextMenu :cluster-machine-status="item" :cluster-name="clusterId" />
    </div>
  </div>
</template>

<style scoped>
@reference "../../../../index.css";

.nodes-list-item {
  @apply flex items-center border-b border-naturals-n4 px-4 py-4;
}

.nodes-list-item > p {
  @apply w-1/5 overflow-hidden text-xs text-ellipsis whitespace-nowrap;
}

.nodes-list-item > .node-name {
  @apply text-xs font-medium text-naturals-n14 transition hover:text-naturals-n10;
}
</style>
