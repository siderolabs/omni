<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <list-item-box
    :itemID="item.metadata.id!"
    :id="`${item.metadata.id}-cluster-box`"
    listID="cluster"
    ref="box"
  >
    <template #default>
      <div class="clusters-grid flex-1">
        <div class="list-item-link">
          <router-link :to="{ name: 'ClusterOverview', params: { cluster: item?.metadata.id } }">
            <word-highlighter
              :query="searchQuery ?? ''"
              :textToHighlight="item.metadata.id"
              split-by-space
              highlightClass="bg-naturals-N14"
            />
          </router-link>
        </div>
        <div class="ml-3" id="machine-count">
          {{ item?.spec?.machines?.healthy ?? 0 }}/{{ item?.spec?.machines?.total ?? 0 }}
        </div>
        <div />
        <item-labels
          class="ml-5"
          v-if="item"
          :resource="item"
          :add-label-func="addClusterLabels"
          :remove-label-func="removeClusterLabels"
          @filter-label="(e) => $emit('filterLabels', e)"
        />
      </div>
      <tooltip description="Open Cluster Dashboard" class="h-6">
        <icon-button
          icon="dashboard"
          @click.stop="() => $router.push({ path: '/cluster/' + item?.metadata.id + '/overview' })"
        />
      </tooltip>
      <t-actions-box style="height: 24px" @click.stop>
        <t-actions-box-item
          icon="nodes"
          @click="
            () => $router.push({ name: 'ClusterScale', params: { cluster: item?.metadata?.id } })
          "
          v-if="canAddClusterMachines"
          >Cluster Scaling
        </t-actions-box-item>
        <t-actions-box-item
          icon="settings"
          @click="
            () =>
              $router.push({
                name: 'ClusterConfigPatches',
                params: { cluster: item?.metadata?.id },
              })
          "
          >Config Patches
        </t-actions-box-item>
        <t-actions-box-item
          icon="dashboard"
          @click="
            () => $router.push({ name: 'ClusterOverview', params: { cluster: item?.metadata?.id } })
          "
          >Open Dashboard
        </t-actions-box-item>
        <t-actions-box-item
          icon="kube-config"
          @click="() => downloadKubeconfig(item?.metadata?.id as string)"
          v-if="canDownloadKubeconfig"
          >Download <code>kubeconfig</code></t-actions-box-item
        >
        <t-actions-box-item
          icon="talos-config"
          @click="() => downloadTalosconfig(item?.metadata?.id)"
          v-if="canDownloadTalosconfig"
          >Download <code>talosconfig</code></t-actions-box-item
        >
        <t-actions-box-item
          icon="delete"
          danger
          @click="openClusterDestroy"
          v-if="canRemoveClusterMachines"
          >Destroy Cluster</t-actions-box-item
        >
      </t-actions-box>
    </template>
    <template #details>
      <cluster-machines :clusterID="item.metadata.id!" />
    </template>
  </list-item-box>
</template>

<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, toRefs, ref } from 'vue'
import {
  DefaultNamespace,
  LabelCluster,
  LabelMachineSet,
  MachineSetNodeType,
  MachineSetType,
} from '@/api/resources'
import { downloadKubeconfig, downloadTalosconfig } from '@/methods'
import { useRouter } from 'vue-router'
import { addClusterLabels, removeClusterLabels } from '@/methods/cluster'
import { controlPlaneMachineSetId } from '@/methods/machineset'
import type { Resource } from '@/api/grpc'
import type { ClusterStatusSpec, MachineSetSpec } from '@/api/omni/specs/omni.pb'
import { Runtime } from '@/api/common/omni.pb'
import WatchResource from '@/api/watch'

import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/common/ActionsBox/TActionsBoxItem.vue'
import ClusterMachines from '@/views/cluster/ClusterMachines/ClusterMachines.vue'
import IconButton from '@/components/common/Button/IconButton.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import WordHighlighter from 'vue-word-highlighter'
import ItemLabels from '@/views/omni/ItemLabels/ItemLabels.vue'
import { setupClusterPermissions } from '@/methods/auth'

import ListItemBox from '@/components/common/List/ListItemBox.vue'

const box = ref<{ collapsed: boolean }>()

const props = defineProps<{
  item: Resource<ClusterStatusSpec>
  defaultOpen?: boolean
  searchQuery?: string
}>()

defineEmits(['filterLabels'])

const { item } = toRefs(props)

const router = useRouter()

const openClusterDestroy = () => {
  router.push({
    query: { modal: 'clusterDestroy', cluster: item.value.metadata.id },
  })
}

const machineSets: Ref<Resource<MachineSetSpec>[]> = ref([])
const machineSetsWatch = new WatchResource(machineSets)
machineSetsWatch.setup(
  computed(() => {
    if (box.value?.collapsed) {
      return undefined
    }

    return {
      resource: {
        type: MachineSetType,
        namespace: DefaultNamespace,
      },
      runtime: Runtime.Omni,
      selectors: [`${LabelCluster}=${item.value.metadata.id!}`],
    }
  }),
)

const {
  canDownloadKubeconfig,
  canDownloadTalosconfig,
  canAddClusterMachines,
  canRemoveClusterMachines,
} = setupClusterPermissions(computed(() => item.value.metadata.id!))

const controlPlaneNodes = ref<Resource[]>([])

const machineNodesWatch = new WatchResource(controlPlaneNodes)
machineNodesWatch.setup(
  computed(() => {
    if (box.value?.collapsed) {
      return undefined
    }

    return {
      resource: {
        type: MachineSetNodeType,
        namespace: DefaultNamespace,
      },
      runtime: Runtime.Omni,
      selectors: [`${LabelMachineSet}=${controlPlaneMachineSetId(item.value.metadata.id!)}`],
    }
  }),
)
</script>

<style>
.clusters-grid {
  @apply items-center grid grid-cols-4 pr-2 gap-2;
}

.clusters-grid > * {
  @apply text-xs truncate;
}
</style>
