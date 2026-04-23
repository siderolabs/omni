<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import type {
  ExtensionsConfigurationSpec,
  MachineExtensionsStatusSpec,
} from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import {
  DefaultNamespace,
  ExtensionsConfigurationType,
  LabelCluster,
  LabelClusterMachine,
  MachineExtensionsStatusType,
} from '@/api/resources'
import Modal from '@/components/Modals/Modal.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError } from '@/notification'
import ExtensionsPicker from '@/views/Extensions/ExtensionsPicker.vue'

const { clusterId, machineId } = defineProps<{
  clusterId: string
  machineId: string
}>()

const open = defineModel<boolean>('open', { default: false })

const { data: configs, loading: configsLoading } = useResourceWatch<ExtensionsConfigurationSpec>(
  () => ({
    skip: !open.value,
    runtime: Runtime.Omni,
    resource: {
      namespace: DefaultNamespace,
      type: ExtensionsConfigurationType,
    },
    selectors: [`${LabelCluster}=${clusterId}`],
  }),
)

const { data: status } = useResourceWatch<MachineExtensionsStatusSpec>(() => ({
  skip: !open.value,
  resource: {
    id: machineId,
    namespace: DefaultNamespace,
    type: MachineExtensionsStatusType,
  },
  runtime: Runtime.Omni,
}))

const indeterminate = computed(() =>
  configs.value?.some((e) => !e.metadata.labels?.[LabelClusterMachine]),
)

const immutableExtensions = computed(() =>
  status.value?.spec.extensions
    ?.filter((e) => e.immutable)
    .reduce<Record<string, boolean>>(
      (prev, { name }) => ({
        ...prev,
        [name!]: true,
      }),
      {},
    ),
)

const enabledExtensions = ref<Record<string, boolean>>({})

watchEffect(() => {
  if (!status.value || configsLoading.value) {
    return
  }

  enabledExtensions.value = (status.value.spec.extensions ?? []).reduce<Record<string, boolean>>(
    (prev, { name }) => ({
      ...prev,
      [name!]: true,
    }),
    {},
  )
})

const updateExtensions = async () => {
  try {
    await updateExtensionsConfig()
  } catch (e) {
    showError('Failed to Update Extensions Config', e.message)

    return
  }

  open.value = false
}

const updateExtensionsConfig = async () => {
  if (!clusterId || !status.value) {
    return
  }

  const extensions: string[] = []

  for (const extension in enabledExtensions.value) {
    if (enabledExtensions.value[extension]) {
      extensions.push(extension)
    }
  }

  extensions.sort()

  const extensionsConfiguration: Resource<ExtensionsConfigurationSpec> = {
    metadata: {
      id: `schematic-${machineId}`,
      namespace: DefaultNamespace,
      type: ExtensionsConfigurationType,
      labels: {
        [LabelCluster]: clusterId,
        [LabelClusterMachine]: machineId,
      },
    },
    spec: {
      extensions,
    },
  }

  try {
    const existing: Resource<ExtensionsConfigurationSpec> = await ResourceService.Get(
      {
        id: extensionsConfiguration.metadata.id,
        namespace: extensionsConfiguration.metadata.namespace,
        type: extensionsConfiguration.metadata.type,
      },
      withRuntime(Runtime.Omni),
    )

    extensionsConfiguration.metadata.version = existing.metadata.version

    await ResourceService.Update(
      extensionsConfiguration,
      existing.metadata.version,
      withRuntime(Runtime.Omni),
    )
  } catch (e) {
    if (e.code !== Code.NOT_FOUND) {
      throw e
    }

    await ResourceService.Create(extensionsConfiguration, withRuntime(Runtime.Omni))
  }
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Update Extensions"
    action-label="Update"
    :loading="!status"
    @confirm="updateExtensions"
  >
    <ExtensionsPicker
      v-if="status"
      v-model="enabledExtensions"
      :talos-version="status.spec.talos_version!.slice(1)"
      class="flex-1"
      :indeterminate="indeterminate"
      :immutable-extensions="immutableExtensions"
    />
  </Modal>
</template>
