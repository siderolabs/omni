<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Runtime } from '@/api/common/omni.pb'
import type { InfraProviderStatusSpec } from '@/api/omni/specs/infra.pb'
import { GrpcTunnelMode } from '@/api/omni/specs/omni.pb'
import { InfraProviderNamespace, InfraProviderStatusType } from '@/api/resources'
import JsonForm from '@/components/Form/JsonForm.vue'
import Labels, { type LabelSelectItem } from '@/components/Labels/Labels.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TInput from '@/components/TInput/TInput.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'

enum GRPCTunnelMode {
  Default = 'Account Default',
  Enabled = 'Enabled',
  Disabled = 'Disabled',
}

const { infraProvider } = defineProps<{
  infraProvider: string
}>()

const kernelArguments = defineModel<string>('kernelArguments')
const initialLabels = defineModel<Record<string, LabelSelectItem>>('initialLabels')
const providerConfig = defineModel<Record<string, unknown>>('providerConfig', { required: true })
const grpcTunnel = defineModel<GrpcTunnelMode>('grpcTunnel', { required: true })

const defaultTunnelMode = (() => {
  switch (grpcTunnel.value) {
    default:
    case GrpcTunnelMode.UNSET:
      return GRPCTunnelMode.Default
    case GrpcTunnelMode.DISABLED:
      return GRPCTunnelMode.Disabled
    case GrpcTunnelMode.ENABLED:
      return GRPCTunnelMode.Enabled
  }
})()

const { data: infraProviderStatus } = useResourceWatch<InfraProviderStatusSpec>(() => ({
  skip: !infraProvider,
  resource: {
    type: InfraProviderStatusType,
    namespace: InfraProviderNamespace,
    id: infraProvider,
  },
  runtime: Runtime.Omni,
}))

const updateGRPCTunnelMode = (value: GRPCTunnelMode) => {
  switch (value) {
    case GRPCTunnelMode.Default:
      grpcTunnel.value = GrpcTunnelMode.UNSET
      break
    case GRPCTunnelMode.Enabled:
      grpcTunnel.value = GrpcTunnelMode.ENABLED
      break
    case GRPCTunnelMode.Disabled:
      grpcTunnel.value = GrpcTunnelMode.DISABLED
      break
  }
}
</script>

<template>
  <div class="text-naturals-n13">Machine Template</div>
  <div class="rounded bg-naturals-n2">
    <div class="px-4 pt-4 pb-2 text-sm text-naturals-n13">Talos Config</div>
    <div class="flex flex-col divide-y divide-naturals-n4 border-t-8 border-naturals-n4 text-xs">
      <div class="flex items-center justify-between gap-2 px-4 py-2">
        <span class="whitespace-nowrap">Kernel Arguments</span>
        <TInput v-model="kernelArguments" class="h-7 w-56" />
      </div>
      <div class="flex items-center justify-between gap-2 px-4 py-2">
        <span class="whitespace-nowrap">Initial Labels</span>
        <Labels v-model="initialLabels" />
      </div>
      <div class="flex items-center justify-between gap-2 px-4 py-2">
        <span class="whitespace-nowrap">Use gRPC Tunnel</span>
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
      <JsonForm v-model="providerConfig" :json-schema="infraProviderStatus.spec.schema" />
    </div>
  </div>
</template>
