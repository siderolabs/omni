<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useSessionStorage } from '@vueuse/core'
import { CollapsibleContent, CollapsibleRoot, CollapsibleTrigger } from 'reka-ui'
import { computed, ref, useId } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import type { Resource } from '@/api/grpc'
import type { ClusterStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterLocked } from '@/api/resources'
import TActionsBox from '@/components/common/ActionsBox/TActionsBox.vue'
import TActionsBoxItem from '@/components/common/ActionsBox/TActionsBoxItem.vue'
import IconButton from '@/components/common/Button/IconButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { downloadKubeconfig, downloadTalosconfig } from '@/methods'
import { setupClusterPermissions } from '@/methods/auth'
import { addClusterLabels, removeClusterLabels } from '@/methods/cluster'
import type { Label } from '@/methods/labels'
import ClusterMachines from '@/views/cluster/ClusterMachines/ClusterMachines.vue'
import ClusterStatus from '@/views/omni/Clusters/ClusterStatus.vue'
import ItemLabels from '@/views/omni/ItemLabels/ItemLabels.vue'
import ClusterDestroy from '@/views/omni/Modals/ClusterDestroy.vue'

const {
  item,
  defaultOpen,
  searchQuery = '',
} = defineProps<{
  item: Resource<ClusterStatusSpec>
  defaultOpen?: boolean
  searchQuery?: string
}>()

defineEmits<{
  filterLabels: [Label]
}>()

const expanded = useSessionStorage(
  () => `cluster-expanded-${item.metadata.id}`,
  () => defaultOpen,
)

const {
  canDownloadKubeconfig,
  canDownloadTalosconfig,
  canAddClusterMachines,
  canRemoveClusterMachines,
} = setupClusterPermissions(computed(() => item.metadata.id))

const locked = computed(() => item.metadata.annotations?.[ClusterLocked] !== undefined)

const regionId = useId()
const labelId = useId()

const clusterDestroyDialogOpen = ref(false)
</script>

<template>
  <CollapsibleRoot
    v-model:open="expanded"
    as="li"
    class="col-span-full grid grid-cols-subgrid overflow-hidden rounded border border-naturals-n5 text-xs"
    :aria-labelledby="labelId"
  >
    <CollapsibleTrigger
      :aria-labelledby="labelId"
      class="group/collapsible-trigger col-span-full grid grid-cols-subgrid items-center bg-naturals-n1 p-4 pl-2 text-left hover:bg-naturals-n3"
    >
      <div class="flex min-w-0 items-center gap-2">
        <TIcon
          class="size-5 shrink-0 rounded-md bg-naturals-n4 transition-transform duration-250 group-data-[state=open]/collapsible-trigger:rotate-180 hover:text-naturals-n13"
          icon="drop-up"
          aria-hidden="true"
        />

        <RouterLink
          :id="labelId"
          :to="{ name: 'ClusterOverview', params: { cluster: item.metadata.id! } }"
          class="list-item-link truncate"
          @click.stop
        >
          <WordHighlighter
            :query="searchQuery"
            :text-to-highlight="item.metadata.id"
            split-by-space
            highlight-class="bg-naturals-n14"
          />
        </RouterLink>

        <TIcon v-if="locked" class="size-3.5 shrink-0" icon="locked" aria-label="locked" />
      </div>

      <div data-testid="machine-count">
        {{ item.spec.machines?.healthy ?? 0 }}/{{ item.spec.machines?.total ?? 0 }}
      </div>

      <ClusterStatus :cluster="item" />

      <div class="flex items-center gap-2 text-naturals-n10">
        <Tooltip :description="`Talos version v${item.spec.talos_version}`">
          <span class="resource-label label-red flex items-center gap-1">
            <TIcon class="size-3.5 shrink-0" icon="talos" />
            {{ item.spec.talos_version }}
          </span>
        </Tooltip>

        <Tooltip :description="`Kubernetes version v${item.spec.kubernetes_version}`">
          <span class="resource-label label-blue flex items-center gap-1">
            <TIcon class="size-3.5 shrink-0" icon="kubernetes" />
            {{ item.spec.kubernetes_version }}
          </span>
        </Tooltip>
      </div>

      <div class="flex items-center justify-self-end">
        <Tooltip description="Open Cluster Dashboard" class="h-6">
          <IconButton
            icon="dashboard"
            aria-label="open dashboard"
            @click.stop="$router.push({ path: `/clusters/${item.metadata.id}` })"
          />
        </Tooltip>

        <TActionsBox aria-label="cluster actions">
          <TActionsBoxItem
            v-if="canAddClusterMachines"
            icon="nodes"
            @select="$router.push({ name: 'ClusterScale', params: { cluster: item.metadata.id! } })"
          >
            Cluster Scaling
          </TActionsBoxItem>
          <TActionsBoxItem
            icon="settings"
            @select="
              $router.push({
                name: 'ClusterConfigPatches',
                params: { cluster: item.metadata.id! },
              })
            "
          >
            Config Patches
          </TActionsBoxItem>
          <TActionsBoxItem
            icon="dashboard"
            @select="
              $router.push({ name: 'ClusterOverview', params: { cluster: item.metadata.id! } })
            "
          >
            Open Dashboard
          </TActionsBoxItem>
          <TActionsBoxItem
            v-if="canDownloadKubeconfig"
            icon="kube-config"
            @select="downloadKubeconfig(item.metadata.id!)"
          >
            Download
            <code>kubeconfig</code>
          </TActionsBoxItem>
          <TActionsBoxItem
            v-if="canDownloadTalosconfig"
            icon="talos-config"
            @select="downloadTalosconfig(item.metadata.id)"
          >
            Download
            <code>talosconfig</code>
          </TActionsBoxItem>
          <TActionsBoxItem
            v-if="canRemoveClusterMachines"
            icon="delete"
            danger
            aria-haspopup="dialog"
            @select="clusterDestroyDialogOpen = true"
          >
            Destroy Cluster
          </TActionsBoxItem>
        </TActionsBox>
      </div>
    </CollapsibleTrigger>

    <CollapsibleContent
      :id="regionId"
      as="section"
      :aria-labelledby="labelId"
      class="collapsible-content col-span-full grid grid-cols-subgrid"
    >
      <div class="col-span-full border-t border-naturals-n6 bg-naturals-n1 px-4 py-2">
        <ItemLabels
          :resource="item"
          :add-label-func="addClusterLabels"
          :remove-label-func="removeClusterLabels"
          @filter-label="(label) => $emit('filterLabels', label)"
        />
      </div>

      <ClusterMachines :cluster-i-d="item.metadata.id!" is-subgrid />
    </CollapsibleContent>

    <ClusterDestroy v-model:open="clusterDestroyDialogOpen" :cluster-id="item.metadata.id!" />
  </CollapsibleRoot>
</template>

<style scoped>
.collapsible-content[data-state='closed'] {
  animation: slideUp 200ms ease-out;
}

@keyframes slideUp {
  from {
    height: var(--reka-collapsible-content-height);
  }
  to {
    height: 0;
  }
}
</style>
