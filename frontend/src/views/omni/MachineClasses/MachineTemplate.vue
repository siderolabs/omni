<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="text-naturals-N13">Machine Template</div>
  <div class="rounded bg-naturals-N2">
    <div class="text-naturals-N13 px-4 pt-4 pb-2 text-sm">Talos Config</div>
    <div class="machine-template text-xs flex flex-col divide-y divide-naturals-N4 border-t-8 border-naturals-N4">
      <div>
        <span>
          Kernel Arguments
        </span>
        <t-input class="h-7 w-56" :model-value="kernelArguments" @update:model-value="value => $emit('update:kernel-arguments', value)"/>
      </div>
      <div>
        <span>
          Initial Labels
        </span>
        <labels :model-value="initialLabels" @update:model-value="value => $emit('update:initial-labels', value)"/>
      </div>
      <div>
        <span>
          Use gRPC Tunnel
        </span>
        <t-select-list
        menu-align="right" class="h-6"
         :values="[GRPCTunnelMode.Default, GRPCTunnelMode.Enabled, GRPCTunnelMode.Disabled]" :default-value="defaultTunnelMode" @checked-value="updateGRPCTunnelMode"/>
      </div>
    </div>
  </div>
  <div class="rounded bg-naturals-N2" v-if="infraProviderStatus?.spec.schema">
    <div class="text-naturals-N13 px-4 pt-4 pb-2 text-sm">{{ infraProviderStatus.spec.name }} Provider Config</div>
    <div class="text-xs flex flex-col divide-y divide-naturals-N4 border-t-8 border-naturals-N4">
    <json-form
      :model-value="providerConfig"
      @update:model-value="value => $emit('update:provider-config', value)"
      :json-schema="infraProviderStatus.spec.schema"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { Resource } from "@/api/grpc";
import { Runtime } from "@/api/common/omni.pb";
import { InfraProviderStatusSpec } from "@/api/omni/specs/infra.pb";
import { GrpcTunnelMode } from "@/api/omni/specs/omni.pb";
import { InfraProviderStatusType, InfraProviderNamespace } from "@/api/resources";
import { computed, ref, toRefs } from "vue";
import TInput from "@/components/common/TInput/TInput.vue";

import WatchResource from "@/api/watch";
import TSelectList from "@/components/common/SelectList/TSelectList.vue";
import Labels from "@/components/common/Labels/Labels.vue";
import JsonForm from "@/components/common/Form/JsonForm.vue";

enum GRPCTunnelMode {
  Default = "Account Default",
  Enabled = "Enabled",
  Disabled = "Disabled"
}

const props = defineProps<{
  infraProvider: string,
  initialLabels: Record<string, any>
  kernelArguments: string
  providerConfig: Record<string, any>
  grpcTunnel: GrpcTunnelMode
}>();

const defaultTunnelMode = (() => {
  switch (props.grpcTunnel) {
  case GrpcTunnelMode.UNSET:
    return GRPCTunnelMode.Default;
  case GrpcTunnelMode.DISABLED:
    return GRPCTunnelMode.Disabled;
  case GrpcTunnelMode.ENABLED:
    return GRPCTunnelMode.Enabled;
  }

  return GrpcTunnelMode.UNSET;
})();

const emit = defineEmits([
  "update:kernel-arguments",
  "update:initial-labels",
  "update:provider-config",
  "update:grpc-tunnel",
]);

const {
  infraProvider,
  initialLabels,
  kernelArguments,
  providerConfig,
} = toRefs(props);

const infraProviderStatus = ref<Resource<InfraProviderStatusSpec>>();
const infraProviderStatusWatch = new WatchResource(infraProviderStatus);

infraProviderStatusWatch.setup(computed(() => {
  if (!infraProvider.value) {
    return;
  }

  return {
    resource: {
      type: InfraProviderStatusType,
      namespace: InfraProviderNamespace,
      id: infraProvider.value,
    },
    runtime: Runtime.Omni,
  };
}));

const updateGRPCTunnelMode = (value: GRPCTunnelMode) => {
  switch (value) {
  case GRPCTunnelMode.Default:
    emit("update:grpc-tunnel", GrpcTunnelMode.UNSET);
    break;
  case GRPCTunnelMode.Enabled:
    emit("update:grpc-tunnel", GrpcTunnelMode.ENABLED);
    break;
  case GRPCTunnelMode.Disabled:
    emit("update:grpc-tunnel", GrpcTunnelMode.DISABLED);
    break;
  }
}
</script>

<style scoped>
.condition {
  @apply border border-opacity-0 rounded-md border-transparent transition-colors;
}

.condition:focus-within {
  @apply border-naturals-N8;
}

code {
  @apply font-roboto rounded bg-naturals-N6 px-1 py-0.5 text-naturals-N13;
}

.machine-template > * {
  @apply px-4 py-2 flex justify-between items-center gap-2;
}

.machine-template > * > *:first-child {
  @apply whitespace-nowrap;
}
</style>
