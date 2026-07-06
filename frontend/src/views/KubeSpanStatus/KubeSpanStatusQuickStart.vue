<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { RequestError } from '@/api/fetch.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type { ClusterSpec, ClusterStatusSpec, ConfigPatchSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import {
  ClusterStatusType,
  ClusterType,
  ConfigPatchName,
  ConfigPatchType,
  DefaultNamespace,
  LabelCluster,
} from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import CodeBlock from '@/components/CodeBlock/CodeBlock.vue'
import type { IconType } from '@/components/Icon/TIcon.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TAlert from '@/components/TAlert.vue'
import { getDocsLink } from '@/methods'
import { useClusterPermissions } from '@/methods/auth'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'

const { clusterId } = defineProps<{
  clusterId: string
}>()

const { canManageConfigPatches } = useClusterPermissions(() => clusterId)

const { data: cluster } = useResourceWatch<ClusterSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterType,
    id: clusterId,
  },
}))

const talosVersion = computed(() => cluster.value?.spec.talos_version)

const { data: clusterStatus } = useResourceWatch<ClusterStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterStatusType,
    id: clusterId,
  },
}))

// KubeSpan's WireGuard overhead scales with the number of peers; large clusters
// are generally advised against enabling a full mesh.
const LARGE_CLUSTER_THRESHOLD = 50
const nodeCount = computed(() => clusterStatus.value?.spec.machines?.total ?? 0)
const isLargeCluster = computed(() => nodeCount.value >= LARGE_CLUSTER_THRESHOLD)

const features: { icon: IconType; title: string; description: string }[] = [
  {
    icon: 'locked',
    title: 'Encrypted mesh',
    description:
      'All node-to-node traffic is encrypted with a WireGuard mesh, securing communication regardless of the underlying network.',
  },
  {
    icon: 'server-network',
    title: 'Cross-network connectivity',
    description:
      'Nodes spanning multiple networks — cloud, on-prem, or edge — can reach each other, even across NAT.',
  },
  {
    icon: 'cloud-connection',
    title: 'Automatic peer discovery',
    description:
      'Peers discover one another and manage endpoints automatically, with no manual routing or VPN setup.',
  },
]

// The Talos machine config patch that enables KubeSpan for the whole cluster.
// Cluster discovery must be enabled for KubeSpan to work; it is on by default,
// but we set it explicitly to be safe.
const kubeSpanPatch = `machine:
  network:
    kubespan:
      enabled: true
cluster:
  discovery:
    enabled: true
`

const confirmOpen = ref(false)
const enabling = ref(false)
const requested = ref(false)

async function enableKubeSpan() {
  enabling.value = true

  const patch: Resource<ConfigPatchSpec> = {
    metadata: {
      namespace: DefaultNamespace,
      type: ConfigPatchType,
      id: `500-${clusterId}-kubespan`,
      labels: {
        [LabelCluster]: clusterId,
      },
      annotations: {
        [ConfigPatchName]: 'Enable KubeSpan',
      },
    },
    spec: {
      data: kubeSpanPatch,
    },
  }

  try {
    await ResourceService.Create(patch, withRuntime(Runtime.Omni))

    requested.value = true
    confirmOpen.value = false
    showSuccess(
      'Enabling KubeSpan',
      'The config patch was applied. Nodes may gracefully reboot before KubeSpan becomes active.',
    )
  } catch (e) {
    if (e instanceof RequestError) {
      switch (e.code) {
        case Code.ALREADY_EXISTS:
          requested.value = true
          confirmOpen.value = false
          showSuccess(
            'Enabling KubeSpan',
            'A KubeSpan config patch already exists for this cluster.',
          )
          return
        case Code.INVALID_ARGUMENT:
          showError('The Config is Invalid', e.message.replace('failed to validate: ', ''))
          return
      }
    }

    showError('Failed to Enable KubeSpan', e instanceof Error ? e.message : String(e))
  } finally {
    enabling.value = false
  }
}
</script>

