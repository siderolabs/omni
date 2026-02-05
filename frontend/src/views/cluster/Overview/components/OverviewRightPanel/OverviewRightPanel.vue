<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import * as semver from 'semver'
import { computed, markRaw, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type {
  ClusterDiagnosticsSpec,
  ClusterDiagnosticsSpecNode,
  ClusterStatusSpec,
  ControlPlaneStatusSpec,
  EtcdBackupStatusSpec,
  KubernetesStatusSpec,
  KubernetesUpgradeStatusSpec,
  TalosUpgradeStatusSpec,
} from '@/api/omni/specs/omni.pb'
import {
  ClusterDiagnosticsType,
  ClusterLocked,
  ClusterStatusType,
  ControlPlaneStatusType,
  DefaultNamespace,
  EtcdBackupStatusType,
  KubernetesStatusType,
  MetricsNamespace,
} from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import type { BackupsStatus } from '@/methods'
import { downloadKubeconfig, downloadTalosconfig } from '@/methods'
import { setupClusterPermissions } from '@/methods/auth'
import { triggerEtcdBackup, updateClusterLock } from '@/methods/cluster'
import { controlPlaneMachineSetId } from '@/methods/machineset'
import { formatISO } from '@/methods/time'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showWarning } from '@/notification'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'
import OverviewOIDCToast from '@/views/cluster/Overview/components/OverviewRightPanel/OverviewOIDCToast.vue'
import OverviewRightPanelCondition from '@/views/cluster/Overview/components/OverviewRightPanel/OverviewRightPanelCondition.vue'
import OverviewRightPanelItem from '@/views/cluster/Overview/components/OverviewRightPanel/OverviewRightPanelItem.vue'
import TClusterStatus from '@/views/omni/Clusters/ClusterStatus.vue'

const { kubernetesUpgradeStatus, talosUpgradeStatus } = defineProps<{
  kubernetesUpgradeStatus?: Resource<KubernetesUpgradeStatusSpec>
  talosUpgradeStatus?: Resource<TalosUpgradeStatusSpec>
  etcdBackups?: BackupsStatus
}>()

const route = useRoute()
const router = useRouter()

const { data: clusterStatus } = useResourceWatch<ClusterStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterStatusType,
    id: route.params.cluster as string,
  },
}))

const { data: backupStatus } = useResourceWatch<EtcdBackupStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: MetricsNamespace,
    type: EtcdBackupStatusType,
    id: route.params.cluster as string,
  },
}))

const { data: controlPlaneStatus } = useResourceWatch<ControlPlaneStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ControlPlaneStatusType,
    id: controlPlaneMachineSetId(route.params.cluster as string),
  },
}))

const { data: kubernetesStatus } = useResourceWatch<KubernetesStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: KubernetesStatusType,
    id: route.params.cluster as string,
  },
}))

const { data: clusterDiagnostics } = useResourceWatch<ClusterDiagnosticsSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterDiagnosticsType,
    id: route.params.cluster as string,
  },
}))

const numNodesWithDiagnostics = computed(() => {
  return clusterDiagnostics.value?.spec.nodes?.length || 0
})

const numTotalDiagnostics = computed(() => {
  const nodes: ClusterDiagnosticsSpecNode[] = clusterDiagnostics.value?.spec.nodes || []
  return nodes.reduce((sum, node) => sum + (node.num_diagnostics || 0), 0) || 0
})

enum Update {
  Talos,
  Kubernetes,
}

const lastBackupError = computed(() => {
  return backupStatus.value?.spec.error
})

const backupTime = computed(() => {
  const t = backupStatus.value?.spec.last_backup_time

  if (!t) {
    return 'Never'
  }

  return formatISO(t, 'yyyy-LL-dd HH:mm:ss')
})

const startingEtcdBackup = ref(false)

const runEtcdBackup = async () => {
  startingEtcdBackup.value = true

  try {
    await triggerEtcdBackup(route.params.cluster as string)
  } catch (e) {
    showError('Failed to Trigger Manual Etcd Backup', e.message)
  }

  startingEtcdBackup.value = false
}

