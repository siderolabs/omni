<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useElementSize, useSessionStorage } from '@vueuse/core'
import { computed, useId, useTemplateRef } from 'vue'
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

const expanded = useSessionStorage(() => `cluster-expanded-${item.metadata.id}`, defaultOpen)

const {
  canDownloadKubeconfig,
  canDownloadTalosconfig,
  canAddClusterMachines,
  canRemoveClusterMachines,
} = setupClusterPermissions(computed(() => item.metadata.id!))

const locked = computed(() => item.metadata.annotations?.[ClusterLocked] !== undefined)

const regionId = useId()
const labelId = useId()

const slider = useTemplateRef('slider')
const { height } = useElementSize(slider)
</script>

<template>
  <div
    class="col-span-full grid grid-cols-subgrid overflow-hidden rounded border border-naturals-n5 text-xs"
  >
    <div
      tabindex="0"
      role="button"
      :aria-expanded="expanded"
      :aria-controls="regionId"
      :aria-labelledby="labelId"
      class="col-span-full grid cursor-pointer grid-cols-subgrid items-center bg-naturals-n1 p-4 pl-2 hover:bg-naturals-n3"
      @click="expanded = !expanded"
    >
      <div class="flex min-w-0 items-center gap-2">
        <TIcon
          :class="{ 'rotate-180': expanded }"
          class="size-5 shrink-0 rounded-md bg-naturals-n4 transition-transform duration-250 hover:text-naturals-n13"
          icon="drop-up"
          aria-hidden="true"
        />

        <RouterLink
          :id="labelId"
          :to="{ name: 'ClusterOverview', params: { cluster: item.metadata.id } }"
          class="list-item-link truncate"
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

      <div id="machine-count">
        {{ item.spec.machines?.healthy ?? 0 }}/{{ item.spec.machines?.total ?? 0 }}
      </div>

      <ClusterStatus :cluster="item" />

      <ItemLabels
        v-if="item"
        :resource="item"
        :add-label-func="addClusterLabels"
        :remove-label-func="removeClusterLabels"
        @filter-label="(label) => $emit('filterLabels', label)"
      />

      <div class="flex items-center justify-self-end">
        <Tooltip description="Open Cluster Dashboard" class="h-6">
          <IconButton
            icon="dashboard"
            aria-label="open dashboard"
            @click.stop="$router.push({ path: `/clusters/${item.metadata.id}` })"
          />
        </Tooltip>

        <TActionsBox aria-label="cluster actions" @click.stop>
          <TActionsBoxItem
            v-if="canAddClusterMachines"
            icon="nodes"
            @click="$router.push({ name: 'ClusterScale', params: { cluster: item.metadata.id } })"
          >
            Cluster Scaling
          </TActionsBoxItem>
          <TActionsBoxItem
            icon="settings"
            @click="
              $router.push({
                name: 'ClusterConfigPatches',
                params: { cluster: item.metadata.id },
              })
            "
          >
            Config Patches
          </TActionsBoxItem>
          <TActionsBoxItem
            icon="dashboard"
            @click="
              $router.push({ name: 'ClusterOverview', params: { cluster: item.metadata.id } })
            "
          >
            Open Dashboard
          </TActionsBoxItem>
          <TActionsBoxItem
            v-if="canDownloadKubeconfig"
            icon="kube-config"
            @click="downloadKubeconfig(item.metadata.id!)"
          >
            Download
            <code>kubeconfig</code>
          </TActionsBoxItem>
          <TActionsBoxItem
            v-if="canDownloadTalosconfig"
            icon="talos-config"
            @click="downloadTalosconfig(item.metadata.id)"
          >
            Download
            <code>talosconfig</code>
          </TActionsBoxItem>
          <TActionsBoxItem
            v-if="canRemoveClusterMachines"
            icon="delete"
            danger
            @click="$router.push({ query: { modal: 'clusterDestroy', cluster: item.metadata.id } })"
          >
            Destroy Cluster
          </TActionsBoxItem>
        </TActionsBox>
      </div>
    </div>

    <section
      :id="regionId"
      :aria-labelledby="labelId"
      class="col-span-full grid grid-cols-subgrid transition-all duration-300"
      :style="{ height: expanded ? `${height}px` : '0' }"
    >
      <ClusterMachines ref="slider" class="h-min" :cluster-i-d="item.metadata.id!" is-subgrid />
    </section>
  </div>
</template>
