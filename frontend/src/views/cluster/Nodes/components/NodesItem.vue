<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="nodes-list-item">
    <p class="node-name">
      <router-link :to="{ name: 'NodeOverview', params: { machine: item.metadata.id } }">
        <WordHighlighter
          :query="searchOption"
          :textToHighlight="nodeName"
          highlightClass="bg-naturals-N14"
        />
      </router-link>
    </p>

    <p>
      <WordHighlighter
        :query="searchOption"
        :textToHighlight="ip"
        highlightClass="bg-naturals-N14"
      />
    </p>
    <p>
      <WordHighlighter
        :query="searchOption"
        :textToHighlight="os"
        highlightClass="bg-naturals-N14"
      />
    </p>
    <p class="flex flex-wrap">
      <tag class="nodes-list-item-role" v-for="role in roles" :key="role">
        {{ role }}
      </tag>
    </p>
    <p>
      <t-status :title="status" />
    </p>
    <div class="nodes-list-item-menu -ml-6">
      <node-context-menu
        :cluster-machine-status="item"
        :cluster-name="route.params.cluster as string"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, toRefs } from 'vue'
import { useRoute } from 'vue-router'
import { getStatus } from '@/methods'
import { ClusterMachineStatusLabelNodeName } from '@/api/resources'
import type { Resource } from '@/api/grpc'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import type { V1NodeSpec, V1NodeStatus } from '@kubernetes/client-node'

import TStatus from '@/components/common/Status/TStatus.vue'
import Tag from '@/components/common/Tag/Tag.vue'
import WordHighlighter from 'vue-word-highlighter'
import NodeContextMenu from '@/views/common/NodeContextMenu.vue'

type MemberSpec = {
  operatingSystem: string
  addresses: string[]
  nodeId?: string
}

const props = defineProps<{
  item: Resource<ClusterMachineStatusSpec & V1NodeSpec & MemberSpec, V1NodeStatus>
  searchOption?: string
}>()

const { item } = toRefs(props)

const os = computed(() => {
  return item.value.spec.operatingSystem || 'unknown'
})

const nodeName = computed(() => {
  return (
    (item.value.metadata.labels || {})[ClusterMachineStatusLabelNodeName] ??
    item.value.spec.nodeId ??
    item.value.metadata.id
  )
})

const status = computed(() => getStatus(item.value))

const ip = computed(() => {
  return (item.value?.spec?.addresses ?? {})[0] ?? ''
})

const roles = computed(() => {
  const roles: any = []

  for (const label in item.value?.metadata.labels) {
    if (label.indexOf('node-role.kubernetes.io/') != -1) {
      roles.push(label.split('/')[1])
    }
  }

  return roles
})

const route = useRoute()
</script>

<style scoped>
.nodes-list-item {
  @apply flex items-center border-b border-naturals-N4 px-4 py-4;
}

.nodes-list-item > p {
  @apply text-xs overflow-hidden whitespace-nowrap overflow-ellipsis w-1/5;
}

.nodes-list-item > .node-name {
  @apply text-xs text-naturals-N14 font-medium hover:text-naturals-N10 transition;
}
</style>
