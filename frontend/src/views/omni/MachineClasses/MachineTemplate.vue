<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, toRefs } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { InfraProviderStatusSpec } from '@/api/omni/specs/infra.pb'
import { GrpcTunnelMode } from '@/api/omni/specs/omni.pb'
import { InfraProviderNamespace, InfraProviderStatusType } from '@/api/resources'
import WatchResource from '@/api/watch'
import JsonForm from '@/components/common/Form/JsonForm.vue'
import Labels from '@/components/common/Labels/Labels.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import TInput from '@/components/common/TInput/TInput.vue'

enum GRPCTunnelMode {
  Default = 'Account Default',
  Enabled = 'Enabled',
  Disabled = 'Disabled',
}

const props = defineProps<{
  infraProvider: string
  initialLabels: Record<string, any>
  kernelArguments: string
  providerConfig: Record<string, any>
  grpcTunnel: GrpcTunnelMode
}>()

const defaultTunnelMode = (() => {
  switch (props.grpcTunnel) {
    default:
    case GrpcTunnelMode.UNSET:
      return GRPCTunnelMode.Default
    case GrpcTunnelMode.DISABLED:
      return GRPCTunnelMode.Disabled
    case GrpcTunnelMode.ENABLED:
      return GRPCTunnelMode.Enabled
  }
})()

const emit = defineEmits([
  'update:kernel-arguments',
  'update:initial-labels',
  'update:provider-config',
  'update:grpc-tunnel',
])

const { infraProvider, initialLabels, kernelArguments, providerConfig } = toRefs(props)

const infraProviderStatus = ref<Resource<InfraProviderStatusSpec>>()
const infraProviderStatusWatch = new WatchResource(infraProviderStatus)

infraProviderStatusWatch.setup(
  computed(() => {
    if (!infraProvider.value) {
      return
    }

    return {
      resource: {
        type: InfraProviderStatusType,
        namespace: InfraProviderNamespace,
        id: infraProvider.value,
      },
      runtime: Runtime.Omni,
    }
  }),
)

const updateGRPCTunnelMode = (value: GRPCTunnelMode) => {
  switch (value) {
    case GRPCTunnelMode.Default:
      emit('update:grpc-tunnel', GrpcTunnelMode.UNSET)
      break
    case GRPCTunnelMode.Enabled:
      emit('update:grpc-tunnel', GrpcTunnelMode.ENABLED)
      break
    case GRPCTunnelMode.Disabled:
      emit('update:grpc-tunnel', GrpcTunnelMode.DISABLED)
      break
  }
}
</script>

<template>
  <div class="text-naturals-n13">Machine Template</div>
  <div class="rounded bg-naturals-n2">
    <div class="px-4 pt-4 pb-2 text-sm text-naturals-n13">Talos Config</div>
    <div
      class="machine-template flex flex-col divide-y divide-naturals-n4 border-t-8 border-naturals-n4 text-xs"
    >
      <div>
        <span>Kernel Arguments</span>
        <TInput
          class="h-7 w-56"
          :model-value="kernelArguments"
          @update:model-value="(value) => $emit('update:kernel-arguments', value)"
        />
      </div>
      <div>
        <span>Initial Labels</span>
        <Labels
          :model-value="initialLabels"
          @update:model-value="(value) => $emit('update:initial-labels', value)"
        />
      </div>
      <div>
        <span>Use gRPC Tunnel</span>
        <TSelectList
          class="h-6"
          :values="[GRPCTunnelMode.Default, GRPCTunnelMode.Enabled, GRPCTunnelMode.Disabled]"
          :default-value="defaultTunnelMode"
          @checked-value="updateGRPCTunnelMode"
        />
      </div>
    </div>
  </div>
  <div v-if="infraProviderStatus?.spec.schema" class="rounded bg-naturals-n2">
    <div class="px-4 pt-4 pb-2 text-sm text-naturals-n13">
      {{ infraProviderStatus.spec.name }} Provider Config
    </div>
    <div class="flex flex-col divide-y divide-naturals-n4 border-t-8 border-naturals-n4 text-xs">
      <JsonForm
        :model-value="providerConfig"
        :json-schema="infraProviderStatus.spec.schema"
        @update:model-value="(value) => $emit('update:provider-config', value)"
      />
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.condition {
  @apply rounded-md border border-transparent transition-colors;
}

.condition:focus-within {
  @apply border-naturals-n8;
}

code {
  @apply rounded bg-naturals-n6 px-1 py-0.5 font-mono text-naturals-n13;
}

.machine-template > * {
  @apply flex items-center justify-between gap-2 px-4 py-2;
}

.machine-template > * > *:first-child {
  @apply whitespace-nowrap;
}
</style>
