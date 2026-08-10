<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type monaco from 'monaco-editor'
import { MarkerSeverity, MarkerTag } from 'monaco-editor'
import { computed, ref, watch } from 'vue'
import { type RouteLocationRaw, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { RequestError } from '@/api/fetch.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import {
  type ClusterMachineStatusSpec,
  type ClusterSpec,
  type ConfigPatchSpec,
  type MachineSetSpec,
  type MachineStatusSpec,
} from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import {
  ClusterMachineStatusType,
  ClusterType,
  ConfigPatchDescription,
  ConfigPatchName,
  ConfigPatchType,
  DefaultNamespace,
  LabelCluster,
  LabelClusterMachine,
  LabelDisabled,
  LabelHostname,
  LabelMachine,
  LabelMachineSet,
  MachineSetType,
  MachineStatusType,
} from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import Switch from '@/components/Switch/Switch.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import { useClusterPermissions, usePermissions } from '@/methods/auth'
import { isConfigPatchDisabled } from '@/methods/config-patch'
import { machineSetTitle, sortMachineSetIds } from '@/methods/machineset'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError } from '@/notification'

type Props = {
  patchId: string
  back: RouteLocationRaw
  machineId?: string
  clusterId?: string
}

const { patchId: initialPatchId, machineId, clusterId, back } = defineProps<Props>()

const { canManageMachineConfigPatches, canReadMachineConfigPatches } = usePermissions()
const {
  canReadConfigPatches: canReadClusterConfigPatches,
  canManageConfigPatches: canManageClusterMachineConfigPatches,
} = useClusterPermissions(() => clusterId)

const { data: configPatch, loading: configPatchLoading } = useResourceWatch<ConfigPatchSpec>(
  () => ({
    runtime: Runtime.Omni,
    resource: {
      namespace: DefaultNamespace,
      type: ConfigPatchType,
      id: initialPatchId,
    },
  }),
)

enum State {
  Unknown = 0,
  Exists = 1,
  NotExists = 2,
}

enum PatchType {
  Cluster = 'Cluster',
  ClusterMachine = 'Cluster Machine',
  Machine = 'Machine',
}

const MIN_WEIGHT = 100
const MAX_WEIGHT = 900

const weight = ref(0)
const invalidWeight = computed(() => weight.value < MIN_WEIGHT || weight.value > MAX_WEIGHT)

const config = ref('')

const patchId = computed(() =>
  !invalidWeight.value ? initialPatchId.replace(/^\d+-/, `${weight.value}-`) : initialPatchId,
)
const patchName = ref('')
const patchDescription = ref('')
const patchType = ref<string>()
const patchEnabled = ref(true)

const { data: machine } = useResourceWatch<MachineStatusSpec>(() => ({
  skip: !machineId,
  resource: {
    type: MachineStatusType,
    id: machineId!,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
}))

const checkEncryption = (model: monaco.editor.ITextModel, tokens: monaco.Token[]) => {
  const markers: monaco.editor.IMarkerData[] = []
  if (!cluster.value?.spec?.features?.disk_encryption) {
    return markers
  }

  if (tokens.length === 0) {
    return markers
  }

  let offset = 0

  for (const token of tokens) {
    const pos = model.getPositionAt(offset)
    const word = model.getWordAtPosition(pos)
    offset += token.offset

    if (token.type !== 'type.yaml') {
      continue
    }

    if (word?.word === 'systemDiskEncryption') {
      markers.push({
        startColumn: word.startColumn,
        endColumn: word.endColumn,
        message:
          'Will have no effect: KMS encryption is enabled.\nKMS encryption config patch always has a higher priority.',
        severity: MarkerSeverity.Info,
        endLineNumber: pos.lineNumber,
        startLineNumber: pos.lineNumber,
        tags: [MarkerTag.Unnecessary],
      })

      break
    }
  }

  return markers
}

const { data: machineSets } = useResourceWatch<MachineSetSpec>(() => ({
  skip: !clusterId,
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: MachineSetType,
  },
  selectors: [`${LabelCluster}=${clusterId}`],
}))