const kubernetesUpgradeAvailable = () => {
  return (kubernetesUpgradeStatus?.spec.upgrade_versions?.length ?? 0) > 0
}

const talosUpdateAvailable = () => {
  return (talosUpgradeStatus?.spec.upgrade_versions?.length ?? 0) > 0
}

const getUpgradeAvailable = (spec: {
  last_upgrade_version?: string
  current_upgrade_version?: string
  upgrade_versions?: string[]
}) => {
  if (spec.current_upgrade_version || !spec.upgrade_versions || !spec.last_upgrade_version) {
    return []
  }

  return spec.upgrade_versions.filter((version: string) => {
    return semver.compare(spec.last_upgrade_version!, version) === -1
  })
}

const kubernetesVersion = computed(() => {
  return getVersion(kubernetesUpgradeStatus!.spec)
})

const talosVersion = computed(() => {
  return getVersion(talosUpgradeStatus!.spec)
})

const newKubernetesVersionsAvailable = computed(() => {
  return getUpgradeAvailable(kubernetesUpgradeStatus!.spec)
})

const newTalosVersionsAvailable = computed(() => {
  return getUpgradeAvailable(talosUpgradeStatus!.spec)
})

const getVersion = (spec: { last_upgrade_version?: string; current_upgrade_version?: string }) => {
  if (spec.current_upgrade_version && spec.last_upgrade_version) {
    return `${spec.last_upgrade_version} ⇾ ${spec.current_upgrade_version}`
  }

  return spec.last_upgrade_version
}

const openClusterUpdate = (type: Update, locked: boolean) => {
  if (locked) {
    return
  }

  if (type === Update.Kubernetes && !canUpdateKubernetes.value) {
    return
  }

  if (type === Update.Talos && !canUpdateTalos.value) {
    return
  }

  const modal = type === Update.Talos ? 'updateTalos' : 'updateKubernetes'

  router.push({ query: { modal: modal, cluster: clusterStatus.value?.metadata.id } })
}

const locked = computed(() => {
  return clusterStatus.value?.metadata.annotations?.[ClusterLocked] !== undefined
})

const updateLock = async () => {
  if (!clusterStatus.value?.metadata.id) {
    return
  }

  try {
    await updateClusterLock(clusterStatus.value.metadata.id, !locked.value)
  } catch (e) {
    showError('Failed To Update Cluster Lock', e.message)
  }
}

const {
  canUpdateKubernetes,
  canUpdateTalos,
  canDownloadKubeconfig,
  canDownloadTalosconfig,
  canDownloadSupportBundle,
  canAddClusterMachines,
  canRemoveClusterMachines,
} = setupClusterPermissions(computed(() => route.params.cluster as string))
</script>

