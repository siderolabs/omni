<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, ref, toRefs } from 'vue'
import { useRouter } from 'vue-router'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { ClusterStatusSpec, MachineSetSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterLocked,
  DefaultNamespace,
  LabelCluster,
  LabelMachineSet,
  MachineSetNodeType,
  MachineSetType,
} from '@/api/resources'
import WatchResource from '@/api/watch'
import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/common/ActionsBox/TActionsBoxItem.vue'
import IconButton from '@/components/common/Button/IconButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import ListItemBox from '@/components/common/List/ListItemBox.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { downloadKubeconfig, downloadTalosconfig } from '@/methods'
import { setupClusterPermissions } from '@/methods/auth'
import { addClusterLabels, removeClusterLabels } from '@/methods/cluster'
import type { Label } from '@/methods/labels'
import { controlPlaneMachineSetId } from '@/methods/machineset'
import ClusterMachines from '@/views/cluster/ClusterMachines/ClusterMachines.vue'
import ClusterStatus from '@/views/omni/Clusters/ClusterStatus.vue'
import ItemLabels from '@/views/omni/ItemLabels/ItemLabels.vue'

const box = ref<{ collapsed: boolean }>()

const props = defineProps<{
  item: Resource<ClusterStatusSpec>
  defaultOpen?: boolean
  searchQuery?: string
}>()

defineEmits<{
  filterLabels: [Label]
}>()

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

const locked = computed(() => {
  return item?.value?.metadata.annotations?.[ClusterLocked] !== undefined
})
</script>

<template>
  <ListItemBox
    :id="`${item.metadata.id}-cluster-box`"
    ref="box"
    :item-i-d="item.metadata.id!"
    list-i-d="cluster"
  >
    <template #default>
      <div class="clusters-grid flex-1">
        <div class="flex items-center gap-1">
          <RouterLink
            :to="{ name: 'ClusterOverview', params: { cluster: item?.metadata.id } }"
            class="list-item-link"
          >
            <WordHighlighter
              :query="searchQuery ?? ''"
              :text-to-highlight="item.metadata.id"
              split-by-space
              highlight-class="bg-naturals-n14"
            />
          </RouterLink>
          <TIcon v-if="locked" class="size-3.5 cursor-pointer" icon="locked" />
        </div>
        <div id="machine-count" class="ml-3">
          {{ item?.spec?.machines?.healthy ?? 0 }}/{{ item?.spec?.machines?.total ?? 0 }}
        </div>
        <div>
          <ClusterStatus :cluster="item" />
        </div>
        <ItemLabels
          v-if="item"
          class="ml-5"
          :resource="item"
          :add-label-func="addClusterLabels"
          :remove-label-func="removeClusterLabels"
          @filter-label="(label) => $emit('filterLabels', label)"
        />
      </div>
      <Tooltip description="Open Cluster Dashboard" class="h-6">
        <IconButton
          icon="dashboard"
          @click.stop="$router.push({ path: `/clusters/${item?.metadata.id}` })"
        />
      </Tooltip>
      <TActionsBox style="height: 24px" @click.stop>
        <TActionsBoxItem
          v-if="canAddClusterMachines"
          icon="nodes"
          @click="
            () => $router.push({ name: 'ClusterScale', params: { cluster: item?.metadata?.id } })
          "
          >Cluster Scaling
        </TActionsBoxItem>
        <TActionsBoxItem
          icon="settings"
          @click="
            () =>
              $router.push({
                name: 'ClusterConfigPatches',
                params: { cluster: item?.metadata?.id },
              })
          "
          >Config Patches
        </TActionsBoxItem>
        <TActionsBoxItem
          icon="dashboard"
          @click="
            () => $router.push({ name: 'ClusterOverview', params: { cluster: item?.metadata?.id } })
          "
          >Open Dashboard
        </TActionsBoxItem>
        <TActionsBoxItem
          v-if="canDownloadKubeconfig"
          icon="kube-config"
          @click="() => downloadKubeconfig(item?.metadata?.id as string)"
          >Download <code>kubeconfig</code></TActionsBoxItem
        >
        <TActionsBoxItem
          v-if="canDownloadTalosconfig"
          icon="talos-config"
          @click="() => downloadTalosconfig(item?.metadata?.id)"
          >Download <code>talosconfig</code></TActionsBoxItem
        >
        <TActionsBoxItem
          v-if="canRemoveClusterMachines"
          icon="delete"
          danger
          @click="openClusterDestroy"
          >Destroy Cluster</TActionsBoxItem
        >
      </TActionsBox>
    </template>
    <template #details>
      <ClusterMachines :cluster-i-d="item.metadata.id!" />
    </template>
  </ListItemBox>
</template>

<style>
@reference "../../../index.css";

.clusters-grid {
  @apply grid grid-cols-4 items-center gap-2 pr-2;
}

.clusters-grid > * {
  @apply truncate text-xs;
}
</style>
