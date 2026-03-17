<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type monaco from 'monaco-editor/esm/vs/editor/editor.api'
import { MarkerSeverity, MarkerTag } from 'monaco-editor/esm/vs/editor/editor.api'
import { computed, defineAsyncComponent, ref, watch, watchEffect } from 'vue'
import { type RouteLocationRaw, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import { EventType } from '@/api/omni/resources/resources.pb'
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
  ClusterPermissionsType,
  ClusterType,
  ConfigPatchDescription,
  ConfigPatchName,
  ConfigPatchType,
  DefaultNamespace,
  LabelCluster,
  LabelClusterMachine,
  LabelHostname,
  LabelMachine,
  LabelMachineSet,
  MachineSetType,
  MachineStatusType,
  VirtualNamespace,
} from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import PageContainer from '@/components/common/PageContainer/PageContainer.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'
import { canManageMachineConfigPatches, canReadMachineConfigPatches } from '@/methods/auth'
import { machineSetTitle, sortMachineSetIds } from '@/methods/machineset'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError } from '@/notification'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'

const CodeEditor = defineAsyncComponent(
  () => import('@/components/common/CodeEditor/CodeEditor.vue'),
)

type Props = {
  patchId: string
  back: RouteLocationRaw
  machineId?: string
  clusterId?: string
}

const { patchId, machineId, clusterId, back } = defineProps<Props>()

const bootstrapped = ref(false)

const { data: configPatch, loading: patchWatchLoading } = useResourceWatch<ConfigPatchSpec>(
  () => ({
    runtime: Runtime.Omni,
    resource: {
      namespace: DefaultNamespace,
      type: ConfigPatchType,
      id: patchId,
    },
  }),
  (e) => {
    if (e.event?.event_type === EventType.BOOTSTRAPPED) {
      bootstrapped.value = true
    }
  },
)

const config = ref('')

const weight = ref(0)
const patchName = ref('User defined patch')
const patchDescription = ref('')

enum PatchType {
  Cluster = 'Cluster',
  ClusterMachine = 'Cluster Machine',
  Machine = 'Machine',
}

let codeEditor: monaco.editor.IStandaloneCodeEditor | undefined

const editorDidMount = (editor: monaco.editor.IStandaloneCodeEditor) => {
  codeEditor = editor
}

const { data: machine } = useResourceWatch<MachineStatusSpec>(() => ({
  skip: !machineId,
  resource: {
    type: MachineStatusType,
    id: machineId!,
    namespace: DefaultNamespace,
  },
  runtime: Runtime.Omni,
}))

const nodeIDMap: Record<string, string> = {}
const machineSetIDMap: Record<string, string> = {}
const patchToCreate: Resource<ConfigPatchSpec> = {
  metadata: {
    namespace: DefaultNamespace,
    type: ConfigPatchType,
    labels: {},
    annotations: {
      [ConfigPatchName]: patchName.value,
    },
  },
  spec: {
    data: '',
  },
}

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

let selectedPatchType: string