const machineSetIDMap = computed(() =>
  Object.fromEntries(
    sortMachineSetIds(
      clusterId,
      machineSets.value.map((value) => value.metadata.id!),
    ).map((machineSetId) => [machineSetTitle(clusterId, machineSetId), machineSetId]),
  ),
)

const nodeIDMap = computed(() =>
  Object.fromEntries(
    clusterMachines.value.map((item) => [
      `Node: ${item.metadata.labels?.[LabelHostname] || item.metadata.id}`,
      item.metadata.id!,
    ]),
  ),
)

const machineSetTitles = computed(() => Object.keys(machineSetIDMap.value))
const machines = computed(() => Object.keys(nodeIDMap.value))

const { data: clusterMachines } = useResourceWatch<ClusterMachineStatusSpec>(() => ({
  skip: !clusterId,
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterMachineStatusType,
  },
  selectors: [`${LabelCluster}=${clusterId}`],
}))

const { data: cluster } = useResourceWatch<ClusterSpec>(() => ({
  skip: !clusterId,
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterType,
    id: clusterId!,
  },
}))

const router = useRouter()

const patchTypes = computed(() => {
  if (clusterId && machineId) {
    return [PatchType.ClusterMachine, PatchType.Machine]
  }

  if (machineId) {
    return [PatchType.Machine]
  }

  return [PatchType.Cluster, ...machineSetTitles.value, ...machines.value]
})

const state = computed(() => {
  if (configPatchLoading.value) {
    return State.Unknown
  }

  return configPatch.value ? State.Exists : State.NotExists
})

const title = computed(() => {
  if (!canReadConfigPatches.value) {
    return 'View Patch'
  }

  if (state.value === State.NotExists) {
    return 'Create Patch'
  }

  if (state.value === State.Exists) {
    return 'Edit Patch'
  }

  return 'Loading...'
})

const saving = ref(false)

const getPatchLabels = () => {
  if (!patchType.value || patchType.value === PatchType.Machine) {
    return {
      [LabelMachine]: machineId!,
    }
  }

  const cluster = clusterId ?? machine.value?.metadata.labels?.[LabelCluster]
  if (!cluster) {
    throw new Error('failed to determine machine cluster')
  }

  const labels: Record<string, string> = {
    [LabelCluster]: cluster,
  }

  const machineID = nodeIDMap.value[patchType.value]

  if (patchType.value === PatchType.ClusterMachine || machineID) {
    labels[LabelClusterMachine] = machineID ?? machine.value?.metadata.id
  }

  const machineSetID = machineSetIDMap.value[patchType.value]

  if (machineSetID) {
    labels[LabelMachineSet] = machineSetID
  }

  return labels
}

const saveConfig = async () => {
  if (invalidWeight.value) return

  const currentPatch: Resource<ConfigPatchSpec> = configPatch.value || {
    metadata: {
      namespace: DefaultNamespace,
      type: ConfigPatchType,
      id: patchId.value,
      labels: getPatchLabels(),
    },
    spec: {},
  }

  currentPatch.metadata.annotations ??= {}
  currentPatch.metadata.labels ??= {}
  currentPatch.spec.data = config.value

  if (patchName.value) {
    currentPatch.metadata.annotations[ConfigPatchName] = patchName.value
  } else {
    delete currentPatch.metadata.annotations[ConfigPatchName]
  }

  if (patchDescription.value) {
    currentPatch.metadata.annotations[ConfigPatchDescription] = patchDescription.value
  } else {
    delete currentPatch.metadata.annotations[ConfigPatchDescription]
  }

  if (patchEnabled.value) {
    delete currentPatch.metadata.labels[LabelDisabled]
  } else {
    currentPatch.metadata.labels[LabelDisabled] = ''
  }

  saving.value = true

  try {
    if (state.value === State.Exists) {
      await ResourceService.Update(currentPatch, undefined, withRuntime(Runtime.Omni))
    } else {
      await ResourceService.Create(currentPatch, withRuntime(Runtime.Omni))
    }

    router.replace(back)
  } catch (e) {
    if (e instanceof RequestError && e.code === Code.INVALID_ARGUMENT) {
      showError('The Config is Invalid', e.message?.replace('failed to validate: ', ''))
    } else {
      showError('Failed to Update the Config', e instanceof Error ? e.message : String(e))
    }
  } finally {
    saving.value = false
  }
}

