<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { compare } from 'semver'
import type { Ref } from 'vue'
import { computed, onMounted, ref, useTemplateRef, watch, watchEffect } from 'vue'
import { useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type {
  MachineInstallDiskStatusSpec,
  MachineStatusSpec,
  TalosVersionSpec,
} from '@/api/omni/specs/omni.pb'
import type { VersionContractSpec } from '@/api/omni/specs/virtual.pb'
import {
  DefaultKubernetesVersion,
  DefaultNamespace,
  LabelNoManualAllocation,
  MachineInstallDiskStatusType,
  MachineStatusLabelAvailable,
  MachineStatusLabelInvalidState,
  MachineStatusLabelReadyToUse,
  MachineStatusLabelReportingEvents,
  MachineStatusType,
  PatchBaseWeightCluster,
  PatchBaseWeightMachineSet,
  TalosVersionType,
  VersionContractType,
  VirtualNamespace,
} from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import TList from '@/components/List/TList.vue'
import ConfigPatchEditModal from '@/components/Modals/ConfigPatchEditModal.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { setupBackupStatus } from '@/methods'
import { usePermissions } from '@/methods/auth'
import {
  ClusterCommandError,
  clusterSync,
  nextAvailableClusterName,
  reconcileInstallDiskConfigs,
} from '@/methods/cluster'
import { machineCompatibleWithCluster } from '@/methods/compat'
import { useFeatures } from '@/methods/features'
import { getPatch } from '@/methods/getPatch'
import { useResourceGet } from '@/methods/useResourceGet'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'
import { initState, PatchID } from '@/states/cluster-management'
import ClusterEtcdBackupCheckbox from '@/views/Clusters/ClusterEtcdBackupCheckbox.vue'
import ClusterMenu from '@/views/Clusters/ClusterMenu.vue'
import ClusterWorkloadProxyingCheckbox from '@/views/Clusters/ClusterWorkloadProxyingCheckbox.vue'
import UntaintSingleNodeModal from '@/views/Clusters/components/UntaintSingleNodeModal.vue'
import DiscoveryServiceSwitcher from '@/views/Clusters/DiscoveryServiceSwitcher.vue'
import ClusterMachineItem from '@/views/Clusters/Management/ClusterMachineItem.vue'
import MachineSets from '@/views/Clusters/Management/MachineSets.vue'
import NodeAuditSkipCheckbox from '@/views/Clusters/NodeAuditSkipCheckbox.vue'
import ItemLabels from '@/views/ItemLabels/ItemLabels.vue'
import AddingMachinesTutorial from '@/views/Machines/components/AddingMachinesTutorial.vue'

definePage({ name: 'ClusterCreate' })

const labelContainer: Ref<Resource> = computed(() => {
  return {
    metadata: {
      id: 'label-container',
      labels: state.value.cluster.labels ?? {},
    },
    spec: {},
  }
})

const { status: backupStatus } = setupBackupStatus()
const { canCreateClusters } = usePermissions()
const configPatchEditModalOpen = ref(false)
const untaintSingleNodeModalOpen = ref(false)

const state = initState()

const addLabels = (_: string, ...labels: string[]) => {
  state.value.addClusterLabels(labels)
}

const removeLabels = (_: string, ...keys: string[]) => {
  state.value.removeClusterLabels(keys)
}

const router = useRouter()

const kubernetesVersionSelector = useTemplateRef('kubernetesVersionSelector')

const { data: talosVersionsList } = useResourceWatch<TalosVersionSpec>({
  runtime: Runtime.Omni,
  resource: {
    type: TalosVersionType,
    namespace: DefaultNamespace,
  },
})

const { data: versionContract } = useResourceGet<VersionContractSpec>(() => ({
  skip: !state.value.cluster.talosVersion,
  runtime: Runtime.Omni,
  resource: {
    type: VersionContractType,
    namespace: VirtualNamespace,
    id: state.value.cluster.talosVersion!,
  },
}))

const reset = ref(0)

const kubernetesVersions: Ref<string[]> = computed(() => {
  for (const version of talosVersionsList.value) {
    if (version.spec.version === state.value.cluster.talosVersion) {
      return version.spec.compatible_kubernetes_versions ?? []
    }
  }

  return []
})

watch(kubernetesVersions, (k8sVersions) => {
  if (k8sVersions.length === 0) {
    kubernetesVersionSelector?.value?.selectItem('')
    return
  }

  const k8sVersionSet = new Set(k8sVersions)

  if (!state.value.cluster.kubernetesVersion) {
    kubernetesVersionSelector?.value?.selectItem(DefaultKubernetesVersion)

    return
  }

  // If currently selected Kubernetes version is not supported by the chosen Talos version
  if (!k8sVersionSet.has(state.value.cluster.kubernetesVersion)) {
    // if the default Kubernetes version is supported by the chosen Talos version, select it
    if (k8sVersionSet.has(DefaultKubernetesVersion)) {
      kubernetesVersionSelector?.value?.selectItem(DefaultKubernetesVersion)
      return
    }

    // select the latest supported Kubernetes version by the chosen Talos version (k8sVersions are sorted on backend)
    kubernetesVersionSelector?.value?.selectItem(k8sVersions[k8sVersions.length - 1])
  }
})

const { data: features } = useFeatures()

const isEmbeddedDiscoveryServiceAvailable = computed(
  () => features.value?.spec.embedded_discovery_service ?? false,
)

const supportsPublicDiscovery = computed(
  () => versionContract.value?.spec.discovery_service_multidoc_config ?? false,
)

watchEffect(() => {
  if (!isEmbeddedDiscoveryServiceAvailable.value) {
    state.value.cluster.features.useEmbeddedDiscoveryService = false
  }

  // the public service can only be turned off on Talos 1.14+; keep it on for older clusters
  if (!supportsPublicDiscovery.value) {
    state.value.cluster.features.disablePublicDiscoveryService = false
  }
})

onMounted(async () => {
  state.value.cluster.name = await nextAvailableClusterName(
    state.value.cluster.name ?? 'talos-default',
  )
})

const createCluster = async () => {
  if (state.value.untaintSingleNode()) {
    untaintSingleNodeModalOpen.value = true
  } else {
    await createCluster_(false)
  }
}

const detectVersionMismatch = (machine: Resource<MachineStatusSpec>) => {
  const compat = machineCompatibleWithCluster(machine, state.value.cluster.talosVersion!)
  return compat.ok ? null : compat.reason
}

const detectAutoInstallNotice = (machine: Resource<MachineStatusSpec>) => {
  const compat = machineCompatibleWithCluster(machine, state.value.cluster.talosVersion!)
  return compat.ok && compat.willAutoInstall ? compat.reason : null
}

const createCluster_ = async (untaint: boolean) => {
  if (
    typeof state.value.controlPlanesCount === 'number' &&
    (state.value.controlPlanesCount - 1) % 2 !== 0
  ) {
    showError(
      'Invalid Number of Control Planes',
      'The total number of control plane nodes must be an odd number to ensure etcd stability. (Three control plane nodes are required for a highly available control plane.)',
    )

    return
  }

  if (untaint) {
    state.value.controlPlanes().patches[PatchID.Untaint] = {
      data: await getPatch(versionContract.value!.spec, 'untaintNode'),
      weight: PatchBaseWeightMachineSet,
      systemPatch: true,
    }
  }

  try {
    // commit the install disk selections first, so they are in place before any MachineSetNode
    // makes a machine installable (the config resource never goes through clusterSync)
    await reconcileInstallDiskConfigs(state.value.installDiskIntents())

    await clusterSync(state.value.resources())
  } catch (e) {
    if (e.message && e.message.indexOf('already exists') >= 0) {
      state.value.cluster.name = await nextAvailableClusterName('talos-default')
    }

    if (e instanceof ClusterCommandError) {
      showError(e.errorNotification.title, e.errorNotification.details)

      return
    }

    showError('Failed to Create the Cluster', e.message)

    return
  }

  showSuccess(
    'Succesfully Created Cluster',
    `Cluster name: ${state.value.cluster.name}, control planes: ${state.value.controlPlanesCount}, workers: ${state.value.workersCount}`,
  )

  const clusterName = state.value.cluster.name

  initState()

  router.push({ name: 'ClusterOverview', params: { cluster: clusterName! } })
}

const { data: installDiskStatuses } = useResourceWatch<MachineInstallDiskStatusSpec>({
  resource: {
    type: MachineInstallDiskStatusType,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
})

const installDiskStatusMap = computed(() =>
  Object.fromEntries(installDiskStatuses.value.map((c) => [c.metadata.id!, c])),
)

const talosVersions = computed(() =>
  talosVersionsList.value
    .filter((v) => !v.spec.deprecated)
    .map(({ spec: { version, unsupported = false } }) => ({
      label: version!,
      value: version!,
      disabled: unsupported,
      tooltip: unsupported ? `This Omni release does not support Talos ${version}.` : undefined,
    }))
    .sort((a, b) => compare(a.value, b.value)),
)

const hasConfigs = computed(() => {
  return Object.keys(state.value.cluster.patches).length > 0
})

const onSavePatchConfig = (config: string) => {
  if (config === '') {
    delete state.value.cluster.patches[PatchID.Default]

    return
  }

  state.value.cluster.patches[PatchID.Default] = {
    data: config,
    weight: PatchBaseWeightCluster,
  }
}

const list = useTemplateRef('list')
</script>

<template>
  <PageContainer disable-padding class="flex h-full flex-col pt-6">
    <PageHeader title="Create Cluster" class="px-6" />

    <div class="flex grow flex-col items-stretch gap-4 overflow-y-auto px-6 pb-6">
      <div class="flex flex-wrap gap-2">
        <TInput
          title="Cluster Name"
          class="grow"
          placeholder="..."
          :model-value="state.cluster.name ?? ''"
          @update:model-value="(value) => (state.cluster.name = value)"
        />
        <TSelectList
          title="Talos Version"
          :values="talosVersions"
          :default-value="state.cluster.talosVersion"
          @checked-value="(value) => (state.cluster.talosVersion = value)"
        />
        <TSelectList
          ref="kubernetesVersionSelector"
          title="Kubernetes Version"
          :values="kubernetesVersions"
          :default-value="state.cluster.kubernetesVersion"
          @checked-value="(value) => (state.cluster.kubernetesVersion = value)"
        />
        <TButton
          variant="primary"
          :icon="hasConfigs ? 'settings-toggle' : 'settings'"
          @click="configPatchEditModalOpen = true"
        >
          Config Patches
        </TButton>
      </div>
      <div class="text-naturals-n13">Cluster Labels</div>
      <ItemLabels
        :resource="labelContainer"
        :add-label-func="addLabels"
        :remove-label-func="removeLabels"
      />
      <div class="text-naturals-n13">Cluster Features</div>
      <div class="flex max-w-sm flex-col gap-3">
        <Tooltip placement="bottom">
          <template #description>
            <div class="flex flex-col gap-1 p-2">
              <p>Encrypt machine disks using Omni as a key management server.</p>
              <p>Once cluster is created it is not possible to update encryption settings.</p>
            </div>
          </template>
          <TCheckbox v-model="state.cluster.features.encryptDisks" label="Encrypt Disks" />
        </Tooltip>
        <ClusterWorkloadProxyingCheckbox
          v-model="state.cluster.features.enableWorkloadProxy"
          :disabled="!features?.spec.enable_workload_proxying"
        />
        <NodeAuditSkipCheckbox v-model="state.cluster.features.enableNodeAuditSkip" />
        <ClusterEtcdBackupCheckbox
          :backup-status="backupStatus"
          :cluster="{
            backup_configuration: state.cluster.etcdBackupConfig,
          }"
          @update:cluster="
            (spec) => {
              state.cluster.etcdBackupConfig = spec.backup_configuration
            }
          "
        />
        <DiscoveryServiceSwitcher
          :use-embedded="state.cluster.features.useEmbeddedDiscoveryService ?? false"
          :disable-public="state.cluster.features.disablePublicDiscoveryService ?? false"
          :embedded-available="isEmbeddedDiscoveryServiceAvailable"
          :public-configurable="supportsPublicDiscovery"
          @change="
            ({ useEmbedded, disablePublic }) => {
              state.cluster.features.useEmbeddedDiscoveryService = useEmbedded
              state.cluster.features.disablePublicDiscoveryService = disablePublic
            }
          "
        />
      </div>
      <div class="text-naturals-n13">Machine Sets</div>
      <MachineSets />
      <div class="text-naturals-n13">Available Machines</div>
      <TList
        ref="list"
        :opts="{
          type: undefined as unknown as MachineStatusSpec,
          resource: {
            namespace: DefaultNamespace,
            type: MachineStatusType,
          },
          runtime: Runtime.Omni,
          selectors: [
            `${MachineStatusLabelAvailable}`,
            `${MachineStatusLabelReadyToUse}`,
            `!${MachineStatusLabelInvalidState}`,
            `${MachineStatusLabelReportingEvents}`,
            `!${LabelNoManualAllocation}`,
          ],
          sortByField: 'created',
        }"
        search
        pagination
        class="h-max shrink-0"
      >
        <template #norecords>
          <TAlert v-if="!$slots.norecords" type="info" title="No Machines Available">
            Machine is available when it is connected, not allocated and is reporting Talos events.
          </TAlert>

          <AddingMachinesTutorial class="mt-4" />
        </template>
        <template v-if="versionContract" #default="{ items, searchQuery }">
          <ClusterMachineItem
            v-for="item in items"
            :key="item.metadata.id"
            :version-contract
            :version-mismatch="detectVersionMismatch(item)"
            :auto-install-notice="detectAutoInstallNotice(item)"
            :reset="reset"
            :item="{
              ...item,
              spec: {
                ...item.spec,
                ...installDiskStatusMap[item.metadata.id!]?.spec,
              },
            }"
            :search-query="searchQuery"
            @filter-label="list?.addFilterLabel"
          />
        </template>
      </TList>
    </div>

    <div
      class="flex h-16 shrink-0 items-center border-t border-naturals-n4 bg-naturals-n1 px-5 py-3"
    >
      <ClusterMenu
        class="w-full"
        :control-planes="state.controlPlanesCount"
        :workers="state.workersCount"
        :on-submit="createCluster"
        :on-reset="() => reset++"
        :disabled="!canCreateClusters || !state.controlPlanesCount"
        action="Create Cluster"
      />
    </div>

    <ConfigPatchEditModal
      id="Cluster"
      v-model:open="configPatchEditModalOpen"
      :config="state.cluster.patches[PatchID.Default]?.data"
      :talos-version="state.cluster.talosVersion"
      @save="onSavePatchConfig"
    />

    <UntaintSingleNodeModal
      v-model:open="untaintSingleNodeModalOpen"
      :talos-version="state.cluster.talosVersion"
      @continue="createCluster_"
    />
  </PageContainer>
</template>