const setPatchType = (value: string) => {
  selectedPatchType = value
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

const machineSetTitles = computed(() => {
  const sorted = sortMachineSetIds(
    clusterId,
    machineSets.value.map((value) => value.metadata.id!),
  )

  return sorted.map((machineSetId) => {
    const title = machineSetTitle(clusterId, machineSetId)
    machineSetIDMap[title] = machineSetId ?? ''
    return title
  })
})

const machines = computed(() => {
  return clusterMachines.value.map((item: Resource) => {
    const name = `Node: ${(item.metadata?.labels ?? {})[LabelHostname] || item.metadata.id}`

    nodeIDMap[name] = item.metadata.id!

    return name
  })
})

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

watch(weight, (value: number) => {
  if (value < 100 || value > 900) {
    return
  }

  let id = patchId
  const match = /^\d+-(.+)/.exec(id)
  if (match) {
    id = match[1]
  }

  router.replace({ params: { patch: `${value}-${id}` } })
})

watch(patchName, () => {
  if (patchName.value) {
    patchToCreate.metadata.annotations![ConfigPatchName] = patchName.value
  } else {
    delete patchToCreate.metadata.annotations![ConfigPatchName]
  }
})

watch(patchDescription, () => {
  if (patchName.value) {
    patchToCreate.metadata.annotations![ConfigPatchDescription] = patchDescription.value
  } else {
    delete patchToCreate.metadata.annotations![ConfigPatchDescription]
  }
})

const loadPatch = () => {
  const match = /^(\d+)-.+/.exec(patchId)

  if (match) {
    weight.value = Math.min(999, Math.max(0, parseInt(match[1])))
  } else {
    weight.value = 500
  }

  if (configPatch.value?.spec?.data) {
    config.value = configPatch.value.spec.data
  }
}

const patchWatchOptions = ref()

const updatePatchWatchOptions = () => {
  patchWatchOptions.value = {
    runtime: Runtime.Omni,
    resource: {
      namespace: DefaultNamespace,
      type: ConfigPatchType,
      id: patchId,
    },
  }
}

updatePatchWatchOptions()
watch(() => patchId, updatePatchWatchOptions)

loadPatch()
watch(configPatch, loadPatch)

const patchTypes = computed(() => {
  if (clusterId && !machineId) {
    return [PatchType.Cluster as string].concat(machineSetTitles.value).concat(machines.value)
  }

  if (machine.value?.metadata.labels?.[LabelCluster] ?? machineId) {
    return [PatchType.ClusterMachine, PatchType.Machine]
  }

  return undefined
})

enum State {
  Unknown = 0,
  Exists = 1,
  NotExists = 2,
}

const state = computed(() => {
  if (!bootstrapped.value) {
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

const subtitle = computed(() => {
  if (state.value === State.Unknown) {
    return ''
  }

  return `Patch ID: ${patchId}`
})

const notes = computed(() => {
  if (state.value === State.Exists || state.value === State.NotExists) {
    return 'Note: Patches are applied immediately on creation/modification, and may result in graceful reboots.'
  }

  return ''
})

const ready = computed(() => {
  return permissionsLoaded.value && !patchWatchLoading.value
})

const saving = ref(false)

const getPatchLabels = () => {
  const patchType = selectedPatchType ?? patchTypes.value?.[0]

  if (!patchType || patchType === PatchType.Machine) {
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

  const machineID = nodeIDMap[patchType]

  if (patchType === PatchType.ClusterMachine || machineID) {
    labels[LabelClusterMachine] = machineID ?? machine.value?.metadata.id
  }

  const machineSetID = machineSetIDMap[patchType]

  if (machineSetID) {
    labels[LabelMachineSet] = machineSetID
  }

  return labels
}

const saveConfig = async () => {
  const create = state.value === State.NotExists

  if (codeEditor) {
    config.value = codeEditor.getValue()
  }

  let currentPatch: Resource<ConfigPatchSpec> | undefined = configPatch.value

  if (create) {
    patchToCreate.metadata.id = patchId
    patchToCreate.spec.data = config.value

    currentPatch = patchToCreate
    currentPatch.metadata.labels = getPatchLabels()
  }

  if (!currentPatch) {
    return
  }

  saving.value = true

  currentPatch.spec.data = config.value

  try {
    if (!create) {
      await ResourceService.Update(currentPatch, undefined, withRuntime(Runtime.Omni))
    } else {
      if (weight.value < 100 || weight.value > 900) {
        throw new Error('User patch weight must be in range 100-900')
      }

      await ResourceService.Create(currentPatch, withRuntime(Runtime.Omni))
    }

    configPatch.value = currentPatch
    router.push(back)
  } catch (e) {
    if (e.code === Code.INVALID_ARGUMENT) {
      showError('The Config is Invalid', e.message?.replace('failed to validate: ', ''))
    } else {
      showError('Failed to Update the Config', e.message)
    }
  } finally {
    saving.value = false
  }
}

const permissionsLoaded = ref(false)
const canReadConfigPatches = ref(false)
const canManageConfigPatches = ref(false)

watchEffect(async () => {
  if (clusterId) {
    const clusterPermissions = await ResourceService.Get(
      {
        namespace: VirtualNamespace,
        type: ClusterPermissionsType,
        id: clusterId,
      },
      withRuntime(Runtime.Omni),
    )

    canReadConfigPatches.value = clusterPermissions?.spec?.can_read_config_patches || false
    canManageConfigPatches.value = clusterPermissions?.spec?.can_manage_config_patches || false
  } else if (machineId) {
    canReadConfigPatches.value = canReadMachineConfigPatches.value
    canManageConfigPatches.value = canManageMachineConfigPatches.value
  } else {
    throw new Error('failed to determine the owner of the patch from the URI')
  }

  permissionsLoaded.value = true
})
</script>

<template>
  <div class="flex h-full flex-col">
    <PageContainer class="flex grow flex-col overflow-hidden">
      <PageHeader :title="title" :subtitle="subtitle" :notes="notes" />
      <ManagedByTemplatesWarning />
      <div v-if="state === State.NotExists" class="mb-4 flex items-center gap-3">
        <TInput v-model="patchName" title="Name" />
        <TInput v-model="patchDescription" class="flex-1" title="Description" />
        <TSelectList
          v-if="patchTypes"
          title="Patch Target"
          :default-value="patchTypes[0]"
          :values="patchTypes"
          @checked-value="setPatchType"
        />
        <Tooltip :open="weight < 100 || weight > 900" placement="bottom-start">
          <TInput v-model="weight" type="number" title="Weight" class="w-28" />
          <template #description>
            <div class="flex items-center gap-2 rounded bg-naturals-n3 p-2 text-xs">
              <TIcon icon="warning" class="h-5 w-5 fill-current text-yellow-y1" />
              Weight should be in range of 100-900.
            </div>
          </template>
        </Tooltip>
      </div>
      <div class="font-sm flex-1 overflow-y-hidden rounded bg-naturals-n1 px-2 py-3">
        <div v-if="!ready" class="flex h-full w-full items-center justify-center">
          <TSpinner class="h-6 w-6" />
        </div>

        <CodeEditor
          v-else
          v-model:value="config"
          :options="{ readOnly: !canManageConfigPatches }"
          :validators="[checkEncryption]"
          :talos-version="cluster?.spec.talos_version"
          @editor-did-mount="editorDidMount"
        />
      </div>
    </PageContainer>
    <div
      class="flex h-16 shrink-0 items-center gap-4 border-t border-naturals-n4 bg-naturals-n1 px-5 py-3"
    >
      <TButton class="secondary" @click="() => $router.push(back)">Back</TButton>
      <div class="flex-1" />
      <TButton
        variant="highlighted"
        :disabled="!canManageConfigPatches || saving"
        @click="saveConfig"
      >
        <TSpinner v-if="saving" class="h-5 w-5" />
        <span v-else>Save</span>
      </TButton>
    </div>
  </div>
</template>