const canReadConfigPatches = computed(() =>
  clusterId ? canReadClusterConfigPatches.value : canReadMachineConfigPatches.value,
)

const canManageConfigPatches = computed(() =>
  clusterId ? canManageClusterMachineConfigPatches.value : canManageMachineConfigPatches.value,
)

watch(
  configPatch,
  (patch) => {
    const match = /^(\d+)-.+/.exec(initialPatchId)

    if (match) {
      weight.value = Math.min(MAX_WEIGHT, Math.max(MIN_WEIGHT, parseInt(match[1])))
    } else {
      weight.value = 500
    }

    if (!patch) {
      patchName.value = 'User defined patch'
      patchDescription.value = ''
      patchEnabled.value = true
      patchType.value = patchTypes.value[0]

      return
    }

    const { labels = {}, annotations = {} } = patch.metadata

    config.value = patch.spec.data ?? ''
    patchName.value = annotations[ConfigPatchName] ?? ''
    patchDescription.value = annotations[ConfigPatchDescription] ?? ''
    patchEnabled.value = !isConfigPatchDisabled(labels)

    switch (true) {
      case LabelMachineSet in labels:
        patchType.value = machineSetTitle(clusterId, labels[LabelMachineSet])
        break
      case LabelClusterMachine in labels:
        patchType.value = PatchType.ClusterMachine
        break
      case LabelMachine in labels:
        patchType.value = PatchType.Machine
        break
      default:
        patchType.value = PatchType.Cluster
    }
  },
  { immediate: true },
)
</script>

<template>
  <div class="flex h-full flex-col">
    <PageContainer class="flex grow flex-col overflow-hidden">
      <PageHeader
        :title="title"
        :subtitle="`Patch ID: ${patchId}`"
        notes="Note: Patches are applied immediately on creation/modification, and may result in graceful reboots."
      />
      <ManagedByTemplatesWarning />
      <TAlert
        v-if="state === State.Exists && !patchEnabled"
        class="mb-4"
        title="Disabled"
        type="warn"
      >
        This config patch is disabled and is not applied to any machines.
      </TAlert>
      <div v-if="state === State.NotExists" class="mb-4 flex items-center gap-3">
        <TInput v-model="patchName" title="Name" />
        <TInput v-model="patchDescription" class="flex-1" title="Description" />
        <TSelectList
          v-if="patchTypes.length"
          v-model="patchType"
          title="Patch Target"
          :values="patchTypes"
        />
        <Tooltip :open="invalidWeight" placement="bottom-start">
          <TInput v-model="weight" type="number" title="Weight" class="w-28" />
          <template #description>
            <div class="flex items-center gap-2 rounded bg-naturals-n3 p-2 text-xs">
              <TIcon icon="warning" class="h-5 w-5 fill-current text-yellow-y1" />
              Weight should be in range of {{ MIN_WEIGHT }}-{{ MAX_WEIGHT }}.
            </div>
          </template>
        </Tooltip>
      </div>
      <div class="font-sm flex-1 overflow-y-hidden rounded bg-naturals-n1 px-2 py-3">
        <div v-if="configPatchLoading" class="flex h-full w-full items-center justify-center">
          <TSpinner class="h-6 w-6" />
        </div>

        <CodeEditor
          v-else
          v-model="config"
          :options="{ readOnly: !canManageConfigPatches }"
          :validators="[checkEncryption]"
          :talos-version="cluster?.spec.talos_version"
          class="size-full"
        />
      </div>
    </PageContainer>
    <div
      class="flex h-16 shrink-0 items-center gap-4 border-t border-naturals-n4 bg-naturals-n1 px-5 py-3"
    >
      <TButton class="secondary" @click="() => $router.push(back)">Back</TButton>
      <div class="flex-1" />

      <Switch v-model="patchEnabled" :disabled="!canManageConfigPatches" label="Enabled" />

      <TButton
        variant="highlighted"
        :disabled="!canManageConfigPatches || configPatchLoading || invalidWeight || saving"
        @click="saveConfig"
      >
        <TSpinner v-if="saving" class="h-5 w-5" />
        <span v-else>Save</span>
      </TButton>
    </div>
  </div>
</template>
