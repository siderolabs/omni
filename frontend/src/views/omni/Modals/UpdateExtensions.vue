<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

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
import Watch from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import { showError } from '@/notification'
import ExtensionsPicker from '@/views/omni/Extensions/ExtensionsPicker.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const router = useRouter()
const route = useRoute()
let closed = false

const close = () => {
  if (closed) {
    return
  }

  closed = true

  router.go(-1)
}

const enabledExtensions = ref<Record<string, boolean>>({})
const immutableExtensions = ref<Record<string, boolean>>({})

const extensionConfiguration = ref<Resource<ExtensionsConfigurationSpec>>()
const extensionConfigurationWatch = new Watch(extensionConfiguration)

const machineExtensionsStatus = ref<Resource<MachineExtensionsStatusSpec>>()
const machineExtensionsStatusWatch = new Watch(machineExtensionsStatus)

const indeterminate = computed(() => {
  return (
    extensionConfiguration?.value &&
    !extensionConfiguration.value.metadata.labels?.[LabelClusterMachine]
  )
})

const configurationLoading = extensionConfigurationWatch.loading

const ready = computed(() => {
  if (!machineExtensionsStatus.value || configurationLoading.value) {
    return false
  }

  return true
})

watch(
  [() => machineExtensionsStatus.value, () => extensionConfiguration.value, () => ready.value],
  () => {
    if (!ready.value) {
      return
    }

    for (const extension of machineExtensionsStatus.value?.spec.extensions ?? []) {
      enabledExtensions.value[extension.name!] = true

      if (extension.immutable) {
        immutableExtensions.value[extension.name!] = true
      }
    }
  },
)

machineExtensionsStatusWatch.setup(
  computed(() => {
    return {
      resource: {
        id: route.query.machine as string,
        namespace: DefaultNamespace,
        type: MachineExtensionsStatusType,
      },
      runtime: Runtime.Omni,
    }
  }),
)

const updateExtensions = async () => {
  try {
    await updateExtensionsConfig()
  } catch (e) {
    showError('Failed to Update Extensions Config', e.message)

    return
  }

  close()
}

const updateExtensionsConfig = async () => {
  const clusterName = machineExtensionsStatus.value?.metadata.labels?.[LabelCluster]
  if (!clusterName || !machineExtensionsStatus.value) {
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
      id: `schematic-${machineExtensionsStatus.value.metadata.id!}`,
      namespace: DefaultNamespace,
      type: ExtensionsConfigurationType,
      labels: {
        [LabelCluster]: clusterName,
        [LabelClusterMachine]: machineExtensionsStatus.value.metadata.id!,
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
    if (e.code === Code.NOT_FOUND) {
      await ResourceService.Create(extensionsConfiguration, withRuntime(Runtime.Omni))

      return
    }

    throw e
  }
}
</script>

<template>
  <div class="modal-window flex flex-col gap-4" style="height: 90%">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Update Extensions</h3>
      <CloseButton @click="close" />
    </div>

    <div v-if="machineExtensionsStatus" class="flex flex-1 flex-col gap-4 overflow-hidden">
      <ExtensionsPicker
        v-model="enabledExtensions"
        :talos-version="machineExtensionsStatus?.spec.talos_version!.slice(1)"
        class="flex-1"
        :indeterminate="indeterminate"
        :immutable-extensions="immutableExtensions"
      />

      <div class="flex justify-between gap-4">
        <TButton type="secondary" @click="close">Cancel</TButton>
        <TButton type="highlighted" @click="updateExtensions">Update</TButton>
      </div>
    </div>
    <div v-else class="flex items-center justify-center">
      <TSpinner class="h-6 w-6" />
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.modal-window {
  @apply h-auto w-1/2 p-8;
}

.heading {
  @apply flex items-center justify-between text-xl text-naturals-n14;
}
</style>
