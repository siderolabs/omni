<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { NodeSpec as V1NodeSpec, NodeStatus as V1NodeStatus } from 'kubernetes-types/core/v1'
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { ClusterMachineStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineStatusLabelNodeName,
  ClusterMachineStatusType,
  DefaultNamespace,
  kubernetes,
  LabelCluster,
  TalosClusterNamespace,
  TalosMemberType,
} from '@/api/resources'
import TGroupAnimation from '@/components/Animation/TGroupAnimation.vue'
import TList from '@/components/List/TList.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import { getContext } from '@/context'
import { useResourceWatch } from '@/methods/useResourceWatch'
import NodesItem, { type TalosMemberSpec } from '@/views/Nodes/components/NodesItem.vue'

definePage({ name: 'Nodes' })

const route = useRoute()
const context = getContext()

const { data: v1Nodes } = useResourceWatch<V1NodeSpec, V1NodeStatus>({
  runtime: Runtime.Kubernetes,
  resource: {
    type: kubernetes.node,
  },
  context,
})

const { data: talosMembers } = useResourceWatch<TalosMemberSpec>({
  runtime: Runtime.Talos,
  resource: {
    type: TalosMemberType,
    namespace: TalosClusterNamespace,
  },
  context,
})

const v1NodesMap = computed(() =>
  Object.fromEntries(v1Nodes.value.map((n) => [n.metadata.name!, n])),
)

const talosMembersMap = computed(() =>
  Object.fromEntries(talosMembers.value.map((m) => [m.metadata.id!, m])),
)

function getNodeItem(
  item: Resource<ClusterMachineStatusSpec>,
): Resource<ClusterMachineStatusSpec & V1NodeSpec & TalosMemberSpec, V1NodeStatus> {
  const itemID = item.metadata.labels?.[ClusterMachineStatusLabelNodeName] ?? item.metadata.id!

  const v1node = v1NodesMap.value[itemID]
  const talosMember = talosMembersMap.value[itemID]

  return {
    ...item,
    spec: {
      ...item.spec,
      ...v1node?.spec,
      ...talosMember?.spec,
    },
    metadata: {
      ...item.metadata,
      labels: {
        ...item.metadata.labels,
        ...v1node?.metadata.labels,
        ...talosMember?.metadata.labels,
      },
    },
    status: v1node?.status,
  }
}
</script>

<template>
  <PageContainer class="flex w-full flex-col gap-4">
    <PageHeader title="All Nodes" />
    <TList
      :opts="{
        runtime: Runtime.Omni,
        resource: {
          type: ClusterMachineStatusType,
          namespace: DefaultNamespace,
        },
        selectors: [`${LabelCluster}=${route.params.cluster}`],
      }"
      search
      pagination
    >
      <template #default="{ items, searchQuery }">
        <div class="nodes-list">
          <div class="nodes-list-heading">
            <p>Name</p>
            <p>IP</p>
            <p>OS</p>
            <p>Roles</p>
            <p>Status</p>
          </div>
          <TGroupAnimation>
            <NodesItem
              v-for="item in items"
              :key="item.metadata.id!"
              :cluster-id="$route.params.cluster"
              :item="getNodeItem(item)"
              :search-option="searchQuery"
            />
          </TGroupAnimation>
        </div>
      </template>
    </TList>
  </PageContainer>
</template>

<style scoped>
@reference "../../../../index.css";

.nodes-list-heading {
  @apply flex items-center bg-naturals-n2;
  padding: 10px 16px;
}
.nodes-list-heading > p {
  @apply w-1/5 text-xs text-naturals-n13;
}
</style>