<template>
  <PageContainer class="@container flex h-full flex-col overflow-y-auto">
    <div class="mx-auto flex flex-col items-center">
      <div class="flex flex-col items-center gap-3 text-center">
        <div class="flex size-14 items-center justify-center rounded-full bg-naturals-n3">
          <TIcon icon="cloud-connection" class="size-7 text-primary-p3" />
        </div>

        <h1 class="text-xl font-medium text-naturals-n14">KubeSpan is not enabled</h1>

        <p class="max-w-xl text-sm text-naturals-n11">
          KubeSpan creates an encrypted WireGuard mesh between the nodes of this cluster, letting
          machines on different networks discover and securely communicate with each other. It is
          currently disabled for this cluster.
        </p>

        <a
          :href="getDocsLink('talos', '/learn-more/kubespan', { talosVersion })"
          target="_blank"
          rel="noopener noreferrer"
          class="link-primary inline-flex items-center gap-1 text-sm"
        >
          Learn more about KubeSpan
          <TIcon icon="external-link" class="size-3.5" />
        </a>
      </div>

      <div
        class="flex max-w-3xl flex-col gap-6 py-6 @4xl:max-w-5xl @4xl:flex-row @4xl:items-start @4xl:gap-8"
      >
        <div class="grid gap-3 @2xl:grid-cols-3 @4xl:flex-1 @4xl:grid-cols-1">
          <div
            v-for="feature in features"
            :key="feature.title"
            class="flex flex-col gap-2 rounded-lg bg-naturals-n2 p-4 @2xl:items-center @2xl:text-center @4xl:items-start @4xl:text-left"
          >
            <div class="flex items-center gap-2">
              <TIcon :icon="feature.icon" class="size-5 text-naturals-n13" />
              <h3 class="text-sm font-medium text-naturals-n14">{{ feature.title }}</h3>
            </div>
            <p class="text-xs text-naturals-n11">{{ feature.description }}</p>
          </div>
        </div>

        <div class="flex flex-col gap-4 rounded-lg bg-naturals-n2 p-6 @4xl:flex-1">
          <div class="flex flex-col gap-1">
            <h2 class="text-base font-medium text-naturals-n14">Enable KubeSpan</h2>
            <p class="text-sm text-naturals-n11">
              This applies the following cluster-wide config patch to every node. Patches are
              applied immediately and may result in graceful reboots.
            </p>
          </div>

          <CodeBlock>{{ kubeSpanPatch }}</CodeBlock>

          <div class="flex flex-wrap items-center gap-3">
            <TButton
              variant="highlighted"
              icon="cloud-connection"
              :disabled="!canManageConfigPatches || requested"
              @click="confirmOpen = true"
            >
              {{ requested ? 'KubeSpan enabling…' : 'Enable KubeSpan' }}
            </TButton>

            <span v-if="!canManageConfigPatches" class="text-xs text-naturals-n9">
              You do not have permission to manage config patches for this cluster.
            </span>
            <span v-else-if="requested" class="text-xs text-naturals-n9">
              KubeSpan status will appear here once the configuration is applied.
            </span>
          </div>
        </div>
      </div>
    </div>

    <ConfirmModal
      v-model:open="confirmOpen"
      title="Enable KubeSpan"
      action-label="Enable KubeSpan"
      :loading="enabling"
      :disabled="!canManageConfigPatches"
      content-class="max-w-xl"
      @confirm="enableKubeSpan"
    >
      <template #description>Cluster {{ clusterId }}</template>

      <ManagedByTemplatesWarning warning-style="popup" :resource="cluster" />

      <p class="text-sm text-naturals-n11">
        A cluster-wide config patch enabling KubeSpan will be created.
        <br />
        This applies immediately and may trigger graceful reboots of the cluster nodes.
        <br />
        Please confirm the action.
      </p>

      <div class="mt-3">
        <TAlert v-if="isLargeCluster" type="warn" title="Large cluster">
          This cluster has {{ nodeCount }} nodes. Enabling KubeSpan on clusters with
          {{ LARGE_CLUSTER_THRESHOLD }} or more nodes is generally not recommended — the WireGuard
          mesh overhead grows with the number of peers and can noticeably degrade network
          performance.
        </TAlert>

        <p v-else class="flex max-w-md items-start gap-1.5 text-xs text-yellow-y1">
          <TIcon icon="warning" class="mt-px size-3.5 shrink-0" />
          <span>
            Wireguard encryption adds overhead to each packet that can reduce network throughput.
            Review the docs to decide if it fits your cluster.
          </span>
        </p>
      </div>
    </ConfirmModal>
  </PageContainer>
</template>