<template>
  <div class="mb-4 h-auto max-w-[20%] min-w-67 rounded bg-naturals-n2 py-5">
    <div class="flex flex-col gap-4 px-2 lg:px-6">
      <h3 class="text-sm text-naturals-n13">Cluster Details</h3>
      <ManagedByTemplatesWarning warning-style="short" />
      <OverviewRightPanelItem
        v-if="clusterStatus?.metadata.created"
        name="Created"
        :value="formatISO(clusterStatus.metadata.created, 'yyyy-LL-dd HH:mm:ss')"
      />
      <OverviewRightPanelItem v-if="clusterStatus" name="Status">
        <TClusterStatus :cluster="clusterStatus" />
      </OverviewRightPanelItem>
      <OverviewRightPanelItem
        v-if="clusterStatus?.spec.machines?.total"
        name="Machines Healthy"
        :value="`${clusterStatus.spec.machines.healthy ?? 0}/${clusterStatus.spec.machines.total}`"
      />
      <OverviewRightPanelItem v-if="numNodesWithDiagnostics > 0" name="Node Warnings">
        <div class="flex items-center gap-1">
          {{ numTotalDiagnostics }} (in {{ numNodesWithDiagnostics }} nodes)
          <Tooltip
            description="Some machines have diagnostic warnings. See the machines section for details."
          >
            <TIcon class="h-4 w-4 text-yellow-y1" icon="warning" />
          </Tooltip>
        </div>
      </OverviewRightPanelItem>
      <OverviewRightPanelItem v-if="talosUpgradeStatus" name="Talos Version">
        <div>{{ talosVersion }}</div>
        <Tooltip
          v-if="newTalosVersionsAvailable?.length"
          :description="
            locked
              ? `Newer Talos versions are available: ${newTalosVersionsAvailable.join(', ')}. However, Talos updates are disabled when the cluster is locked.`
              : `Newer Talos versions are available: ${newTalosVersionsAvailable.join(', ')}`
          "
        >
          <TIcon
            v-if="newTalosVersionsAvailable"
            class="h-4 w-4 cursor-pointer"
            icon="upgrade-available"
            @click="openClusterUpdate(Update.Talos, locked)"
          />
        </Tooltip>
      </OverviewRightPanelItem>
      <OverviewRightPanelItem v-if="kubernetesUpgradeStatus" name="Kubernetes Version">
        <div>{{ kubernetesVersion }}</div>
        <Tooltip
          v-if="newKubernetesVersionsAvailable?.length"
          :description="
            locked
              ? `Newer Kubernetes versions are available: ${newKubernetesVersionsAvailable.join(', ')}. However, Kubernetes updates are disabled when the cluster is locked.`
              : `Newer Kubernetes versions are available: ${newKubernetesVersionsAvailable.join(', ')}`
          "
        >
          <TIcon
            class="h-4 w-4 cursor-pointer"
            icon="upgrade-available"
            @click="openClusterUpdate(Update.Kubernetes, locked)"
          />
        </Tooltip>
      </OverviewRightPanelItem>
    </div>
    <template v-if="clusterStatus?.spec">
      <div class="my-3 h-px bg-naturals-n4" />
      <div class="flex flex-col gap-4 px-2 lg:px-6">
        <h3 class="text-sm text-naturals-n13">Control Plane</h3>
        <OverviewRightPanelItem name="Ready">
          <span :class="clusterStatus.spec.controlplaneReady ? '' : 'text-red-r1'">
            {{ clusterStatus.spec.controlplaneReady ? 'Yes' : 'No' }}
          </span>
        </OverviewRightPanelItem>
        <OverviewRightPanelItem v-if="clusterStatus.metadata.created" name="Last Backup">
          <div v-if="startingEtcdBackup" class="flex gap-1">
            Starting...
            <TSpinner class="h-4 w-4" />
          </div>
          <template v-else>
            <Tooltip v-if="lastBackupError">
              <template #description>
                <div class="max-w-lg wrap-break-word">{{ lastBackupError }}</div>
              </template>
              <TIcon class="h-4 w-4 text-yellow-y1" icon="warning" />
            </Tooltip>
            {{ backupTime }}
            <Tooltip v-if="etcdBackups?.enabled" description="Trigger Etcd Backup">
              <TIcon
                class="h-4 w-4 cursor-pointer text-green-g1"
                icon="play-circle"
                @click="runEtcdBackup"
              />
            </Tooltip>
          </template>
        </OverviewRightPanelItem>
        <OverviewRightPanelCondition
          v-for="condition in controlPlaneStatus?.spec.conditions"
          :key="condition.type?.toString()"
          :condition="condition"
        />
      </div>
      <div class="my-3 h-px bg-naturals-n4" />
      <div class="flex flex-col gap-4 px-2 lg:px-6">
        <h3 class="text-sm text-naturals-n13">Kubernetes</h3>
        <OverviewRightPanelItem
          name="API Available"
          :value="clusterStatus.spec.kubernetesAPIReady ? 'Yes' : 'No'"
        />
        <OverviewRightPanelItem
          v-if="kubernetesStatus && kubernetesStatus.spec.nodes"
          name="Nodes"
          :value="kubernetesStatus.spec.nodes.length ?? 0"
        />
      </div>
      <div class="my-3 h-px bg-naturals-n4" />
      <div class="flex flex-col gap-4 px-2 lg:px-6">
        <TButton
          :disabled="!canDownloadKubeconfig"
          type="primary"
          icon="kube-config"
          icon-position="left"
          @click="
            () => {
              downloadKubeconfig(clusterStatus!.metadata.id!)
              showWarning('Note on kubectl', { description: markRaw(OverviewOIDCToast) })
            }
          "
        >
          Download
          <code>kubeconfig</code>
        </TButton>

        <TButton
          :disabled="!canDownloadTalosconfig"
          type="primary"
          icon="talos-config"
          icon-position="left"
          @click="() => downloadTalosconfig(clusterStatus!.metadata.id!)"
        >
          Download
          <code>talosconfig</code>
        </TButton>

        <TButton
          is="router-link"
          :disabled="!canDownloadSupportBundle"
          type="primary"
          icon="lifebuoy"
          icon-position="left"
          :to="{
            query: { modal: 'downloadSupportBundle', cluster: clusterStatus.metadata.id },
          }"
        >
          Download Support Bundle
        </TButton>

        <TButton
          is="router-link"
          type="primary"
          icon="kube-config"
          icon-position="left"
          :to="{
            query: { modal: 'exportClusterTemplate' },
          }"
        >
          Export Cluster Template
        </TButton>
      </div>
      <div class="my-3 h-px bg-naturals-n4" />
      <div class="flex flex-col gap-4 px-2 lg:px-6">
        <Tooltip
          class="grow"
          :disabled="!locked"
          :description="`Cluster scaling is disabled when the cluster is locked.`"
        >
          <TButton
            is="router-link"
            type="highlighted"
            icon="nodes"
            icon-position="left"
            :disabled="!canAddClusterMachines || locked"
            :to="{
              name: 'ClusterScale',
              params: { cluster: clusterStatus.metadata.id },
            }"
          >
            Cluster Scaling
          </TButton>
        </Tooltip>

        <Tooltip
          class="grow"
          :disabled="!locked"
          :description="`Kubernetes updates are disabled when the cluster is locked.`"
        >
          <TButton
            type="primary"
            icon="kubernetes"
            icon-position="left"
            :disabled="!canUpdateKubernetes || !kubernetesUpgradeAvailable() || locked"
            @click="openClusterUpdate(Update.Kubernetes, locked)"
          >
            Update Kubernetes
          </TButton>
        </Tooltip>

        <Tooltip
          class="grow"
          :disabled="!locked"
          :description="`Talos updates are disabled when the cluster is locked.`"
        >
          <TButton
            type="primary"
            icon="sidero-monochrome"
            icon-position="left"
            :disabled="!canUpdateTalos || !talosUpdateAvailable() || locked"
            @click="openClusterUpdate(Update.Talos, locked)"
          >
            Update Talos
          </TButton>
        </Tooltip>

        <TButton
          is="router-link"
          type="primary"
          icon="settings"
          icon-position="left"
          :to="{
            name: 'ClusterConfigPatches',
            params: { cluster: clusterStatus.metadata.id },
          }"
        >
          Config Patches
        </TButton>

        <TButton
          type="primary"
          :icon="locked ? 'locked' : 'unlocked'"
          icon-position="left"
          @click="updateLock"
        >
          {{ locked ? 'Unlock Cluster' : 'Lock Cluster' }}
        </TButton>

        <Tooltip
          class="grow"
          :disabled="!locked"
          :description="`Cluster deletion is disabled when the cluster is locked.`"
        >
          <TButton
            is="router-link"
            class="text-red-r1"
            type="secondary"
            icon="delete"
            icon-position="left"
            :disabled="!canRemoveClusterMachines || locked"
            :to="{
              query: { modal: 'clusterDestroy', cluster: clusterStatus.metadata.id },
            }"
          >
            Destroy Cluster
          </TButton>
        </Tooltip>
      </div>
    </template>
    <div v-else class="flex items-center justify-center p-4">
      <TSpinner class="h-6 w-6" />
    </div>
  </div>
</template>
